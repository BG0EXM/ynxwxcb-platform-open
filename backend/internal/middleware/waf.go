package middleware

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"ynxwxcb-platform/internal/database"
)

// 注入/跨站特征（不区分大小写）：
//   - select…from 之间只允许出现字段列表字符（字母数字、*、逗号、表名.字段名等），中文或句子间的 select…from 不会误判；
//   - 覆盖 <script 带属性、事件处理器（onerror= 等）、iframe 与 javascript: 伪协议。
var wafRegex = regexp.MustCompile(`(?is)(\bselect\b(?:[\w*,()\x60"'\[\]\s]|\.\w){1,200}?\bfrom\b|\binsert\s+into\b|\bupdate\b\s+[\w\x60"\[\]]+\s+set\b|\bdelete\s+from\b|\bdrop\s+table\b|\bunion\b(\s+all)?\s+select\b|<\s*script\b|<\s*iframe\b|<[a-z][^>]*\son[a-z]+\s*=|javascript\s*:|\balert\s*\()`)

// sqlCommentRegex 剔除 /**/ 注释，防止 select/**/x/**/from 之类的注释混淆绕过
var sqlCommentRegex = regexp.MustCompile(`/\*.*?\*/`)

// wafMatch 对原文及剔除注释后的文本分别检测
func wafMatch(s string) bool {
	if s == "" {
		return false
	}
	return wafRegex.MatchString(s) || wafRegex.MatchString(sqlCommentRegex.ReplaceAllString(s, " "))
}

// WAF 拦截日志限频：同一 IP 每 10 秒最多记 1 条，全局每分钟最多 60 条，防止被刷屏撑爆审计日志
var (
	wafLogMu      sync.Mutex
	wafLogLast    = map[string]time.Time{}
	wafLogWindow  time.Time
	wafLogCounter int
)

func wafShouldLog(ip string) bool {
	wafLogMu.Lock()
	defer wafLogMu.Unlock()
	now := time.Now()
	if now.Sub(wafLogWindow) > time.Minute {
		wafLogWindow = now
		wafLogCounter = 0
		// 顺带清理过期的 IP 记录，防止内存只增不减
		for k, t := range wafLogLast {
			if now.Sub(t) > time.Minute {
				delete(wafLogLast, k)
			}
		}
	}
	if wafLogCounter >= 60 {
		return false
	}
	if t, ok := wafLogLast[ip]; ok && now.Sub(t) < 10*time.Second {
		return false
	}
	wafLogLast[ip] = now
	wafLogCounter++
	return true
}

