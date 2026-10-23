package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/secrecy"
)

// ===================== AI 写作助手 =====================
// 模型接口统一按 OpenAI 兼容格式（/chat/completions）调用：本地模型（Ollama、vLLM 等）与网络接口（DeepSeek 等）共用一套代码。
// 金额一律以“微元”（1 元 = 1,000,000）整数存储；单价单位为“元/百万 token”，故 token 数 × 单价 恰为微元。
// 对话正文不落库，只记录谁、何时、用了多少 token、扣了多少钱。

const microPerYuan = 1_000_000

var (
	aiSecret string
	// 每个用户同一时刻只允许一条在途请求（防并发超扣）；全局并发上限保护单机服务器
	aiBusy = sync.Map{}
	aiSlot = make(chan struct{}, 8)
	// 上游请求专用客户端：不设整体超时（流式回复时间不定），只限制连接与首包时间
	aiClient = &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       60 * time.Second,
	}}
	beijing = time.FixedZone("CST", 8*3600)
)

// SetAISecret 设置 API 密钥加密所用的根密钥（取自 jwt.secret，启动时调用一次）
func SetAISecret(s string) { aiSecret = s }

func aiCipher() (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte("ynxwxcb-ai-api-key|" + aiSecret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func aiEncrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	g, err := aiCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func aiDecrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	g, err := aiCipher()
	if err != nil {
		return "", err
	}
	if len(raw) < g.NonceSize() {
		return "", errors.New("密文长度异常")
	}
	plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

type aiSettings struct {
	Enabled      bool
	Provider     string
	BaseURL      string
	KeyEnc       string
	Model        string
	PriceHit     float64
	PriceMiss    float64
	PriceOut     float64
	OffpeakRatio float64
	MaxOut       int
	MaxInChars   int
	WarnMicro    int64
}

func loadAISettings() (*aiSettings, error) {
	s := &aiSettings{}
	var enabled int
	var provider, baseURL, keyEnc, model sql.NullString
	err := database.DB.QueryRow(`SELECT enabled, provider, base_url, api_key_enc, model, price_in_hit, price_in_miss, price_out,
		offpeak_ratio, max_output_tokens, max_input_chars, warn_balance_micro FROM ai_settings WHERE id = 1`).
		Scan(&enabled, &provider, &baseURL, &keyEnc, &model, &s.PriceHit, &s.PriceMiss, &s.PriceOut,
			&s.OffpeakRatio, &s.MaxOut, &s.MaxInChars, &s.WarnMicro)
	if err != nil {
		return nil, err
	}
	s.Enabled = enabled == 1
	s.Provider = provider.String
	s.BaseURL = baseURL.String
	s.KeyEnc = keyEnc.String
	s.Model = model.String
	if s.MaxOut <= 0 {
		s.MaxOut = 2048
	}
	if s.MaxInChars <= 0 {
		s.MaxInChars = 12000
	}
	return s, nil
}

// isFree 三项单价都为 0（典型为本地模型）时不扣费
func (s *aiSettings) isFree() bool {
	return s.PriceHit <= 0 && s.PriceMiss <= 0 && s.PriceOut <= 0
}

// isPeakTime 北京时间周一至周五 9:00-12:00、14:00-18:00 为高峰时段（DeepSeek 官方口径；法定节假日不单独识别，按高峰价计，对用户偏保守）
func isPeakTime(t time.Time) bool {
	t = t.In(beijing)
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h := t.Hour()
	return (h >= 9 && h < 12) || (h >= 14 && h < 18)
}

// ratioAt 返回 t 时刻的价格系数：高峰为 1，空闲时段为 offpeak_ratio
func (s *aiSettings) ratioAt(t time.Time) float64 {
	if s.OffpeakRatio <= 0 || s.OffpeakRatio >= 1 || isPeakTime(t) {
		return 1
	}
	return s.OffpeakRatio
}

// costMicro 计算费用（微元，向上取整，避免有价模型出现零成本漏洞）
func (s *aiSettings) costMicro(hit, miss, out int, ratio float64) int64 {
	if s.isFree() {
		return 0
	}
	v := (float64(hit)*s.PriceHit + float64(miss)*s.PriceMiss + float64(out)*s.PriceOut) * ratio
	return int64(math.Ceil(v - 1e-9))
}

func (s *aiSettings) providerLabel() string {
	if s.Provider == "local" {
		return "本地模型"
	}
	return "网络接口"
}

func aiFail(w http.ResponseWriter, status int, code, msg string) {
	middleware.JSON(w, status, map[string]string{"error": msg, "code": code})
}

func estTokens(s string, perRune float64) int {
	return int(math.Ceil(float64(utf8.RuneCountInString(s)) * perRune))
}

func yuan(micro int64) float64 { return float64(micro) / microPerYuan }

func ensureAIAccount(userID int64) error {
	_, err := database.DB.Exec("INSERT OR IGNORE INTO ai_accounts (user_id) VALUES (?)", userID)
	return err
}

func aiBalance(userID int64) int64 {
	var b int64
	database.DB.QueryRow("SELECT balance_micro FROM ai_accounts WHERE user_id = ?", userID).Scan(&b)
	return b
}

// ---------------------------------------------------------------------
// 用户端：状态 / 提示词 / 用量
// ---------------------------------------------------------------------

// AIStatus 当前用户可见的 AI 状态：是否开启、模型类型、余额、单价（不含接口地址与密钥）
func AIStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	s, err := loadAISettings()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	ensureAIAccount(userID)
	bal := aiBalance(userID)
	configured := s.BaseURL != "" && s.Model != ""
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"enabled":        s.Enabled && configured,
		"provider":       s.Provider,
		"provider_label": s.providerLabel(),
		"model":          s.Model,
		"free":           s.isFree(),
		"balance_micro":  bal,
		"balance":        yuan(bal),
		"low":            !s.isFree() && bal < s.WarnMicro,
		"warn":           yuan(s.WarnMicro),
		"price_in_hit":   s.PriceHit,
		"price_in_miss":  s.PriceMiss,
		"price_out":      s.PriceOut,
		"offpeak_ratio":  s.OffpeakRatio,
		"peak_now":       isPeakTime(time.Now()),
		"max_input":      s.MaxInChars,
	})
}

