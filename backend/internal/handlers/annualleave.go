package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListAnnualLeaveConfigs 年休假统计（按年）
// 管理员看全部；普通用户只看自己。每人显示：配置天数、已休天数（联动请假 annual）、剩余天数
func ListAnnualLeaveConfigs(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	if year == "" {
		year = time.Now().Format("2006")
	}
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	// 查询所有启用用户 + 年休假配置
	query := `SELECT u.id, u.real_name, d.name,
			COALESCE(c.days, 0)
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN annual_leave_configs c ON c.user_id = u.id AND c.year = ?
		WHERE u.status = 1`
	args := []interface{}{year}
	if roleCode != "admin" {
		query += ` AND u.id = ?`
		args = append(args, userID)
	}
	query += ` GROUP BY u.id ORDER BY u.id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	type Row struct {
		UserID     int64   `json:"user_id"`
		UserName   string  `json:"user_name"`
		Department string  `json:"department"`
		ConfigDays float64 `json:"config_days"`
		UsedDays   float64 `json:"used_days"`
		RemainDays float64 `json:"remain_days"`
	}
	base := map[int64]*Row{}
	var userIDs []int64
	for rows.Next() {
		var rw Row
		var dept sql.NullString
		rows.Scan(&rw.UserID, &rw.UserName, &dept, &rw.ConfigDays)
		if dept.Valid {
			rw.Department = dept.String
		}
		base[rw.UserID] = &rw
		userIDs = append(userIDs, rw.UserID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 查询已休年假天数（leave_type='annual'，按当年实际覆盖天数，跨年假期用 julianday 拆分）
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	usedQuery := `SELECT user_id, SUM(overlap_days) FROM (
			SELECT user_id,
				CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER) as overlap_days
			FROM leave_records
			WHERE status = 1 AND leave_type = 'annual' AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE overlap_days > 0 GROUP BY user_id`
	usedRows, err := database.DB.Query(usedQuery, yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart)
	if err == nil {
		for usedRows.Next() {
			var uid int64
			var used float64
			if usedRows.Scan(&uid, &used) == nil {
				if rw, ok := base[uid]; ok {
					rw.UsedDays = used
				}
			}
		}
		usedRows.Close()
	}

	list := []Row{}
	var totalConfig, totalUsed float64
	for _, uid := range userIDs {
		if rw, ok := base[uid]; ok {
			rw.RemainDays = rw.ConfigDays - rw.UsedDays
			list = append(list, *rw)
			totalConfig += rw.ConfigDays
			totalUsed += rw.UsedDays
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"year": year, "list": list,
		"total": map[string]float64{"config_days": totalConfig, "used_days": totalUsed},
	})
}

// ExportAnnualLeaveConfigs 导出年休假统计 Excel（仅管理员）
func ExportAnnualLeaveConfigs(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	if year == "" {
		year = time.Now().Format("2006")
	}
	query := `SELECT u.real_name, d.name, COALESCE(c.days, 0)
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN annual_leave_configs c ON c.user_id = u.id AND c.year = ?
		WHERE u.status = 1 ORDER BY u.id`
	rows, err := database.DB.Query(query, year)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// 已休天数（联动请假 annual）
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	usedMap := map[int64]float64{}
	usedQuery := `SELECT user_id, SUM(overlap_days) FROM (
			SELECT user_id,
				CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER) as overlap_days
			FROM leave_records
			WHERE status = 1 AND leave_type = 'annual' AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE overlap_days > 0 GROUP BY user_id`
	urows, err := database.DB.Query(usedQuery, yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart)
	if err == nil {
		for urows.Next() {
			var uid int64
			var used float64
			if urows.Scan(&uid, &used) == nil {
				usedMap[uid] = used
			}
		}
		urows.Close()
	}

	headers := []string{"序号", "姓名", "部门", "配置天数", "已休天数", "剩余天数"}
	data := [][]interface{}{}
	idx := 1
	var uid int64
	for rows.Next() {
		var name, dept sql.NullString
		var configDays float64
		rows.Scan(&name, &dept, &configDays)
		used := usedMap[uid]
		remain := configDays - used
		data = append(data, []interface{}{
			idx, name.String, dept.String, configDays, used, remain,
		})
		idx++
		uid++
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	exportExcel(w, "年休假统计", "年休假统计.xlsx", headers, data)
}

// SaveAnnualLeaveConfig 配置某人某年年休假天数（仅管理员）
func SaveAnnualLeaveConfig(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.AnnualLeaveConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.UserID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择人员"})
		return
	}
	if req.Year == "" {
		req.Year = time.Now().Format("2006")
	}
	if req.Days < 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "天数不能为负"})
		return
	}
	_, err := database.DB.Exec(
		`INSERT INTO annual_leave_configs (user_id, year, days, updated_by, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id, year) DO UPDATE SET days=?, updated_by=?, updated_at=CURRENT_TIMESTAMP`,
		req.UserID, req.Year, req.Days, operatorID,
		req.Days, operatorID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "保存成功"})
}