func WAF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. 检查 URL 路径及查询参数（同时检查原始与解码后形式，防止 URL 编码绕过）
		decodedQuery, _ := url.QueryUnescape(r.URL.RawQuery)
		decodedPath, _ := url.PathUnescape(r.URL.Path)

		// URL 解码失败（如 %zz）时用原文检测，不能以空串放行
		if decodedQuery == "" {
			decodedQuery = r.URL.RawQuery
		}
		if decodedPath == "" {
			decodedPath = r.URL.Path
		}
		if wafMatch(r.URL.Path) ||
			wafMatch(r.URL.RawQuery) ||
			wafMatch(decodedQuery) ||
			wafMatch(decodedPath) {
			blockWAF(w, r)
			return
		}

		// 2. 检查 POST/PUT/PATCH/DELETE 请求体；仅两个真实文件上传接口的 multipart 请求体交由上传安全与涉密引擎处理，
		//    其余接口即使伪装成 multipart 也照常检测，杜绝改 Content-Type 绕过
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			contentType := r.Header.Get("Content-Type")
			skip := strings.HasPrefix(contentType, "multipart/form-data") && IsUploadRequest(r)
			if !skip && r.Body != nil {
				// 限制最多读取 1MB 用于检测，防止超大请求体耗尽内存
				bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				// 无论读取是否出错都要把已读部分放回，保证后续 handler 读到完整请求体
				r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), r.Body))
				if err == nil {
					if wafMatch(string(bodyBytes)) {
						blockWAF(w, r)
						return
					}
					// JSON 中的 \u003c 等转义同样要还原后逐个字符串值检测
					if bytes.Contains(bodyBytes, []byte(`\u`)) && jsonStringsMatch(bodyBytes) {
						blockWAF(w, r)
						return
					}
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func blockWAF(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r)

	// 尝试获取登录用户信息
	userID, _ := r.Context().Value(ContextUserID).(int64)
	username, _ := r.Context().Value(ContextUsername).(string)
	realName, _ := r.Context().Value(ContextRealName).(string)

	operatorName := "外部访问者"
	department := "网络安全防线"
	if username != "" {
		operatorName = username
		if realName != "" {
			operatorName = realName + " (" + username + ")"
		}
		department = "已认证会话"
	}

	// 真实写入系统操作审计日志 (operation_logs)，按 IP 限频
	logIt := wafShouldLog(ip)
	go func(uid int64, uname, dept, clientIP, path, rawQuery string) {
		if !logIt {
			return
		}
		detail := "检测到非法注入/跨站探测攻击 (路由: " + path
		if rawQuery != "" {
			detail += "，参数: " + rawQuery
		}
		detail += ")"
		if len(detail) > 500 {
			detail = detail[:500] + "..."
		}
		_, err := database.DB.Exec(
			`INSERT INTO operation_logs (user_id, user_name, department, module, action, detail, ip, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now','localtime'))`,
			uid, uname, dept, "系统安全", "WAF拦截", detail, clientIP,
		)
		if err != nil {
			log.Printf("[WAF] 写入操作日志失败: %v", err)
		}
	}(userID, operatorName, department, ip, r.URL.Path, r.URL.RawQuery)

	accept := r.Header.Get("Accept")
	// 浏览器地址栏直接访问或普通网页访问时，返回 UTF-8 格式的专用中文安全阻断页
	if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>安全拦截 - 伊宁县委宣传部网络安全防御系统</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif; background: #0b0f19; color: #f0f3f8; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #151b2b; border: 1px solid rgba(239, 68, 68, 0.4); border-radius: 12px; padding: 40px; max-width: 520px; box-shadow: 0 10px 40px rgba(239, 68, 68, 0.15); text-align: center; }
    .icon { font-size: 56px; line-height: 1; margin-bottom: 16px; }
    h1 { font-size: 20px; margin: 0 0 12px; color: #f87171; font-weight: 600; }
    p { font-size: 14px; color: #94a3b8; line-height: 1.6; margin: 0 0 24px; text-align: justify; }
    .meta { background: #0c101c; border: 1px solid #1e293b; border-radius: 8px; padding: 14px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 13px; color: #cbd5e1; text-align: left; }
    .meta div { margin-bottom: 6px; }
    .meta div:last-child { margin-bottom: 0; }
    .tag { color: #ef4444; font-weight: bold; }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">🛡️</div>
    <h1>请求已被系统安全防线实时拦截</h1>
    <p>伊宁县委宣传部工作平台应用级防火墙 (WAF) 检测到当前请求中包含非法 SQL 注入或跨站脚本攻击特征指令。为保障党政政务平台数据安全，该请求已被阻断并实时记录至安全审计日志。</p>
    <div class="meta">
      <div>安全状态: <span class="tag">WAF_BLOCK (403 Forbidden)</span></div>
      <div>来源IP: ` + html.EscapeString(ip) + `</div>
      <div>拦截路由: ` + html.EscapeString(r.URL.Path) + `</div>
    </div>
  </div>
</body>
</html>`
		w.Write([]byte(html))
		return
	}

	// API 或 Ajax 请求返回 UTF-8 编码的 JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{"error": "检测到非法注入攻击，操作已被拦截并记录", "code": "WAF_BLOCK", "ip": ip})
}

// jsonStringsMatch 解析 JSON 请求体，对其中全部字符串值（已还原转义）做特征检测
func jsonStringsMatch(body []byte) bool {
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		return false
	}
	var walk func(x interface{}) bool
	walk = func(x interface{}) bool {
		switch t := x.(type) {
		case string:
			return wafMatch(t)
		case []interface{}:
			for _, e := range t {
				if walk(e) {
					return true
				}
			}
		case map[string]interface{}:
			for k, e := range t {
				if wafMatch(k) || walk(e) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}