// AIListPrompts 启用中的提示词场景（不返回系统提示词正文）
func AIListPrompts(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, scene_key, name, category, hint, placeholder FROM ai_prompts WHERE status = 1 ORDER BY sort, id`)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type item struct {
		ID          int64  `json:"id"`
		Key         string `json:"scene_key"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Hint        string `json:"hint"`
		Placeholder string `json:"placeholder"`
	}
	list := []item{}
	for rows.Next() {
		var it item
		var key, cat, hint, ph sql.NullString
		if err := rows.Scan(&it.ID, &key, &it.Name, &cat, &hint, &ph); err != nil {
			continue
		}
		it.Key, it.Category, it.Hint, it.Placeholder = key.String, cat.String, hint.String, ph.String
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	middleware.JSON(w, http.StatusOK, list)
}

// AIMyUsage 当前用户的用量与扣费明细（分页），附本月汇总
func AIMyUsage(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	p := parsePage(r)
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM ai_usage WHERE user_id = ?", userID).Scan(&total); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	rows, err := database.DB.Query(`SELECT datetime(created_at, '+8 hours'), scene, model, prompt_tokens, completion_tokens, cost_micro, status, estimated
		FROM ai_usage WHERE user_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`, userID, p.PageSize, (p.Page-1)*p.PageSize)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type row struct {
		Time       string  `json:"time"`
		Scene      string  `json:"scene"`
		Model      string  `json:"model"`
		Prompt     int     `json:"prompt_tokens"`
		Completion int     `json:"completion_tokens"`
		Cost       float64 `json:"cost"`
		Status     string  `json:"status"`
		Estimated  bool    `json:"estimated"`
	}
	list := []row{}
	for rows.Next() {
		var it row
		var scene, model, status sql.NullString
		var cost int64
		var est int
		if err := rows.Scan(&it.Time, &scene, &model, &it.Prompt, &it.Completion, &cost, &status, &est); err != nil {
			continue
		}
		it.Scene, it.Model, it.Status, it.Cost, it.Estimated = scene.String, model.String, status.String, yuan(cost), est == 1
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	month := time.Now().In(beijing).Format("2006-01")
	var mCost, mTokens, mCalls int64
	database.DB.QueryRow(`SELECT COALESCE(SUM(cost_micro),0), COALESCE(SUM(prompt_tokens+completion_tokens),0), COUNT(*)
		FROM ai_usage WHERE user_id = ? AND datetime(created_at, '+8 hours') GLOB ?`, userID, month+"*").Scan(&mCost, &mTokens, &mCalls)
	ensureAIAccount(userID)
	var topup, spent int64
	database.DB.QueryRow("SELECT total_topup_micro, total_spent_micro FROM ai_accounts WHERE user_id = ?", userID).Scan(&topup, &spent)
	res := paginateResult(list, total, p.Page, p.PageSize)
	res["summary"] = map[string]interface{}{
		"balance":      yuan(aiBalance(userID)),
		"month_cost":   yuan(mCost),
		"month_tokens": mTokens,
		"month_calls":  mCalls,
		"total_topup":  yuan(topup),
		"total_spent":  yuan(spent),
	}
	middleware.JSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------
// 上游调用
// ---------------------------------------------------------------------

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiUsageInfo struct {
	Prompt     int
	Completion int
	Hit        int
	Known      bool
}

func aiEndpoint(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	b = strings.TrimSuffix(b, "/chat/completions")
	return b + "/chat/completions"
}

// aiUpstream 向模型接口发起一次对话请求；调用方负责关闭 Body
func aiUpstream(ctx context.Context, s *aiSettings, msgs []aiMessage, stream bool, maxTokens int) (*http.Response, error) {
	payload := map[string]interface{}{
		"model":      s.Model,
		"messages":   msgs,
		"stream":     stream,
		"max_tokens": maxTokens,
	}
	if stream {
		payload["stream_options"] = map[string]bool{"include_usage": true}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aiEndpoint(s.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key, err := aiDecrypt(s.KeyEnc); err != nil {
		return nil, errors.New("接口密钥无法解密，请管理员重新填写密钥")
	} else if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return aiClient.Do(req)
}

func upstreamErrorMessage(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "AI 接口密钥无效或无权限，请联系管理员检查设置"
	case http.StatusPaymentRequired:
		return "AI 服务商账户余额不足，请联系管理员"
	case http.StatusTooManyRequests:
		return "AI 服务繁忙，请稍后再试"
	case http.StatusNotFound:
		return "AI 接口地址或模型名称有误，请联系管理员检查设置"
	}
	return fmt.Sprintf("AI 服务暂时不可用（%d），请稍后再试", status)
}

// parseUsage 解析不同厂商的用量字段：DeepSeek 直接给命中/未命中，OpenAI 系给 prompt_tokens_details.cached_tokens
func parseUsage(raw json.RawMessage) (aiUsageInfo, bool) {
	var u struct {
		PromptTokens     int  `json:"prompt_tokens"`
		CompletionTokens int  `json:"completion_tokens"`
		CacheHit         *int `json:"prompt_cache_hit_tokens"`
		Details          *struct {
			Cached int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &u) != nil {
		return aiUsageInfo{}, false
	}
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return aiUsageInfo{}, false
	}
	hit := 0
	if u.CacheHit != nil {
		hit = *u.CacheHit
	} else if u.Details != nil {
		hit = u.Details.Cached
	}
	if hit < 0 {
		hit = 0
	}
	if hit > u.PromptTokens {
		hit = u.PromptTokens
	}
	return aiUsageInfo{Prompt: u.PromptTokens, Completion: u.CompletionTokens, Hit: hit, Known: true}, true
}

// settleAIUsage 在一个事务内扣减余额并写入用量明细，返回扣费后的余额
func settleAIUsage(userID int64, scene string, s *aiSettings, u aiUsageInfo, cost int64, status string, estimated bool) int64 {
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("[AI] 扣费失败（开启事务）: %v", err)
		return aiBalance(userID)
	}
	defer tx.Rollback()
	est := 0
	if estimated {
		est = 1
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO ai_accounts (user_id) VALUES (?)", userID); err != nil {
		log.Printf("[AI] 扣费失败（建账户）: %v", err)
		return aiBalance(userID)
	}
	if _, err := tx.Exec(`UPDATE ai_accounts SET balance_micro = balance_micro - ?, total_spent_micro = total_spent_micro + ?,
		total_tokens = total_tokens + ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`,
		cost, cost, u.Prompt+u.Completion, userID); err != nil {
		log.Printf("[AI] 扣费失败（更新余额）: %v", err)
		return aiBalance(userID)
	}
	if _, err := tx.Exec(`INSERT INTO ai_usage (user_id, scene, provider, model, prompt_tokens, hit_tokens, completion_tokens, cost_micro, status, estimated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, scene, s.Provider, s.Model, u.Prompt, u.Hit, u.Completion, cost, status, est); err != nil {
		log.Printf("[AI] 扣费失败（写明细）: %v", err)
		return aiBalance(userID)
	}
	var bal int64
	tx.QueryRow("SELECT balance_micro FROM ai_accounts WHERE user_id = ?", userID).Scan(&bal)
	if err := tx.Commit(); err != nil {
		log.Printf("[AI] 扣费失败（提交）: %v", err)
		return aiBalance(userID)
	}
	return bal
}

