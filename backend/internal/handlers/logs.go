package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// logOperation 记录一条操作日志（尽力而为，任何失败都不影响主流程）
// module: 模块名（如"收文管理"）；action: 动作（新增/修改/删除/导出/登录）；
// detail: 人类可读的中文描述（如"删除用户「张三」"）
func logOperation(r *http.Request, module, action, detail string) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	realName, _ := r.Context().Value(middleware.ContextRealName).(string)
	logWithUser(userID, realName, module, action, detail, clientIP(r))
}

// logWithUser 按指定用户写日志（登录等上下文尚无用户的场景用）
func logWithUser(userID int64, userName, module, action, detail, ip string) {
	dept := ""
	if userID != 0 {
		var name, d sql.NullString
		if err := database.DB.QueryRow(
			`SELECT u.real_name, d.name FROM users u LEFT JOIN departments d ON u.department_id = d.id WHERE u.id = ?`, userID).
			Scan(&name, &d); err == nil {
			if name.Valid && name.String != "" {
				userName = name.String
			}
			dept = d.String
		}
	}
	database.DB.Exec(
		`INSERT INTO operation_logs (user_id, user_name, department, module, action, detail, ip, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now','localtime'))`,
		userID, userName, dept, module, action, detail, ip)
}

// clientIP 提取客户端 IP（委托给中间件统一实现）
func clientIP(r *http.Request) string {
	return middleware.ClientIP(r)
}

// CleanupOldLogs 清理超过 1 年的操作日志（启动时与每日定时调用）
func CleanupOldLogs() {
	database.DB.Exec("DELETE FROM operation_logs WHERE created_at < datetime('now', '-1 year', 'localtime')")
}

// ListOperationLogs 操作日志列表（仅管理员），支持模块/动作/日期/关键词筛选
func ListOperationLogs(w http.ResponseWriter, r *http.Request) {
	module := r.URL.Query().Get("module")
	action := r.URL.Query().Get("action")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	keyword := r.URL.Query().Get("keyword")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if module != "" {
		where += ` AND module = ?`
		args = append(args, module)
	}
	if action != "" {
		where += ` AND action = ?`
		args = append(args, action)
	}
	if start != "" {
		where += ` AND created_at >= ?`
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		where += ` AND created_at <= ?`
		args = append(args, end+" 23:59:59")
	}
	if keyword != "" {
		where += ` AND (user_name LIKE ? OR detail LIKE ?)`
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}

	p := parsePage(r)

	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM operation_logs"+where, args...).Scan(&total); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	query := `SELECT id, user_id, user_name, department, module, action, detail, ip, created_at FROM operation_logs` +
		where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type logRow struct {
		ID         int64  `json:"id"`
		UserID     int64  `json:"user_id"`
		UserName   string `json:"user_name"`
		Department string `json:"department"`
		Module     string `json:"module"`
		Action     string `json:"action"`
		Detail     string `json:"detail"`
		IP         string `json:"ip"`
		CreatedAt  string `json:"created_at"`
	}
	list := []logRow{}
	for rows.Next() {
		var it logRow
		var userName, department, moduleName, actionName, detail, ip sql.NullString
		var createdAt sql.NullString
		if err := rows.Scan(&it.ID, &it.UserID, &userName, &department, &moduleName, &actionName, &detail, &ip, &createdAt); err != nil {
			continue
		}
		it.UserName = userName.String
		it.Department = department.String
		it.Module = moduleName.String
		it.Action = actionName.String
		it.Detail = detail.String
		it.IP = ip.String
		// 规范化时间显示：去掉驱动返回的 T/Z，统一为 YYYY-MM-DD HH:MM:SS
		ca := strings.Replace(createdAt.String, "T", " ", 1)
		ca = strings.TrimSuffix(ca, "Z")
		it.CreatedAt = ca
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}
