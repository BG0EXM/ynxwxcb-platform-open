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

// OvertimeHoursPerDay 补休换算：8 小时加班 = 1 天补休
const OvertimeHoursPerDay = 8.0

// ListOvertimeRecords 加班记录列表（管理员看全部，普通用户仅本人）
func ListOvertimeRecords(w http.ResponseWriter, r *http.Request) {
	currentUser, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	userID := r.URL.Query().Get("user_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	year := r.URL.Query().Get("year")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if roleCode != "admin" {
		// 普通用户强制只看本人，忽略传入的 user_id
		where += ` AND o.user_id = ?`
		args = append(args, currentUser)
	} else if userID != "" {
		where += ` AND o.user_id = ?`
		args = append(args, userID)
	}
	if start != "" {
		where += ` AND o.overtime_date >= ?`
		args = append(args, start)
	}
	if end != "" {
		where += ` AND o.overtime_date <= ?`
		args = append(args, end)
	}
	if year != "" {
		where += ` AND o.overtime_date LIKE ?`
		args = append(args, year+"%")
	}

	query := `SELECT o.id, o.user_id, u.real_name, o.overtime_date, o.hours, o.reason,
			o.created_by, o.created_at
		FROM overtime_records o
		LEFT JOIN users u ON o.user_id = u.id` + where + ` ORDER BY o.overtime_date DESC, o.id DESC`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.OvertimeRecord{}
	for rows.Next() {
		var o models.OvertimeRecord
		var name, reason sql.NullString
		var overtimeDate sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&o.ID, &o.UserID, &name, &overtimeDate, &o.Hours, &reason, &o.CreatedBy, &createdAt); err != nil {
			continue
		}
		o.UserName = name.String
		o.OvertimeDate = overtimeDate.String
		o.Reason = reason.String
		if createdAt.Valid {
			o.CreatedAt = createdAt.Time
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateOvertimeRecord 录入加班记录（仅管理员）
func CreateOvertimeRecord(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.OvertimeRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.UserID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择人员"})
		return
	}
	if !isValidDate(req.OvertimeDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "加班日期格式应为 YYYY-MM-DD"})
		return
	}
	if req.Hours <= 0 || req.Hours > 24 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "加班时长应在 0~24 小时之间"})
		return
	}
	_, err := database.DB.Exec(
		`INSERT INTO overtime_records (user_id, overtime_date, hours, reason, created_by) VALUES (?, ?, ?, ?, ?)`,
		req.UserID, req.OvertimeDate, req.Hours, req.Reason, operatorID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	var personName string
	database.DB.QueryRow("SELECT real_name FROM users WHERE id=?", req.UserID).Scan(&personName)
	logOperation(r, "加班管理", "新增", fmt.Sprintf("为「%s」录入加班 %.1f 小时（%s）", personName, req.Hours, req.OvertimeDate))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// DeleteOvertimeRecord 删除加班记录（仅管理员）
func DeleteOvertimeRecord(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var personName, overtimeDate string
	database.DB.QueryRow(
		`SELECT u.real_name, o.overtime_date FROM overtime_records o LEFT JOIN users u ON o.user_id=u.id WHERE o.id=?`, id).
		Scan(&personName, &overtimeDate)
	_, err := database.DB.Exec("DELETE FROM overtime_records WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	logOperation(r, "加班管理", "删除", "删除「"+personName+"」的加班记录（"+overtimeDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportOvertimeRecords 导出加班记录 Excel（管理员）
func ExportOvertimeRecords(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")
	query := `SELECT o.id, u.real_name, o.overtime_date, o.hours, o.reason, o.created_at
		FROM overtime_records o
		LEFT JOIN users u ON o.user_id = u.id WHERE 1=1`
	args := []interface{}{}
	if month != "" {
		query += ` AND o.overtime_date LIKE ?`
		args = append(args, month+"%")
	} else if year != "" {
		query += ` AND o.overtime_date LIKE ?`
		args = append(args, year+"%")
	}
	query += ` ORDER BY o.overtime_date DESC, o.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	headers := []string{"序号", "姓名", "加班日期", "加班时长(小时)", "事由", "登记时间"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var name, overtimeDate, reason sql.NullString
		var hours float64
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &name, &overtimeDate, &hours, &reason, &createdAt); err != nil {
			continue
		}
		data = append(data, []interface{}{
			idx, name.String, formatDateStr(overtimeDate.String), hours, reason.String, formatDateTime(createdAt.Time),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	logOperation(r, "加班管理", "导出", "导出加班记录")
	exportExcel(w, "加班记录", "加班记录.xlsx", headers, data)
}

// getCompRemainDays 计算指定人员的全部可补休天数（不限定月份）
// = 加班折合补休天数 - 已用补休天数
func getCompRemainDays(userID int64) float64 {
	var hours float64
	if err := database.DB.QueryRow("SELECT COALESCE(SUM(hours),0) FROM overtime_records WHERE user_id=?", userID).Scan(&hours); err != nil {
		return 0
	}
	compDays := hours / OvertimeHoursPerDay

	var used float64
	if err := database.DB.QueryRow(
		`SELECT COALESCE(SUM(eff),0) FROM (
			SELECT MIN(days, CAST(julianday(end_date) - julianday(start_date) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'comp' AND user_id = ?
		) WHERE eff > 0`, userID).Scan(&used); err != nil {
		return 0
	}
	return compDays - used
}

// OvertimeStats 加班统计（按人，可按年或按月）
// 返回每人：加班小时数、折合补休天数、已补休天数、剩余可补天数
// 补休从 leave_records 的 leave_type='comp'（按日期范围实际覆盖天数计算）
func OvertimeStats(w http.ResponseWriter, r *http.Request) {
	currentUser, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	year := r.URL.Query().Get("year")   // YYYY
	month := r.URL.Query().Get("month") // YYYY-MM（可选，不填则统计全年）
	dateLike := ""
	datePrefix := ""
	if month != "" {
		dateLike = month + "%"
		datePrefix = month
	} else {
		if year == "" {
			year = time.Now().Format("2006")
		}
		dateLike = year + "%"
		datePrefix = year
	}

	// 1. 查询加班汇总（按年/月）
	// 注意：user_id 必须从 u.id 取，不能从 o.user_id（无加班记录时 o.user_id 为 NULL）
	otQuery := `SELECT u.id, u.real_name, d.name,
			COALESCE(SUM(o.hours), 0)
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN overtime_records o ON o.user_id = u.id AND o.overtime_date LIKE ?
		WHERE u.status = 1`
	otArgs := []interface{}{dateLike}
	if roleCode != "admin" {
		// 普通用户仅统计本人
		otQuery += ` AND u.id = ?`
		otArgs = append(otArgs, currentUser)
	}
	otQuery += ` GROUP BY u.id ORDER BY u.id`
	rows, err := database.DB.Query(otQuery, otArgs...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	type Row struct {
		UserID        int64   `json:"user_id"`
		UserName      string  `json:"user_name"`
		Department    string  `json:"department"`
		OvertimeHours float64 `json:"overtime_hours"`
		CompDays      float64 `json:"comp_days"`   // 折合补休天数
		UsedDays      float64 `json:"used_days"`   // 已补休天数
		RemainDays    float64 `json:"remain_days"` // 剩余可补
	}
	base := map[int64]*Row{}
	var userIDs []int64
	for rows.Next() {
		var rw Row
		var userName, dept sql.NullString
		if err := rows.Scan(&rw.UserID, &userName, &dept, &rw.OvertimeHours); err != nil {
			continue
		}
		rw.UserName = userName.String
		rw.Department = dept.String
		rw.CompDays = rw.OvertimeHours / OvertimeHoursPerDay
		base[rw.UserID] = &rw
		userIDs = append(userIDs, rw.UserID)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	rows.Close()

	// 2. 查询已用补休天数（leave_type='comp'，按日期范围实际覆盖天数计算）
	rangeStart := datePrefix + "-01"
	var rangeEnd string
	if month != "" {
		if t, err := time.Parse("2006-01", month); err == nil {
			rangeEnd = t.AddDate(0, 1, -1).Format("2006-01-02")
		} else {
			rangeEnd = rangeStart
		}
	} else {
		rangeEnd = year + "-12-31"
	}
	compQuery := `SELECT user_id, SUM(eff) FROM (
			SELECT user_id,
				-- 已补休 = MIN(登记天数, 期间重叠整天)，半天补休(0.5)精确扣减
				MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'comp' AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE eff > 0 GROUP BY user_id`
	crows, err := database.DB.Query(compQuery, rangeEnd, rangeEnd, rangeStart, rangeStart, rangeEnd, rangeStart)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	for crows.Next() {
		var uid int64
		var used float64
		if err := crows.Scan(&uid, &used); err != nil {
			continue
		}
		if rw, ok := base[uid]; ok {
			rw.UsedDays = used
		}
	}
	if err := crows.Err(); err != nil {
		crows.Close()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	crows.Close()

	// 2b. 全时口径的剩余可补天数（与补休登记校验 getCompRemainDays 一致，避免按钮状态与后端校验不一致）
	allHours := map[int64]float64{}
	if hr, err := database.DB.Query("SELECT user_id, COALESCE(SUM(hours),0) FROM overtime_records GROUP BY user_id"); err == nil {
		for hr.Next() {
			var uid int64
			var h float64
			if hr.Scan(&uid, &h) == nil {
				allHours[uid] = h
			}
		}
		hr.Close()
	}
	allUsed := map[int64]float64{}
	if ur, err := database.DB.Query(
		`SELECT user_id, COALESCE(SUM(eff),0) FROM (
			SELECT user_id, MIN(days, CAST(julianday(end_date) - julianday(start_date) + 1 AS INTEGER)) as eff
			FROM leave_records WHERE status = 1 AND leave_type = 'comp' GROUP BY id
		) WHERE eff > 0 GROUP BY user_id`); err == nil {
		for ur.Next() {
			var uid int64
			var d float64
			if ur.Scan(&uid, &d) == nil {
				allUsed[uid] = d
			}
		}
		ur.Close()
	}

	// 3. 组装
	list := []Row{}
	var totalHours, totalUsed float64
	for _, uid := range userIDs {
		if rw, ok := base[uid]; ok {
			// 剩余可补统一按全时口径（全部加班折合 - 全部已补休）
			rw.RemainDays = allHours[uid]/OvertimeHoursPerDay - allUsed[uid]
			list = append(list, *rw)
			totalHours += rw.OvertimeHours
			totalUsed += rw.UsedDays
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"period": datePrefix, "list": list,
		"total": map[string]float64{
			"overtime_hours": totalHours, "used_days": totalUsed,
		},
	})
}