// ---------------------------------------------------------------------
// 对话（SSE 流式）
// ---------------------------------------------------------------------

// AIChat 流式对话。请求：{scene_id, messages:[{role,content}]}；系统提示词一律由服务端按场景注入，客户端不能自带 system 消息。
func AIChat(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)

	var req struct {
		SceneID  int64       `json:"scene_id"`
		Messages []aiMessage `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aiFail(w, http.StatusBadRequest, "", "请求格式错误")
		return
	}
	s, err := loadAISettings()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "读取 AI 设置失败")
		return
	}
	if !s.Enabled {
		aiFail(w, http.StatusForbidden, "AI_DISABLED", "管理员尚未开启 AI 写作助手")
		return
	}
	if s.BaseURL == "" || s.Model == "" {
		aiFail(w, http.StatusServiceUnavailable, "AI_NOT_CONFIGURED", "AI 接口尚未配置，请联系管理员")
		return
	}

	// 校验消息：仅允许 user/assistant，最后一条必须是用户消息
	if n := len(req.Messages); n == 0 || n > 40 {
		aiFail(w, http.StatusBadRequest, "", "对话内容为空或轮次过多，请清空后重新开始")
		return
	}
	totalRunes := 0
	msgs := make([]aiMessage, 0, len(req.Messages)+1)
	for _, m := range req.Messages {
		m.Content = strings.TrimSpace(m.Content)
		if (m.Role != "user" && m.Role != "assistant") || m.Content == "" {
			aiFail(w, http.StatusBadRequest, "", "对话内容格式错误")
			return
		}
		totalRunes += utf8.RuneCountInString(m.Content)
		msgs = append(msgs, m)
	}
	if msgs[len(msgs)-1].Role != "user" {
		aiFail(w, http.StatusBadRequest, "", "对话内容格式错误")
		return
	}
	if totalRunes > s.MaxInChars {
		aiFail(w, http.StatusBadRequest, "AI_TOO_LONG", fmt.Sprintf("本次对话累计 %d 字，超过上限 %d 字，请清空对话或精简内容后再试", totalRunes, s.MaxInChars))
		return
	}

	// 保密防线：用户输入一律先过涉密检测，命中即阻断并留痕（与文件上传同一口径）
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		if res := secrecy.CheckPromptText(m.Content); res.Violated {
			logOperation(r, "保密防线", "涉密阻断", fmt.Sprintf("拦截涉密内容发送至AI助手 (密级: %s, 规则: %s, 详情: %s)", res.Category, res.Rule, res.Detail))
			middleware.JSON(w, http.StatusForbidden, map[string]interface{}{
				"error":              "【国家保密安全警报】检测到输入内容带有国家秘密或内部保密标识，严禁在非涉密系统处理传输！",
				"security_violation": "SECRECY_LEAK_PREVENTED",
				"filename":           "AI 写作助手输入内容",
				"matched_rule":       res.Rule,
				"category":           res.Category,
				"detail":             res.Detail,
			})
			return
		}
	}

	// 场景提示词
	sceneName := "自由对话"
	system := "你是县委宣传部的办公助手，请用通俗准确的中文回答，不得编造事实、数据、文号和人名。"
	if req.SceneID > 0 {
		var name, sp string
		if err := database.DB.QueryRow("SELECT name, system_prompt FROM ai_prompts WHERE id = ? AND status = 1", req.SceneID).Scan(&name, &sp); err == nil {
			sceneName, system = name, sp
		}
	}
	upMsgs := append([]aiMessage{{Role: "system", Content: system}}, msgs...)

	// 余额预检：按“本次可能产生的最高费用”（输入全按未命中 + 输出打满上限 + 高峰价）预留，不足即停止服务
	if err := ensureAIAccount(userID); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "读取账户失败")
		return
	}
	if !s.isFree() {
		estIn := estTokens(system, 1.0) + totalRunes + 50
		worst := s.costMicro(0, estIn, s.MaxOut, 1)
		if bal := aiBalance(userID); bal < worst || bal <= 0 {
			aiFail(w, http.StatusPaymentRequired, "AI_NO_BALANCE", "您的 AI 额度不足，已停止服务，请联系管理员充值")
			return
		}
	}

	// 并发控制
	if _, loaded := aiBusy.LoadOrStore(userID, true); loaded {
		aiFail(w, http.StatusTooManyRequests, "AI_BUSY", "上一条回复尚未结束，请稍候")
		return
	}
	defer aiBusy.Delete(userID)
	select {
	case aiSlot <- struct{}{}:
		defer func() { <-aiSlot }()
	default:
		aiFail(w, http.StatusServiceUnavailable, "AI_FULL", "当前使用人数较多，请稍后再试")
		return
	}

	start := time.Now()
	ratio := s.ratioAt(start)
	ctx, cancel := context.WithTimeout(r.Context(), 170*time.Second)
	defer cancel()
	resp, err := aiUpstream(ctx, s, upMsgs, true, s.MaxOut)
	if err != nil {
		log.Printf("[AI] 连接模型接口失败: %v", err)
		aiFail(w, http.StatusBadGateway, "AI_UPSTREAM", "无法连接 AI 服务，请稍后再试或联系管理员")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("[AI] 模型接口返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		aiFail(w, http.StatusBadGateway, "AI_UPSTREAM", upstreamErrorMessage(resp.StatusCode))
		return
	}

	flusher, ok := w.(http.Flusher)
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	send := func(v interface{}) {
		b, _ := json.Marshal(v)
		rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
		fmt.Fprintf(w, "data: %s\n\n", b)
		if ok {
			flusher.Flush()
		}
	}

	var out strings.Builder
	var usage aiUsageInfo
	truncated := false
	streamErr := ""
	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	for {
		line, rerr := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage json.RawMessage `json:"usage"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil {
				if chunk.Error != nil {
					streamErr = "AI 服务返回错误，请稍后再试"
					log.Printf("[AI] 上游流内错误: %s", chunk.Error.Message)
					break
				}
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" {
						out.WriteString(c.Delta.Content)
						send(map[string]string{"delta": c.Delta.Content})
					}
					if c.FinishReason != nil && *c.FinishReason == "length" {
						truncated = true
					}
				}
				if u, found := parseUsage(chunk.Usage); found {
					usage = u
				}
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				// 客户端中断或网络异常：已生成部分仍按估算计费
				if ctx.Err() == nil {
					streamErr = "AI 服务连接中断，已生成的内容已保留"
				}
			}
			break
		}
	}

	// 结算：优先用上游返回的真实用量，缺失时按字数估算（并在明细中标记“估算”）
	estimated := false
	status := "ok"
	if streamErr != "" || ctx.Err() != nil {
		status = "partial"
	}
	if !usage.Known {
		if out.Len() == 0 {
			// 一个字都没生成：不扣费、不记明细
			if streamErr != "" {
				send(map[string]string{"error": streamErr})
			}
			return
		}
		estimated = true
		usage = aiUsageInfo{Prompt: estTokens(system, 0.7) + int(float64(totalRunes)*0.7), Completion: estTokens(out.String(), 0.7)}
	}
	miss := usage.Prompt - usage.Hit
	if miss < 0 {
		miss = 0
	}
	cost := s.costMicro(usage.Hit, miss, usage.Completion, ratio)
	bal := settleAIUsage(userID, sceneName, s, usage, cost, status, estimated)

	if streamErr != "" {
		send(map[string]string{"error": streamErr})
	}
	send(map[string]interface{}{
		"done":          true,
		"truncated":     truncated,
		"estimated":     estimated,
		"prompt_tokens": usage.Prompt,
		"completion":    usage.Completion,
		"cost":          yuan(cost),
		"balance":       yuan(bal),
		"low":           !s.isFree() && bal < s.WarnMicro,
		"empty":         out.Len() == 0,
	})
}

