package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	defer rows.Close()

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
		var userName, dept sql.NullString
		if err := rows.Scan(&rw.UserID, &userName, &dept, &rw.ConfigDays); err != nil {
			continue
		}
		rw.UserName = userName.String
		rw.Department = dept.String
		base[rw.UserID] = &rw
		userIDs = append(userIDs, rw.UserID)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 查询已休年假天数（leave_type='annual'，按当年实际覆盖天数，支持半天/小时假）
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	usedQuery := `SELECT user_id, SUM(eff) FROM (
			SELECT user_id,
				-- 废除 MIN 粗暴逻辑：按请假区间在目标年份的实际日历天数比例分配扣减天数，确保总扣减天数不超过申请天数
				ROUND(days * (
					julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1.0
				) / (julianday(end_date) - julianday(start_date) + 1.0), 2) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'annual' AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE eff > 0 GROUP BY user_id`
	usedRows, err := database.DB.Query(usedQuery, yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer usedRows.Close()
	for usedRows.Next() {
		var uid int64
		var used float64
		if err := usedRows.Scan(&uid, &used); err != nil {
			continue
		}
		if rw, ok := base[uid]; ok {
			rw.UsedDays = used
		}
	}
	if err := usedRows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
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
	query := `SELECT u.id, u.real_name, d.name, COALESCE(c.days, 0)
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN annual_leave_configs c ON c.user_id = u.id AND c.year = ?
		WHERE u.status = 1 ORDER BY u.id LIMIT 10000`
	rows, err := database.DB.Query(query, year)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	// 先读完用户（含 id）并关闭，避免 MaxOpenConns=1 下未关闭 rows 再查询导致死锁
	type userRow struct {
		id         int64
		name, dept string
		configDays float64
	}
	users := []userRow{}
	for rows.Next() {
		var u userRow
		var name, dept sql.NullString
		if err := rows.Scan(&u.id, &name, &dept, &u.configDays); err != nil {
			continue
		}
		u.name = name.String
		u.dept = dept.String
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 已休天数（联动请假 annual，支持半天/小时假）
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	usedMap := map[int64]float64{}
	usedQuery := `SELECT user_id, SUM(eff) FROM (
			SELECT user_id,
				-- 按请假区间在目标年份的实际日历天数比例分配扣减天数，确保总扣减天数不超过申请天数
				ROUND(days * (
					julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1.0
				) / (julianday(end_date) - julianday(start_date) + 1.0), 2) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'annual' AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE eff > 0 GROUP BY user_id LIMIT 10000`
	urows, err := database.DB.Query(usedQuery, yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer urows.Close()
	for urows.Next() {
		var uid int64
		var used float64
		if err := urows.Scan(&uid, &used); err != nil {
			continue
		}
		usedMap[uid] = used
	}
	if err := urows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	headers := []string{"序号", "姓名", "部门", "配置天数", "已休天数", "剩余天数"}
	data := [][]interface{}{}
	idx := 1
	for _, u := range users {
		used := usedMap[u.id]
		remain := u.configDays - used
		data = append(data, []interface{}{
			idx, u.name, u.dept, u.configDays, used, remain,
		})
		idx++
	}
	logOperation(r, "年休假管理", "导出", "导出年休假统计")
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
	if !isValidYear(req.Year) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "年份格式应为 YYYY"})
		return
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
	var personName string
	database.DB.QueryRow("SELECT real_name FROM users WHERE id=?", req.UserID).Scan(&personName)
	logOperation(r, "年休假管理", "修改", fmt.Sprintf("设置「%s」%s年年休假 %.1f 天", personName, req.Year, req.Days))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "保存成功"})
}

// getAnnualLeaveRemainDays 计算指定人员在某年份的剩余年休假天数
// 返回：(剩余天数, 该年配置的总天数)
func getAnnualLeaveRemainDays(userID int64, year string, excludeID int64) (float64, float64) {
	return getAnnualLeaveRemainDaysFromQuerier(database.DB, userID, year, excludeID)
}

// getAnnualLeaveRemainDaysTx 事务内计算剩余年休假天数
func getAnnualLeaveRemainDaysTx(tx *sql.Tx, userID int64, year string, excludeID int64) (float64, float64) {
	return getAnnualLeaveRemainDaysFromQuerier(tx, userID, year, excludeID)
}

type queryRower interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func getAnnualLeaveRemainDaysFromQuerier(q queryRower, userID int64, year string, excludeID int64) (float64, float64) {
	var configDays float64
	if err := q.QueryRow("SELECT COALESCE(days, 0) FROM annual_leave_configs WHERE user_id=? AND year=?", userID, year).Scan(&configDays); err != nil {
		configDays = 0
	}
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	query := `SELECT COALESCE(SUM(eff), 0) FROM (
		SELECT ROUND(days * (
			julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
			- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1.0
		) / (julianday(end_date) - julianday(start_date) + 1.0), 2) as eff
		FROM leave_records
		WHERE status = 1 AND leave_type = 'annual' AND user_id = ? AND id != ? AND start_date <= ? AND end_date >= ?
	) WHERE eff > 0`
	var usedDays float64
	if err := q.QueryRow(query, yearEnd, yearEnd, yearStart, yearStart, userID, excludeID, yearEnd, yearStart).Scan(&usedDays); err != nil {
		usedDays = 0
	}
	remain := configDays - usedDays
	if remain < 0 {
		remain = 0
	}
	return remain, configDays
}
