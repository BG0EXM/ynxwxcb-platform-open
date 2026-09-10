package handlers

import (
	"encoding/json"
	"net/http"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// GetPermissionMatrix 返回权限点目录 + 角色列表 + 当前角色-权限矩阵（仅管理员）
func GetPermissionMatrix(w http.ResponseWriter, r *http.Request) {
	// 角色列表
	roleRows, err := database.DB.Query("SELECT id, name, code, COALESCE(description,'') FROM roles ORDER BY id")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	roles := []models.Role{}
	for roleRows.Next() {
		var role models.Role
		if err := roleRows.Scan(&role.ID, &role.Name, &role.Code, &role.Description); err != nil {
			continue
		}
		roles = append(roles, role)
	}
	if err := roleRows.Err(); err != nil {
		roleRows.Close()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	roleRows.Close()

	// 角色-权限映射（按角色 code 聚合）
	matrix := map[string][]string{}
	for _, role := range roles {
		matrix[role.Code] = []string{}
	}
	permRows, err := database.DB.Query(
		`SELECT r.code, rp.permission_code FROM role_permissions rp JOIN roles r ON rp.role_id = r.id`)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	for permRows.Next() {
		var roleCode, code string
		if err := permRows.Scan(&roleCode, &code); err != nil {
			continue
		}
		matrix[roleCode] = append(matrix[roleCode], code)
	}
	if err := permRows.Err(); err != nil {
		permRows.Close()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	permRows.Close()

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"permissions": database.PermissionCatalog,
		"roles":       roles,
		"matrix":      matrix,
	})
}

// SavePermissionMatrix 保存角色-权限矩阵（仅管理员）
// body: {"roles":[{"role_id":2,"codes":["contact.view",...]}]}
func SavePermissionMatrix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Roles []struct {
			RoleID int64    `json:"role_id"`
			Codes  []string `json:"codes"`
		} `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	// 合法权限点集合
	valid := map[string]bool{}
	for _, p := range database.PermissionCatalog {
		valid[p.Code] = true
	}
	// 角色 id -> code（用于保护 admin）
	roleCodeByID := map[int64]string{}
	rrows, err := database.DB.Query("SELECT id, code FROM roles")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	for rrows.Next() {
		var id int64
		var code string
		if err := rrows.Scan(&id, &code); err == nil {
			roleCodeByID[id] = code
		}
	}
	rrows.Close()

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	for _, rp := range req.Roles {
		// admin 角色锁定，忽略对其的修改（代码层旁路，始终全权限）
		if roleCodeByID[rp.RoleID] == "admin" {
			continue
		}
		if _, err := tx.Exec("DELETE FROM role_permissions WHERE role_id = ?", rp.RoleID); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
			return
		}
		for _, c := range rp.Codes {
			if !valid[c] {
				continue
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO role_permissions (role_id, permission_code) VALUES (?, ?)", rp.RoleID, c); err != nil {
				tx.Rollback()
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}

	// 重新加载缓存
	middleware.ReloadPermissions()

	logOperation(r, "系统管理", "修改", "修改角色-功能权限矩阵")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "保存成功"})
}