// ---------------------------------------------------------------------
// 管理员：模型设置
// ---------------------------------------------------------------------

func maskKey(enc string) string {
	if enc == "" {
		return ""
	}
	key, err := aiDecrypt(enc)
	if err != nil || key == "" {
		return "（已保存，但无法解密，请重新填写）"
	}
	r := []rune(key)
	if len(r) <= 8 {
		return "••••••••"
	}
	return "••••••••" + string(r[len(r)-4:])
}

func AIAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	s, err := loadAISettings()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"enabled":           s.Enabled,
		"provider":          s.Provider,
		"base_url":          s.BaseURL,
		"model":             s.Model,
		"has_key":           s.KeyEnc != "",
		"key_masked":        maskKey(s.KeyEnc),
		"price_in_hit":      s.PriceHit,
		"price_in_miss":     s.PriceMiss,
		"price_out":         s.PriceOut,
		"offpeak_ratio":     s.OffpeakRatio,
		"max_output_tokens": s.MaxOut,
		"max_input_chars":   s.MaxInChars,
		"warn_balance":      yuan(s.WarnMicro),
	})
}

func AIAdminSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled      bool    `json:"enabled"`
		Provider     string  `json:"provider"`
		BaseURL      string  `json:"base_url"`
		Model        string  `json:"model"`
		APIKey       string  `json:"api_key"`
		ClearKey     bool    `json:"clear_key"`
		PriceHit     float64 `json:"price_in_hit"`
		PriceMiss    float64 `json:"price_in_miss"`
		PriceOut     float64 `json:"price_out"`
		OffpeakRatio float64 `json:"offpeak_ratio"`
		MaxOut       int     `json:"max_output_tokens"`
		MaxInChars   int     `json:"max_input_chars"`
		WarnBalance  float64 `json:"warn_balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aiFail(w, http.StatusBadRequest, "", "请求格式错误")
		return
	}
	old, err := loadAISettings()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "保存失败")
		return
	}
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.Model = strings.TrimSpace(req.Model)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.Provider != "local" && req.Provider != "remote" {
		aiFail(w, http.StatusBadRequest, "", "请选择模型类型")
		return
	}
	if req.BaseURL != "" && !strings.HasPrefix(req.BaseURL, "http://") && !strings.HasPrefix(req.BaseURL, "https://") {
		aiFail(w, http.StatusBadRequest, "", "接口地址须以 http:// 或 https:// 开头")
		return
	}
	if utf8.RuneCountInString(req.BaseURL) > 300 || utf8.RuneCountInString(req.Model) > 100 || len(req.APIKey) > 500 {
		aiFail(w, http.StatusBadRequest, "", "接口地址、模型名或密钥过长")
		return
	}
	for _, p := range []float64{req.PriceHit, req.PriceMiss, req.PriceOut} {
		if p < 0 || p > 10000 || math.IsNaN(p) {
			aiFail(w, http.StatusBadRequest, "", "单价须在 0 到 10000 元/百万 token 之间")
			return
		}
	}
	if req.OffpeakRatio <= 0 || req.OffpeakRatio > 1 {
		aiFail(w, http.StatusBadRequest, "", "空闲时段折扣系数须大于 0 且不超过 1（不打折填 1，DeepSeek 为 0.5）")
		return
	}
	if req.MaxOut < 64 || req.MaxOut > 32768 {
		aiFail(w, http.StatusBadRequest, "", "单次最大输出须在 64 到 32768 token 之间")
		return
	}
	if req.MaxInChars < 500 || req.MaxInChars > 100000 {
		aiFail(w, http.StatusBadRequest, "", "单次最大输入须在 500 到 100000 字之间")
		return
	}
	if req.WarnBalance < 0 || req.WarnBalance > 10000 {
		aiFail(w, http.StatusBadRequest, "", "低额度提醒线须在 0 到 10000 元之间")
		return
	}
	keyEnc := old.KeyEnc
	switch {
	case req.APIKey != "":
		enc, err := aiEncrypt(req.APIKey)
		if err != nil {
			aiFail(w, http.StatusInternalServerError, "", "密钥加密失败")
			return
		}
		keyEnc = enc
	case req.ClearKey:
		keyEnc = ""
	}
	if req.Enabled {
		if req.BaseURL == "" || req.Model == "" {
			aiFail(w, http.StatusBadRequest, "", "开启前请先填写接口地址和模型名称")
			return
		}
		if req.Provider == "remote" && keyEnc == "" {
			aiFail(w, http.StatusBadRequest, "", "网络接口须填写 API 密钥")
			return
		}
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	if _, err := database.DB.Exec(`UPDATE ai_settings SET enabled=?, provider=?, base_url=?, api_key_enc=?, model=?, price_in_hit=?, price_in_miss=?,
		price_out=?, offpeak_ratio=?, max_output_tokens=?, max_input_chars=?, warn_balance_micro=?, updated_at=CURRENT_TIMESTAMP WHERE id = 1`,
		enabled, req.Provider, req.BaseURL, keyEnc, req.Model, req.PriceHit, req.PriceMiss, req.PriceOut, req.OffpeakRatio,
		req.MaxOut, req.MaxInChars, int64(math.Round(req.WarnBalance*microPerYuan))); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "保存失败")
		return
	}
	state := "关闭"
	if req.Enabled {
		state = "开启"
	}
	label := "网络接口"
	if req.Provider == "local" {
		label = "本地模型"
	}
	keyNote := ""
	if req.APIKey != "" {
		keyNote = "，已更新密钥"
	} else if req.ClearKey {
		keyNote = "，已清除密钥"
	}
	logOperation(r, "AI助手", "修改", fmt.Sprintf("修改 AI 设置：%s，%s，模型 %s%s", state, label, req.Model, keyNote))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已保存"})
}

// AIAdminTest 测试连接：用已保存的设置发一条极短的对话，不计入任何用户的费用
func AIAdminTest(w http.ResponseWriter, r *http.Request) {
	s, err := loadAISettings()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "读取设置失败")
		return
	}
	if s.BaseURL == "" || s.Model == "" {
		aiFail(w, http.StatusBadRequest, "", "请先填写并保存接口地址和模型名称")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	t0 := time.Now()
	resp, err := aiUpstream(ctx, s, []aiMessage{{Role: "user", Content: "请只回复“连接成功”四个字。"}}, false, 32)
	if err != nil {
		middleware.JSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": "无法连接：" + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		middleware.JSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": upstreamErrorMessage(resp.StatusCode)})
		return
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	reply := ""
	if json.Unmarshal(body, &parsed) == nil && len(parsed.Choices) > 0 {
		reply = strings.TrimSpace(parsed.Choices[0].Message.Content)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "连接成功",
		"reply":   reply,
		"ms":      time.Since(t0).Milliseconds(),
	})
}

// ---------------------------------------------------------------------
// 管理员：用户额度 / 加款 / 流水 / 用量
// ---------------------------------------------------------------------

func AIAdminAccounts(w http.ResponseWriter, r *http.Request) {
	p := parsePage(r)
	where := " WHERE u.status = 1"
	args := []interface{}{}
	if kw := strings.TrimSpace(r.URL.Query().Get("keyword")); kw != "" {
		where += " AND (u.real_name LIKE ? OR u.username LIKE ?)"
		args = append(args, "%"+kw+"%", "%"+kw+"%")
	}
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM users u"+where, args...).Scan(&total); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	month := time.Now().In(beijing).Format("2006-01")
	q := `SELECT u.id, u.real_name, u.username, COALESCE(d.name,''), COALESCE(a.balance_micro,0), COALESCE(a.total_topup_micro,0),
		COALESCE(a.total_spent_micro,0), COALESCE(a.total_tokens,0),
		COALESCE((SELECT SUM(cost_micro) FROM ai_usage x WHERE x.user_id = u.id AND datetime(x.created_at,'+8 hours') GLOB ?),0)
		FROM users u LEFT JOIN departments d ON u.department_id = d.id LEFT JOIN ai_accounts a ON a.user_id = u.id` + where +
		` ORDER BY COALESCE(a.total_spent_micro,0) DESC, u.id LIMIT ? OFFSET ?`
	qargs := append([]interface{}{month + "*"}, args...)
	qargs = append(qargs, p.PageSize, (p.Page-1)*p.PageSize)
	rows, err := database.DB.Query(q, qargs...)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type row struct {
		UserID     int64   `json:"user_id"`
		RealName   string  `json:"real_name"`
		Username   string  `json:"username"`
		Department string  `json:"department"`
		Balance    float64 `json:"balance"`
		TotalTopup float64 `json:"total_topup"`
		TotalSpent float64 `json:"total_spent"`
		Tokens     int64   `json:"total_tokens"`
		MonthSpent float64 `json:"month_spent"`
	}
	list := []row{}
	for rows.Next() {
		var it row
		var bal, top, spent, mspent int64
		if err := rows.Scan(&it.UserID, &it.RealName, &it.Username, &it.Department, &bal, &top, &spent, &it.Tokens, &mspent); err != nil {
			continue
		}
		it.Balance, it.TotalTopup, it.TotalSpent, it.MonthSpent = yuan(bal), yuan(top), yuan(spent), yuan(mspent)
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	var allBal, allTop, allSpent int64
	database.DB.QueryRow("SELECT COALESCE(SUM(balance_micro),0), COALESCE(SUM(total_topup_micro),0), COALESCE(SUM(total_spent_micro),0) FROM ai_accounts").Scan(&allBal, &allTop, &allSpent)
	res := paginateResult(list, total, p.Page, p.PageSize)
	res["summary"] = map[string]float64{"balance": yuan(allBal), "total_topup": yuan(allTop), "total_spent": yuan(allSpent)}
	middleware.JSON(w, http.StatusOK, res)
}

// AIAdminTopup 管理员给用户加款（线下微信转账后手工记账）；金额为负表示更正扣减，扣减后余额不得为负
func AIAdminTopup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int64   `json:"user_id"`
		Amount float64 `json:"amount"`
		Remark string  `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aiFail(w, http.StatusBadRequest, "", "请求格式错误")
		return
	}
	req.Remark = strings.TrimSpace(req.Remark)
	if req.UserID <= 0 || req.Amount == 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) {
		aiFail(w, http.StatusBadRequest, "", "请选择用户并填写非零金额")
		return
	}
	if math.Abs(req.Amount) > 100000 {
		aiFail(w, http.StatusBadRequest, "", "单次金额不能超过 100000 元")
		return
	}
	if utf8.RuneCountInString(req.Remark) > 100 {
		aiFail(w, http.StatusBadRequest, "", "备注请控制在 100 字以内")
		return
	}
	micro := int64(math.Round(req.Amount * microPerYuan))
	if micro == 0 {
		aiFail(w, http.StatusBadRequest, "", "金额最多精确到 0.000001 元")
		return
	}
	var name string
	if err := database.DB.QueryRow("SELECT real_name FROM users WHERE id = ? AND status = 1", req.UserID).Scan(&name); err != nil {
		aiFail(w, http.StatusBadRequest, "", "用户不存在或已被禁用")
		return
	}
	operator, _ := r.Context().Value(middleware.ContextRealName).(string)

	tx, err := database.DB.Begin()
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec("INSERT OR IGNORE INTO ai_accounts (user_id) VALUES (?)", req.UserID); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	var bal int64
	if err := tx.QueryRow("SELECT balance_micro FROM ai_accounts WHERE user_id = ?", req.UserID).Scan(&bal); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	if bal+micro < 0 {
		aiFail(w, http.StatusBadRequest, "", fmt.Sprintf("扣减后余额将为负，当前余额仅 %.2f 元", yuan(bal)))
		return
	}
	kind := "topup"
	topupAdd := micro
	if micro < 0 {
		kind = "adjust"
		topupAdd = 0
	}
	if _, err := tx.Exec(`UPDATE ai_accounts SET balance_micro = balance_micro + ?, total_topup_micro = total_topup_micro + ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`,
		micro, topupAdd, req.UserID); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	after := bal + micro
	if _, err := tx.Exec(`INSERT INTO ai_ledger (user_id, kind, amount_micro, balance_after_micro, operator, remark) VALUES (?, ?, ?, ?, ?, ?)`,
		req.UserID, kind, micro, after, operator, req.Remark); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	if err := tx.Commit(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "操作失败")
		return
	}
	word := "加款"
	if micro < 0 {
		word = "扣减更正"
	}
	logOperation(r, "AI助手", word, fmt.Sprintf("为「%s」%s %.2f 元，余额 %.2f 元，备注：%s", name, word, math.Abs(yuan(micro)), yuan(after), req.Remark))
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "已记账", "balance": yuan(after)})
}

