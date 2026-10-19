package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ynxwxcb-platform/internal/auth"
)

type contextKey string

const (
	ContextUserID   contextKey = "user_id"
	ContextUsername contextKey = "username"
	ContextRealName contextKey = "real_name"
	ContextRoleCode contextKey = "role_code"
)

// Auth 认证中间件，校验 JWT
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		tokenStr := ""
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				JSON(w, http.StatusUnauthorized, map[string]string{"error": "认证格式错误"})
				return
			}
			tokenStr = parts[1]
		}
		// 不再接受网址参数 ?token=：令牌会被写进访问日志、浏览器历史与 Referer，只允许 Authorization 请求头
		if tokenStr == "" {
			JSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			JSON(w, http.StatusUnauthorized, map[string]string{"error": "登录已过期，请重新登录"})
			return
		}
		// 令牌版本校验：改密/禁用/登出后旧令牌立即失效；用户名必须与库内一致，防止 ID 复用后旧令牌复活
		u, ok := loadSessionUser(claims.UserID)
		if !ok || u.TokenVersion != claims.TokenVersion || u.Username != claims.Username {
			JSON(w, http.StatusUnauthorized, map[string]string{"error": "登录已失效，请重新登录"})
			return
		}
		if u.Status != 1 {
			JSON(w, http.StatusUnauthorized, map[string]string{"error": "账号已被禁用，请联系管理员"})
			return
		}
		// 仍在使用默认密码：服务端强制先改密，仅放行个人信息、修改密码与退出登录
		if u.MustChange && !mustChangeAllowed(r.URL.Path) {
			JSON(w, http.StatusForbidden, map[string]string{"error": "当前仍在使用默认密码，请先修改密码", "code": "MUST_CHANGE_PASSWORD"})
			return
		}

		// 身份与角色一律以数据库实时状态为准，不信任令牌中携带的角色与姓名
		ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextUsername, u.Username)
		ctx = context.WithValue(ctx, ContextRealName, u.RealName)
		ctx = context.WithValue(ctx, ContextRoleCode, u.RoleCode)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// mustChangeAllowed 默认密码用户在改密前允许访问的接口
func mustChangeAllowed(path string) bool {
	switch path {
	case "/api/auth/profile", "/api/auth/change-password", "/api/auth/logout":
		return true
	}
	return false
}

// RequireRole 角色校验中间件
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleCode, _ := r.Context().Value(ContextRoleCode).(string)
			for _, role := range roles {
				if role == roleCode {
					next.ServeHTTP(w, r)
					return
				}
			}
			JSON(w, http.StatusForbidden, map[string]string{"error": "无权访问"})
		})
	}
}

// JSON 输出 JSON 响应
func JSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}
