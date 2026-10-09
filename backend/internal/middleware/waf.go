package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"ynxwxcb-platform/internal/database"
)

// (?i) makes it case-insensitive
var wafRegex = regexp.MustCompile(`(?i)(select\s+.*\s+from|insert\s+into|update\s+.*\s+set|delete\s+from|drop\s+table|union\s+select|<script>|javascript:|alert\()`)

func WAF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. 检查 URL 路径及查询参数（同时检查原始与解码后形式，防止 URL 编码绕过）
		decodedQuery, _ := url.QueryUnescape(r.URL.RawQuery)
		decodedPath, _ := url.PathUnescape(r.URL.Path)

		if wafRegex.MatchString(r.URL.Path) ||
			wafRegex.MatchString(r.URL.RawQuery) ||
			wafRegex.MatchString(decodedQuery) ||
			wafRegex.MatchString(decodedPath) {
			blockWAF(w, r)
			return
		}

		// 2. 检查 POST/PUT 请求体（排除 multipart/form-data 文件上传，由专用上传安全与涉密引擎处理）
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			contentType := r.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, "multipart/form-data") && r.Body != nil {
				// 限制最多读取 1MB 用于检测，防止超大请求体耗尽内存
				lr := io.LimitReader(r.Body, 1<<20)
				bodyBytes, err := io.ReadAll(lr)
				if err == nil {
					// 恢复 body 供后续 handler 读取
					r.Body = io.NopCloser(io.MultiReader(bytes.NewBuffer(bodyBytes), r.Body))
					if wafRegex.MatchString(string(bodyBytes)) {
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

	// 真实写入系统操作审计日志 (operation_logs)
	go func(uid int64, uname, dept, clientIP, path, rawQuery string) {
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
      <div>来源IP: ` + ip + `</div>
      <div>拦截路由: ` + r.URL.Path + `</div>
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
	w.Write([]byte(`{"error": "检测到非法注入攻击，操作已被拦截并记录", "code": "WAF_BLOCK", "ip": "` + ip + `"}`))
}