func AIAdminLedger(w http.ResponseWriter, r *http.Request) {
	p := parsePage(r)
	where := " WHERE 1=1"
	args := []interface{}{}
	if uid, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64); uid > 0 {
		where += " AND l.user_id = ?"
		args = append(args, uid)
	}
	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM ai_ledger l"+where, args...).Scan(&total); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	rows, err := database.DB.Query(`SELECT l.id, datetime(l.created_at,'+8 hours'), l.user_id, COALESCE(u.real_name,'（已删除）'), l.kind, l.amount_micro, l.balance_after_micro, l.operator, l.remark
		FROM ai_ledger l LEFT JOIN users u ON u.id = l.user_id`+where+` ORDER BY l.id DESC LIMIT ? OFFSET ?`,
		append(args, p.PageSize, (p.Page-1)*p.PageSize)...)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type row struct {
		ID       int64   `json:"id"`
		Time     string  `json:"time"`
		UserID   int64   `json:"user_id"`
		RealName string  `json:"real_name"`
		Kind     string  `json:"kind"`
		Amount   float64 `json:"amount"`
		After    float64 `json:"balance_after"`
		Operator string  `json:"operator"`
		Remark   string  `json:"remark"`
	}
	list := []row{}
	for rows.Next() {
		var it row
		var amt, after int64
		var op, remark sql.NullString
		if err := rows.Scan(&it.ID, &it.Time, &it.UserID, &it.RealName, &it.Kind, &amt, &after, &op, &remark); err != nil {
			continue
		}
		it.Amount, it.After, it.Operator, it.Remark = yuan(amt), yuan(after), op.String, remark.String
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}

func AIAdminUsage(w http.ResponseWriter, r *http.Request) {
	p := parsePage(r)
	where := " WHERE 1=1"
	args := []interface{}{}
	if uid, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64); uid > 0 {
		where += " AND x.user_id = ?"
		args = append(args, uid)
	}
	if m := strings.TrimSpace(r.URL.Query().Get("month")); len(m) == 7 {
		where += " AND datetime(x.created_at,'+8 hours') GLOB ?"
		args = append(args, m+"*")
	}
	var total int
	var sumCost, sumTokens int64
	if err := database.DB.QueryRow("SELECT COUNT(*), COALESCE(SUM(x.cost_micro),0), COALESCE(SUM(x.prompt_tokens+x.completion_tokens),0) FROM ai_usage x"+where, args...).Scan(&total, &sumCost, &sumTokens); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	rows, err := database.DB.Query(`SELECT datetime(x.created_at,'+8 hours'), COALESCE(u.real_name,'（已删除）'), x.scene, x.provider, x.model, x.prompt_tokens, x.completion_tokens, x.cost_micro, x.status, x.estimated
		FROM ai_usage x LEFT JOIN users u ON u.id = x.user_id`+where+` ORDER BY x.id DESC LIMIT ? OFFSET ?`,
		append(args, p.PageSize, (p.Page-1)*p.PageSize)...)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type row struct {
		Time       string  `json:"time"`
		RealName   string  `json:"real_name"`
		Scene      string  `json:"scene"`
		Provider   string  `json:"provider"`
		Model      string  `json:"model"`
		Prompt     int     `json:"prompt_tokens"`
		Completion int     `json:"completion_tokens"`
		Cost       float64 `json:"cost"`
		Status     string  `json:"status"`
		Estimated  bool    `json:"estimated"`
	}
	list := []row{}
	for rows.Next() {
		var it row
		var scene, provider, model, status sql.NullString
		var cost int64
		var est int
		if err := rows.Scan(&it.Time, &it.RealName, &scene, &provider, &model, &it.Prompt, &it.Completion, &cost, &status, &est); err != nil {
			continue
		}
		it.Scene, it.Provider, it.Model, it.Status, it.Cost, it.Estimated = scene.String, provider.String, model.String, status.String, yuan(cost), est == 1
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	res := paginateResult(list, total, p.Page, p.PageSize)
	res["summary"] = map[string]interface{}{"cost": yuan(sumCost), "tokens": sumTokens, "calls": total}
	middleware.JSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------
// 管理员：提示词库
// ---------------------------------------------------------------------

func AIAdminListPrompts(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT id, scene_key, name, category, system_prompt, hint, placeholder, sort, status, builtin FROM ai_prompts ORDER BY sort, id`)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	defer rows.Close()
	type item struct {
		ID           int64  `json:"id"`
		Key          string `json:"scene_key"`
		Name         string `json:"name"`
		Category     string `json:"category"`
		SystemPrompt string `json:"system_prompt"`
		Hint         string `json:"hint"`
		Placeholder  string `json:"placeholder"`
		Sort         int    `json:"sort"`
		Status       int    `json:"status"`
		Builtin      int    `json:"builtin"`
	}
	list := []item{}
	for rows.Next() {
		var it item
		var key, cat, hint, ph sql.NullString
		if err := rows.Scan(&it.ID, &key, &it.Name, &cat, &it.SystemPrompt, &hint, &ph, &it.Sort, &it.Status, &it.Builtin); err != nil {
			continue
		}
		it.Key, it.Category, it.Hint, it.Placeholder = key.String, cat.String, hint.String, ph.String
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "查询失败")
		return
	}
	middleware.JSON(w, http.StatusOK, list)
}

