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

// MarkAttendance 管理员晨会点到（单人或批量）
// body: {"attend_date":"2026-08-05","records":[{"user_id":1,"status":1,"remark":""},...]}
// 或单条: {"attend_date":"2026-08-05","user_id":1,"status":1,"remark":""}
func MarkAttendance(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	if roleCode != "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅管理员可点到"})
		return
	}

	var req struct {
		AttendDate string `json:"attend_date"`
		UserID     int64  `json:"user_id"`
		Status     int    `json:"status"`
		LeaveType  string `json:"leave_type"`
		Remark     string `json:"remark"`
		Records    []struct {
			UserID    int64  `json:"user_id"`
			Status    int    `json:"status"`
			LeaveType string `json:"leave_type"`
			Remark    string `json:"remark"`
		} `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.AttendDate == "" {
		req.AttendDate = time.Now().Format("2006-01-02")
	}

	// 批量
	if len(req.Records) > 0 {
		for _, rec := range req.Records {
			status := rec.Status
			if status == 0 {
				status = 1
			}
			_, err := database.DB.Exec(
				`INSERT INTO attendances (user_id, attend_date, status, leave_type, remark, marked_by) VALUES (?, ?, ?, ?, ?, ?)
				 ON CONFLICT(user_id, attend_date) DO UPDATE SET status=?, leave_type=?, remark=?, marked_by=?, updated_at=CURRENT_TIMESTAMP`,
				rec.UserID, req.AttendDate, status, rec.LeaveType, rec.Remark, operatorID,
				status, rec.LeaveType, rec.Remark, operatorID)
			if err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "点到失败"})
				return
			}
		}
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "点到成功"})
		return
	}

	// 单条
	if req.UserID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少人员"})
		return
	}
	status := req.Status
	if status == 0 {
		status = 1
	}
	_, err := database.DB.Exec(
		`INSERT INTO attendances (user_id, attend_date, status, leave_type, remark, marked_by) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, attend_date) DO UPDATE SET status=?, leave_type=?, remark=?, marked_by=?, updated_at=CURRENT_TIMESTAMP`,
		req.UserID, req.AttendDate, status, req.LeaveType, req.Remark, operatorID,
		status, req.LeaveType, req.Remark, operatorID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "点到失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "点到成功"})
}

// ListAttendances 考勤记录（管理员看全部，普通用户看自己，分页）
func ListAttendances(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	date := r.URL.Query().Get("date")
	month := r.URL.Query().Get("month")
	userIDFilter := r.URL.Query().Get("user_id")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if date != "" {
		where += ` AND a.attend_date = ?`
		args = append(args, date)
	}
	if month != "" {
		where += ` AND a.attend_date LIKE ?`
		args = append(args, month+"%")
	}
	if roleCode != "admin" {
		where += ` AND a.user_id = ?`
		args = append(args, userID)
	}
	if userIDFilter != "" && roleCode == "admin" {
		where += ` AND a.user_id = ?`
		args = append(args, userIDFilter)
	}

	p := parsePage(r)

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances a"+where, args...).Scan(&total)

	query := `SELECT a.id, a.user_id, u.real_name, a.attend_date, a.status, a.leave_type, a.remark, a.created_at, a.updated_at
		FROM attendances a LEFT JOIN users u ON a.user_id = u.id` + where +
		` ORDER BY a.attend_date DESC, a.user_id LIMIT ? OFFSET ?`
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.Attendance{}
	for rows.Next() {
		var a models.Attendance
		rows.Scan(&a.ID, &a.UserID, &a.UserName, &a.AttendDate, &a.Status, &a.LeaveType, &a.Remark, &a.CreatedAt, &a.UpdatedAt)
		list = append(list, a)
	}
	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}

// AttendanceStats 考勤统计（指定日期，管理员用）
func AttendanceStats(w http.ResponseWriter, r *http.Request) {
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	if roleCode != "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅管理员可查看统计"})
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	stats := map[string]int{"total": 0, "present": 0, "leave": 0, "trip": 0, "absent": 0}
	rows, err := database.DB.Query("SELECT status, COUNT(*) FROM attendances WHERE attend_date=? GROUP BY status", date)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var status, count int
		rows.Scan(&status, &count)
		total += count
		switch status {
		case 1:
			stats["present"] = count
		case 2:
			stats["leave"] = count
		case 3:
			stats["trip"] = count
		case 4:
			stats["absent"] = count
		}
	}
	stats["total"] = total
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"stats": stats, "date": date,
	})
}

// AttendanceDates 已点到日期列表（管理员选择日期用）
func AttendanceDates(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT DISTINCT attend_date FROM attendances ORDER BY attend_date DESC LIMIT 60")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	dates := []string{}
	for rows.Next() {
		var d string
		rows.Scan(&d)
		dates = append(dates, d)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"dates": dates})
}

// MarkUsers 管理员点到人员列表（全部启用人员 + 当日状态）
// MarkUsers 点到用户列表（含当日已有考勤状态与请假状态）
func MarkUsers(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 查询所有启用用户，LEFT JOIN 当天考勤记录和当天有效的请假记录
	// 若用户当天已请假（请假区间覆盖该日），自动标记为请假状态并带上请假类型
	// 用 GROUP BY u.id 去重：同一用户可能有多条覆盖该日的请假记录
	// 同时判断当天是否已保存过点到（存在考勤记录）
	var savedCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances WHERE attend_date = ?", date).Scan(&savedCount)
	saved := savedCount > 0

	query := `SELECT u.id, u.real_name, d.name,
			COALESCE(a.status, 0) as status, COALESCE(a.leave_type, '') as leave_type, COALESCE(a.remark, '') as remark,
			CASE WHEN MAX(l.id) IS NOT NULL THEN 2 ELSE 0 END as auto_leave,
			COALESCE(MAX(l.leave_type), '') as auto_leave_type
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN attendances a ON a.user_id = u.id AND a.attend_date = ?
		LEFT JOIN leave_records l ON l.user_id = u.id AND l.status = 1
			AND l.start_date <= ? AND l.end_date >= ?
		WHERE u.status = 1
		GROUP BY u.id
		ORDER BY u.id`
	rows, err := database.DB.Query(query, date, date, date)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	type UserItem struct {
		ID         int64  `json:"id"`
		RealName   string `json:"real_name"`
		Department string `json:"department"`
		Status     int    `json:"status"`
		LeaveType  string `json:"leave_type"`
		Remark     string `json:"remark"`
		AutoLeave  int    `json:"auto_leave"`
	}
	list := []UserItem{}
	for rows.Next() {
		var u UserItem
		var dept, autoLeaveType sql.NullString
		var autoLeave int
		rows.Scan(&u.ID, &u.RealName, &dept, &u.Status, &u.LeaveType, &u.Remark, &autoLeave, &autoLeaveType)
		if dept.Valid {
			u.Department = dept.String
		}
		// 若当天无考勤记录（status=0）但有请假记录，自动标记为请假，并记录 auto_leave=1
		if u.Status == 0 && autoLeave == 2 {
			u.Status = 2
			u.AutoLeave = 1
			if autoLeaveType.Valid {
				u.LeaveType = autoLeaveType.String
			}
		}
		list = append(list, u)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "saved": saved})
}

// 请假类型常量
var LeaveTypes = []string{"annual", "sick", "personal", "marriage", "maternity", "bereavement", "other"}

// CreateLeaveRecord 登记请假
// 管理员可为任意人员登记；普通用户只能为自己提交请假
func CreateLeaveRecord(w http.ResponseWriter, r *http.Request) {
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	var req models.LeaveRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	// 非管理员只能为自己请假，防止代他人登记
	if roleCode != "admin" {
		req.UserID = operatorID
	}
	if req.UserID == 0 || req.LeaveType == "" || req.StartDate == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写完整信息"})
		return
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if req.Days <= 0 {
		req.Days = 1
	}
	_, err := database.DB.Exec(
		`INSERT INTO leave_records (user_id, leave_type, start_date, end_date, days, reason, status) VALUES (?, ?, ?, ?, ?, ?, 1)`,
		req.UserID, req.LeaveType, req.StartDate, req.EndDate, req.Days, req.Reason)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "登记失败"})
		return
	}
	_ = operatorID
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "请假登记成功"})
}

// ListLeaveRecords 请假记录列表（分页）
func ListLeaveRecords(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	leaveType := r.URL.Query().Get("leave_type")
	userIDFilter := r.URL.Query().Get("user_id")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if leaveType != "" {
		where += ` AND l.leave_type = ?`
		args = append(args, leaveType)
	}
	if roleCode != "admin" {
		where += ` AND l.user_id = ?`
		args = append(args, userID)
	}
	if userIDFilter != "" && roleCode == "admin" {
		where += ` AND l.user_id = ?`
		args = append(args, userIDFilter)
	}

	p := parsePage(r)

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM leave_records l"+where, args...).Scan(&total)

	query := `SELECT l.id, l.user_id, u.real_name, d.name, l.leave_type, l.start_date, l.end_date, l.days, l.reason, l.status, l.created_at, l.updated_at
		FROM leave_records l LEFT JOIN users u ON l.user_id = u.id
		LEFT JOIN departments d ON u.department_id = d.id` + where +
		` ORDER BY l.id DESC LIMIT ? OFFSET ?`
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.LeaveRecord{}
	for rows.Next() {
		var l models.LeaveRecord
		var deptName sql.NullString
		rows.Scan(&l.ID, &l.UserID, &l.UserName, &deptName, &l.LeaveType, &l.StartDate, &l.EndDate,
			&l.Days, &l.Reason, &l.Status, &l.CreatedAt, &l.UpdatedAt)
		if deptName.Valid {
			l.Department = deptName.String
		}
		list = append(list, l)
	}
	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}

// UpdateLeaveRecord 修改请假（提交人本人或管理员）
func UpdateLeaveRecord(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.LeaveRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.LeaveType == "" || req.StartDate == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写完整信息"})
		return
	}
	// 权限校验：管理员可改任意，普通用户只能改自己的
	if roleCode != "admin" {
		var ownerID int64
		err := database.DB.QueryRow("SELECT user_id FROM leave_records WHERE id = ?", req.ID).Scan(&ownerID)
		if err != nil || ownerID != userID {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权修改该请假记录"})
			return
		}
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if req.Days <= 0 {
		req.Days = 1
	}
	_, err := database.DB.Exec(
		`UPDATE leave_records SET user_id=?, leave_type=?, start_date=?, end_date=?, days=?, reason=? WHERE id=?`,
		req.UserID, req.LeaveType, req.StartDate, req.EndDate, req.Days, req.Reason, req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
}

// DeleteLeaveRecord 删除请假记录
func DeleteLeaveRecord(w http.ResponseWriter, r *http.Request) {
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	if roleCode != "admin" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "仅管理员可删除"})
		return
	}
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	_, err := database.DB.Exec("DELETE FROM leave_records WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// LeaveStats 请假统计（各类请假 + 总计）
func LeaveStats(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	year := r.URL.Query().Get("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	// 各类型天数统计
	query := `SELECT leave_type, COUNT(*) as cnt, SUM(days) as total_days FROM leave_records
		WHERE strftime('%Y', start_date) = ?`
	args := []interface{}{year}
	if roleCode != "admin" {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	query += ` GROUP BY leave_type`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	result := map[string]interface{}{"year": year}
	totalDays := 0.0
	totalCount := 0
	for _, lt := range LeaveTypes {
		result[lt+"_count"] = 0
		result[lt+"_days"] = 0
	}
	for rows.Next() {
		var lt string
		var cnt int
		var days sql.NullFloat64
		rows.Scan(&lt, &cnt, &days)
		d := 0.0
		if days.Valid {
			d = days.Float64
		}
		result[lt+"_count"] = cnt
		result[lt+"_days"] = d
		totalDays += d
		totalCount += cnt
	}
	result["total_count"] = totalCount
	result["total_days"] = totalDays
	middleware.JSON(w, http.StatusOK, result)
}

// AttendanceMonthly 月度考勤统计（按人员）
// 返回每个人员的出勤/请假/出差/未到天数 + 各类请假天数
func AttendanceMonthly(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month") // YYYY-MM
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// 先查询出勤汇总：从所有启用用户出发，LEFT JOIN 当月考勤（无考勤记录的用户也列出，便于统计请假）
	query := `SELECT u.id, u.real_name, d.name,
			COALESCE(SUM(CASE WHEN a.status=1 THEN 1 ELSE 0 END),0) as present,
			COALESCE(SUM(CASE WHEN a.status=2 THEN 1 ELSE 0 END),0) as leave,
			COALESCE(SUM(CASE WHEN a.status=3 THEN 1 ELSE 0 END),0) as trip,
			COALESCE(SUM(CASE WHEN a.status=4 THEN 1 ELSE 0 END),0) as absent
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN attendances a ON a.user_id = u.id AND a.attend_date LIKE ?
		WHERE u.status = 1
		GROUP BY u.id ORDER BY u.id`
	rows, err := database.DB.Query(query, month+"%")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	type Row struct {
		UserID     int64   `json:"user_id"`
		UserName   string  `json:"user_name"`
		Department string  `json:"department"`
		Present    int     `json:"present"`
		Leave      int     `json:"leave"`
		Trip       int     `json:"trip"`
		Absent     int     `json:"absent"`
		AnnualDays float64 `json:"annual_days"`
		SickDays   float64 `json:"sick_days"`
		PersonalDays float64 `json:"personal_days"`
		OtherDays  float64 `json:"other_days"`
	}

	// 收集 user_id 列表，先关闭外层 rows 再查请假明细（避免 MaxOpenConns=1 死锁）
	userIDs := []int64{}
	base := map[int64]*Row{}
	for rows.Next() {
		var rw Row
		var dept sql.NullString
		rows.Scan(&rw.UserID, &rw.UserName, &dept, &rw.Present, &rw.Leave, &rw.Trip, &rw.Absent)
		if dept.Valid {
			rw.Department = dept.String
		}
		base[rw.UserID] = &rw
		userIDs = append(userIDs, rw.UserID)
	}
	rows.Close()

	// 单独查询请假明细（跨月假期按当月实际覆盖天数计算）
	// 请假区间 [start_date, end_date] 与当月 [month_start, month_end] 的重叠天数
	// = julianday(min(end_date, month_end)) - julianday(max(start_date, month_start)) + 1
	monthStart := month + "-01"
	// 计算月末：下月首日减一天
	var monthEnd string
	if len(month) == 7 {
		ym := time.Date(2006, 1, 1, 0, 0, 0, 0, time.Local)
		if t, err := time.Parse("2006-01", month); err == nil {
			ym = t
		}
		monthEnd = ym.AddDate(0, 1, -1).Format("2006-01-02")
	} else {
		monthEnd = monthStart
	}

	leaveQuery := `SELECT user_id,
			COALESCE(SUM(CASE WHEN leave_type='annual' THEN overlap_days ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN leave_type='sick' THEN overlap_days ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN leave_type='personal' THEN overlap_days ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN leave_type NOT IN ('annual','sick','personal') THEN overlap_days ELSE 0 END),0)
		FROM (
			SELECT user_id, leave_type,
				CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER) as overlap_days
			FROM leave_records
			WHERE status = 1 AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE overlap_days > 0 GROUP BY user_id`
	lrows, err := database.DB.Query(leaveQuery, monthEnd, monthEnd, monthStart, monthStart, monthEnd, monthStart)
	if err == nil {
		for lrows.Next() {
			var uid int64
			var annual, sick, personal, other float64
			if lrows.Scan(&uid, &annual, &sick, &personal, &other) == nil {
				if rw, ok := base[uid]; ok {
					rw.AnnualDays = annual
					rw.SickDays = sick
					rw.PersonalDays = personal
					rw.OtherDays = other
				}
			}
		}
		lrows.Close()
	}

	list := []Row{}
	var totalPresent, totalLeave, totalTrip, totalAbsent int
	for _, uid := range userIDs {
		if rw, ok := base[uid]; ok {
			list = append(list, *rw)
			totalPresent += rw.Present
			totalLeave += rw.Leave
			totalTrip += rw.Trip
			totalAbsent += rw.Absent
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"month": month, "list": list,
		"total": map[string]int{
			"present": totalPresent, "leave": totalLeave, "trip": totalTrip, "absent": totalAbsent,
		},
	})
}

// AttendanceYearly 年度考勤统计
// 返回：按月出勤汇总 + 每个干部全年各类休假天数（跨年假期按当年实际天数计算）
func AttendanceYearly(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year") // YYYY
	if year == "" {
		year = time.Now().Format("2006")
	}

	// 按月出勤汇总
	query := `SELECT substr(a.attend_date, 1, 7) as ym,
			SUM(CASE WHEN a.status=1 THEN 1 ELSE 0 END) as present,
			SUM(CASE WHEN a.status=2 THEN 1 ELSE 0 END) as leave,
			SUM(CASE WHEN a.status=3 THEN 1 ELSE 0 END) as trip,
			SUM(CASE WHEN a.status=4 THEN 1 ELSE 0 END) as absent
		FROM attendances a
		WHERE a.attend_date LIKE ?
		GROUP BY ym ORDER BY ym`
	rows, err := database.DB.Query(query, year+"%")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	type MonthRow struct {
		Month   string `json:"month"`
		Present int    `json:"present"`
		Leave   int    `json:"leave"`
		Trip    int    `json:"trip"`
		Absent  int    `json:"absent"`
	}
	monthly := []MonthRow{}
	var totalPresent, totalLeave, totalTrip, totalAbsent int
	for rows.Next() {
		var rw MonthRow
		rows.Scan(&rw.Month, &rw.Present, &rw.Leave, &rw.Trip, &rw.Absent)
		monthly = append(monthly, rw)
		totalPresent += rw.Present
		totalLeave += rw.Leave
		totalTrip += rw.Trip
		totalAbsent += rw.Absent
	}

	// 每个干部全年各类休假天数
	// 跨年假期（如 2025-12-20~2026-01-10）按当年实际覆盖天数计算
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	leaveQuery := `SELECT l.id, l.real_name, d.name,
			COALESCE(SUM(CASE WHEN od.leave_type='annual' THEN od.overlap ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN od.leave_type='sick' THEN od.overlap ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN od.leave_type='personal' THEN od.overlap ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN od.leave_type NOT IN ('annual','sick','personal') THEN od.overlap ELSE 0 END),0),
			COALESCE(SUM(od.overlap),0)
		FROM (
			SELECT user_id, leave_type,
				CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER) as overlap
			FROM leave_records
			WHERE status = 1 AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) od
		LEFT JOIN users l ON l.id = od.user_id
		LEFT JOIN departments d ON l.department_id = d.id
		WHERE od.overlap > 0
		GROUP BY od.user_id ORDER BY od.user_id`
	lrows, err := database.DB.Query(leaveQuery, yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer lrows.Close()

	type PersonRow struct {
		UserID      int64   `json:"user_id"`
		UserName    string  `json:"user_name"`
		Department  string  `json:"department"`
		AnnualDays  float64 `json:"annual_days"`
		SickDays    float64 `json:"sick_days"`
		PersonalDays float64 `json:"personal_days"`
		OtherDays   float64 `json:"other_days"`
		TotalDays   float64 `json:"total_days"`
	}
	persons := []PersonRow{}
	var totalAnnual, totalSick, totalPersonal, totalOther, totalAll float64
	for lrows.Next() {
		var p PersonRow
		var dept sql.NullString
		if lrows.Scan(&p.UserID, &p.UserName, &dept, &p.AnnualDays, &p.SickDays, &p.PersonalDays, &p.OtherDays, &p.TotalDays) == nil {
			if dept.Valid {
				p.Department = dept.String
			}
			persons = append(persons, p)
			totalAnnual += p.AnnualDays
			totalSick += p.SickDays
			totalPersonal += p.PersonalDays
			totalOther += p.OtherDays
			totalAll += p.TotalDays
		}
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"year": year, "monthly": monthly,
		"total": map[string]int{
			"present": totalPresent, "leave": totalLeave, "trip": totalTrip, "absent": totalAbsent,
		},
		"persons": persons,
		"leave_total": map[string]float64{
			"annual": totalAnnual, "sick": totalSick, "personal": totalPersonal, "other": totalOther,
			"total_days": totalAll,
		},
	})
}

// GetAssignees 可分配/可选人员列表（启用用户）
func GetAssignees(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(
		`SELECT u.id, u.real_name, d.name FROM users u LEFT JOIN departments d ON u.department_id = d.id WHERE u.status = 1 ORDER BY u.id`)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	type Assignee struct {
		ID         int64  `json:"id"`
		RealName   string `json:"real_name"`
		Department string `json:"department"`
	}
	list := []Assignee{}
	for rows.Next() {
		var a Assignee
		var dept sql.NullString
		rows.Scan(&a.ID, &a.RealName, &dept)
		if dept.Valid {
			a.Department = dept.String
		}
		list = append(list, a)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}
