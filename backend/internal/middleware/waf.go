package middleware

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
	"time"

	"ynxwxcb-platform/internal/database"
)

// (?i) makes it case-insensitive
var wafRegex = regexp.MustCompile(`(?i)(select\s+.*\s+from|insert\s+into|update\s+.*\s+set|delete\s+from|drop\s+table|union\s+select|<script>|javascript:|alert\()`)

func WAF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check URL path and query
		if wafRegex.MatchString(r.URL.Path) || wafRegex.MatchString(r.URL.RawQuery) {
			blockWAF(w, r)
			return
		}

		// 2. Check Body if POST/PUT
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			if r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					// Restore body for next handlers
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
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
	
	// Try to get user info if already authenticated, else fallback
	userID, _ := r.Context().Value(ContextUserID).(int64)
	username, _ := r.Context().Value(ContextUsername).(string)
	realName, _ := r.Context().Value(ContextRealName).(string)
	
	if username == "" {
		username = "未知(WAF拦截)"
		realName = "恶意探测者"
	}

	// 异步记录审计日志
	go func(uid int64, uname, rname, ip, path string) {
		database.DB.Exec(
			`INSERT INTO operation_logs (user_id, username, real_name, ip_address, module, action, detail, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			uid, uname, rname, ip, "系统安全", "WAF拦截", "检测到非法注入/跨站探测攻击 (路由: "+path+")", time.Now(),
		)
	}(userID, username, realName, ip, r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error": "检测到非法注入攻击，操作已被拦截并记录", "code": "WAF_BLOCK"}`))
}
