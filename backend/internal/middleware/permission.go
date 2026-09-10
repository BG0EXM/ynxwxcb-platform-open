package middleware

import (
	"database/sql"
	"net/http"
	"sync"

	"ynxwxcb-platform/internal/database"
)

// 角色-权限缓存：roleCode -> 权限点集合
var (
	permMu    sync.RWMutex
	permCache map[string]map[string]bool
)

// ReloadPermissions 从数据库重新加载角色-权限映射到内存缓存
// 启动时调用一次，权限矩阵保存后再次调用
func ReloadPermissions() {
	m := map[string]map[string]bool{}
	rows, err := database.DB.Query(
		`SELECT r.code, rp.permission_code FROM roles r
		 LEFT JOIN role_permissions rp ON rp.role_id = r.id`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var roleCode string
		var perm sql.NullString
		if err := rows.Scan(&roleCode, &perm); err != nil {
			continue
		}
		if m[roleCode] == nil {
			m[roleCode] = map[string]bool{}
		}
		if perm.Valid {
			m[roleCode][perm.String] = true
		}
	}
	if err := rows.Err(); err != nil {
		return
	}
	permMu.Lock()
	permCache = m
	permMu.Unlock()
}

// hasPermission 判断角色是否拥有某权限点（admin 始终通过）
func hasPermission(roleCode, code string) bool {
	if roleCode == "admin" {
		return true
	}
	permMu.RLock()
	defer permMu.RUnlock()
	if permCache == nil {
		return false
	}
	return permCache[roleCode][code]
}

// RequirePerm 权限点校验中间件（拥有任一权限点即放行）
func RequirePerm(codes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleCode, _ := r.Context().Value(ContextRoleCode).(string)
			for _, c := range codes {
				if hasPermission(roleCode, c) {
					next.ServeHTTP(w, r)
					return
				}
			}
			// 返回 JSON（前端据此弹出友好提示）
			JSON(w, http.StatusForbidden, map[string]string{"error": "无权限访问该功能"})
		})
	}
}

// RolePermissions 返回指定角色的权限点集合（供登录/资料接口返回给前端；admin 返回全部）
func RolePermissions(roleCode string) []string {
	if roleCode == "admin" {
		codes := make([]string, 0, len(database.PermissionCatalog))
		for _, p := range database.PermissionCatalog {
			codes = append(codes, p.Code)
		}
		return codes
	}
	permMu.RLock()
	defer permMu.RUnlock()
	codes := []string{}
	if permCache != nil {
		for c := range permCache[roleCode] {
			codes = append(codes, c)
		}
	}
	return codes
}