type aiPromptReq struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	SystemPrompt string `json:"system_prompt"`
	Hint         string `json:"hint"`
	Placeholder  string `json:"placeholder"`
	Sort         int    `json:"sort"`
	Status       int    `json:"status"`
}

func (q *aiPromptReq) validate() string {
	q.Name = strings.TrimSpace(q.Name)
	q.Category = strings.TrimSpace(q.Category)
	q.SystemPrompt = strings.TrimSpace(q.SystemPrompt)
	q.Hint = strings.TrimSpace(q.Hint)
	q.Placeholder = strings.TrimSpace(q.Placeholder)
	if q.Name == "" || utf8.RuneCountInString(q.Name) > 30 {
		return "场景名称不能为空且不超过 30 字"
	}
	if q.SystemPrompt == "" || utf8.RuneCountInString(q.SystemPrompt) > 3000 {
		return "提示词不能为空且不超过 3000 字"
	}
	if utf8.RuneCountInString(q.Category) > 20 || utf8.RuneCountInString(q.Hint) > 100 || utf8.RuneCountInString(q.Placeholder) > 200 {
		return "分类、说明或输入示例过长"
	}
	if q.Status != 0 {
		q.Status = 1
	}
	return ""
}

func AIAdminCreatePrompt(w http.ResponseWriter, r *http.Request) {
	var q aiPromptReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		aiFail(w, http.StatusBadRequest, "", "请求格式错误")
		return
	}
	q.Status = 1
	if msg := q.validate(); msg != "" {
		aiFail(w, http.StatusBadRequest, "", msg)
		return
	}
	if _, err := database.DB.Exec(`INSERT INTO ai_prompts (scene_key, name, category, system_prompt, hint, placeholder, sort, status) VALUES ('', ?, ?, ?, ?, ?, ?, 1)`,
		q.Name, q.Category, q.SystemPrompt, q.Hint, q.Placeholder, q.Sort); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "保存失败")
		return
	}
	logOperation(r, "AI助手", "新增", "新增提示词场景「"+q.Name+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已保存"})
}

func AIAdminUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	var q aiPromptReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil || q.ID <= 0 {
		aiFail(w, http.StatusBadRequest, "", "请求格式错误")
		return
	}
	if msg := q.validate(); msg != "" {
		aiFail(w, http.StatusBadRequest, "", msg)
		return
	}
	res, err := database.DB.Exec(`UPDATE ai_prompts SET name=?, category=?, system_prompt=?, hint=?, placeholder=?, sort=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id = ?`,
		q.Name, q.Category, q.SystemPrompt, q.Hint, q.Placeholder, q.Sort, q.Status, q.ID)
	if err != nil {
		aiFail(w, http.StatusInternalServerError, "", "保存失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		aiFail(w, http.StatusNotFound, "", "场景不存在")
		return
	}
	logOperation(r, "AI助手", "修改", "修改提示词场景「"+q.Name+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已保存"})
}

func AIAdminDeletePrompt(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var name string
	var builtin int
	if err := database.DB.QueryRow("SELECT name, builtin FROM ai_prompts WHERE id = ?", id).Scan(&name, &builtin); err != nil {
		aiFail(w, http.StatusNotFound, "", "场景不存在")
		return
	}
	if builtin == 1 {
		aiFail(w, http.StatusBadRequest, "", "系统预置场景不能删除，可以改为停用")
		return
	}
	if _, err := database.DB.Exec("DELETE FROM ai_prompts WHERE id = ?", id); err != nil {
		aiFail(w, http.StatusInternalServerError, "", "删除失败")
		return
	}
	logOperation(r, "AI助手", "删除", "删除提示词场景「"+name+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}
