package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
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

	validStatus := func(s int) bool { return s >= 1 && s <= 6 }

	// 组装待写入记录
	type markRec struct {
		UserID    int64
		Status    int
		LeaveType string
		Remark    string
	}
	recs := []markRec{}
	if len(req.Records) > 0 {
		for _, rec := range req.Records {
			status := rec.Status
			if status == 0 {
				status = 1
			}
			if rec.UserID == 0 || !validStatus(status) {
				middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "点到参数不合法"})
				return
			}
			recs = append(recs, markRec{rec.UserID, status, rec.LeaveType, rec.Remark})
		}
	} else {
		if req.UserID == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少人员"})
			return
		}
		status := req.Status
		if status == 0 {
			status = 1
		}
		if !validStatus(status) {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "点到参数不合法"})
			return
		}
		recs = append(recs, markRec{req.UserID, status, req.LeaveType, req.Remark})
	}

	// 批量写入，整体事务
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "点到失败"})
		return
	}
	defer tx.Rollback()
	for _, rec := range recs {
		if _, err := tx.Exec(
			`INSERT INTO attendances (user_id, attend_date, status, leave_type, remark, marked_by) VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(user_id, attend_date) DO UPDATE SET status=?, leave_type=?, remark=?, marked_by=?, updated_at=CURRENT_TIMESTAMP`,
			rec.UserID, req.AttendDate, rec.Status, rec.LeaveType, rec.Remark, operatorID,
			rec.Status, rec.LeaveType, rec.Remark, operatorID); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "点到失败"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "点到失败"})
		return
	}
	logOperation(r, "考勤管理", "新增", "考勤点到（"+req.AttendDate+"，共 "+strconv.Itoa(len(recs))+" 人）")
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
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM attendances a"+where, args...).Scan(&total); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

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
		var userName, leaveType, remark sql.NullString
		if err := rows.Scan(&a.ID, &a.UserID, &userName, &a.AttendDate, &a.Status, &leaveType, &remark, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		a.UserName = userName.String
		a.LeaveType = leaveType.String
		a.Remark = remark.String
		list = append(list, a)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}

// AttendanceStats 考勤统计（指定日期）
func AttendanceStats(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	stats := map[string]int{"total": 0, "present": 0, "leave": 0, "trip": 0, "absent": 0, "late": 0, "training": 0}
	rows, err := database.DB.Query("SELECT status, COUNT(*) FROM attendances WHERE attend_date=? GROUP BY status", date)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var status, count int
		if err := rows.Scan(&status, &count); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
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
		case 5:
			stats["late"] = count
		case 6:
			stats["training"] = count
		}
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
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
		if err := rows.Scan(&d); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
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
			CASE WHEN MAX(CASE WHEN l.leave_type != 'comp' THEN l.id END) IS NOT NULL THEN 2 ELSE 0 END as auto_leave,
			COALESCE(MAX(CASE WHEN l.leave_type != 'comp' THEN l.leave_type END), '') as auto_leave_type,
			CASE WHEN MAX(CASE WHEN l.leave_type = 'comp' THEN l.id END) IS NOT NULL THEN 1 ELSE 0 END as auto_comp
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
		ID              int64  `json:"id"`
		RealName        string `json:"real_name"`
		Department      string `json:"department"`
		Status          int    `json:"status"`
		LeaveType       string `json:"leave_type"`
		Remark          string `json:"remark"`
		AutoLeave       int    `json:"auto_leave"`
		AutoComp        int    `json:"auto_comp"`
		HasLeave        bool   `json:"has_leave"`
		LeaveRecordType string `json:"leave_record_type"`
	}
	list := []UserItem{}
	for rows.Next() {
		var u UserItem
		var dept, autoLeaveType sql.NullString
		var autoLeave, autoComp int
		if err := rows.Scan(&u.ID, &u.RealName, &dept, &u.Status, &u.LeaveType, &u.Remark, &autoLeave, &autoLeaveType, &autoComp); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
		if dept.Valid {
			u.Department = dept.String
		}
		if autoLeave == 2 {
			u.HasLeave = true
			if autoLeaveType.Valid {
				u.LeaveRecordType = autoLeaveType.String
			}
			u.AutoLeave = 1
		}
		if autoComp == 1 {
			u.AutoComp = 1
		}
		// 若当天无考勤记录（status=0）但有请假记录，自动标记为请假，并记录 auto_leave=1
		if u.Status == 0 && autoLeave == 2 {
			u.Status = 2
			if autoLeaveType.Valid {
				u.LeaveType = autoLeaveType.String
			}
		}
		// 补休（comp）：无考勤记录时按出勤处理，标记 auto_comp=1（补休视为正常出勤）
		if u.Status == 0 && autoComp == 1 {
			u.Status = 1
		}
		list = append(list, u)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "saved": saved})
}

// 请假类型常量
var LeaveTypes = []string{"annual", "sick", "personal", "marriage", "maternity", "bereavement", "prenatal", "family", "training", "comp", "other"}

// 请假类型中文名（日志/友好展示用）
var leaveTypeLabels = map[string]string{
	"annual": "年假", "sick": "病假", "personal": "事假", "marriage": "婚假",
	"maternity": "产假", "bereavement": "丧假", "prenatal": "产检假", "family": "探亲假",
	"training": "培训", "comp": "补休", "other": "其他",
}

func leaveTypeLabel(t string) string {
	if v, ok := leaveTypeLabels[t]; ok {
		return v
	}
	return t
}

// isValidLeaveType 校验请假类型是否在白名单内
func isValidLeaveType(t string) bool {
	for _, lt := range LeaveTypes {
		if lt == t {
			return true
		}
	}
	return false
}

// isValidDate 校验日期格式是否为 YYYY-MM-DD
func isValidDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// isValidYear 校验年份格式是否为 YYYY
func isValidYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// syncAttendanceForDateRangeTx 级联更新覆盖时间段内 attendances 表的记录
// 当请假变动（创建、修改、删除）时，逐日回溯检查用户在当天的有效请假情况：
// - 若当天存在有效请假：更新或补全考勤记录为请假状态（status=2, leave_type=该请假类型）
// - 若当天已无任何有效请假：若考勤记录原为请假状态（status=2），恢复为正常状态（status=1, leave_type=''）
func syncAttendanceForDateRangeTx(tx *sql.Tx, userID int64, startDate, endDate string) error {
	t1, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return err
	}
	t2, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return err
	}
	if t2.Before(t1) {
		return nil
	}

	for d := t1; !d.After(t2); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		// 1. 查询当天是否存在生效中的请假记录
		var activeLeaveType string
		err := tx.QueryRow(
			`SELECT leave_type FROM leave_records 
			 WHERE user_id = ? AND status = 1 AND start_date <= ? AND end_date >= ?
			 ORDER BY id DESC LIMIT 1`,
			userID, dateStr, dateStr).Scan(&activeLeaveType)
		hasActiveLeave := (err == nil && activeLeaveType != "")

		// 2. 检查 attendances 表中当天是否有记录
		var attID int64
		var currentStatus int
		attErr := tx.QueryRow(
			`SELECT id, status FROM attendances WHERE user_id = ? AND attend_date = ?`,
			userID, dateStr).Scan(&attID, &currentStatus)

		if attErr == nil {
			if hasActiveLeave {
				targetStatus := 2
				targetRemark := "请假登记联动"
				if activeLeaveType == "comp" {
					targetStatus = 1
					targetRemark = "补休登记联动"
				}
				if currentStatus != targetStatus {
					_, err = tx.Exec(
						`UPDATE attendances SET status = ?, leave_type = ?, remark = CASE WHEN remark = '' OR remark IS NULL THEN ? ELSE remark END, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
						targetStatus, activeLeaveType, targetRemark, attID)
					if err != nil {
						return err
					}
				}
			} else {
				// 若当天已无任何有效请假，且原考勤记录为请假状态（status=2），则恢复为正常出勤状态（status=1）
				if currentStatus == 2 {
					_, err = tx.Exec(
						`UPDATE attendances SET status = 1, leave_type = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
						attID)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

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
	if !isValidLeaveType(req.LeaveType) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请假类型不合法"})
		return
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if !isValidDate(req.StartDate) || !isValidDate(req.EndDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	// 起止日期合法性校验（防止 start>end 脏数据）
	if req.EndDate < req.StartDate {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "结束日期不能早于开始日期"})
		return
	}
	if req.Days <= 0 {
		req.Days = 1
	}

	// 开启事务：事务内重新聚合核算额度，若不足立即回滚报错，杜绝并发穿透与额度超支
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统繁忙，请稍后再试"})
		return
	}
	defer tx.Rollback()

	// 补休校验：在事务内聚合重新计算剩余额度
	if req.LeaveType == "comp" {
		remain := getCompRemainDaysTx(tx, req.UserID)
		if remain < req.Days {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("可补休天数不足（剩余 %s 天，申请 %.1f 天）", strconv.FormatFloat(remain, 'f', -1, 64), req.Days),
			})
			return
		}
	}

	// 年休假校验：当年年休假剩余额度不足时不允许登记
	if req.LeaveType == "annual" {
		if len(req.StartDate) < 4 || len(req.EndDate) < 4 {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式错误，无法提取年份"})
			return
		}
		startYear := req.StartDate[:4]
		endYear := req.EndDate[:4]
		if startYear == endYear {
			remain, configDays := getAnnualLeaveRemainDaysTx(tx, req.UserID, startYear, 0)
			if configDays <= 0 {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": startYear + "年度尚未配置年休假额度，请联系管理员配置"})
				return
			}
			if remain < req.Days {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("%s年度年休假剩余天数不足（剩余 %s 天，申请 %.1f 天）", startYear, strconv.FormatFloat(remain, 'f', -1, 64), req.Days),
				})
				return
			}
		} else {
			// 跨年申请：按实际日历天数比例分配扣减天数核算
			t1, _ := time.Parse("2006-01-02", req.StartDate)
			t2, _ := time.Parse("2006-01-02", req.EndDate)
			totalSpan := t2.Sub(t1).Hours()/24 + 1
			if totalSpan < 1 {
				totalSpan = 1
			}
			endOfStartYear, _ := time.Parse("2006-01-02", startYear+"-12-31")
			span1 := endOfStartYear.Sub(t1).Hours()/24 + 1
			need1 := math.Round((req.Days*span1/totalSpan)*100) / 100
			need2 := math.Round((req.Days-need1)*100) / 100

			remain1, config1 := getAnnualLeaveRemainDaysTx(tx, req.UserID, startYear, 0)
			if config1 <= 0 {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": startYear + "年度尚未配置年休假额度，请联系管理员配置"})
				return
			}
			if remain1 < need1 {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("%s年度年休假剩余天数不足（剩余 %s 天，跨年申请需扣减 %s 天）", startYear, strconv.FormatFloat(remain1, 'f', -1, 64), strconv.FormatFloat(need1, 'f', -1, 64)),
				})
				return
			}

			remain2, config2 := getAnnualLeaveRemainDaysTx(tx, req.UserID, endYear, 0)
			if config2 <= 0 {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": endYear + "年度尚未配置年休假额度，请联系管理员配置"})
				return
			}
			if remain2 < need2 {
				tx.Rollback()
				middleware.JSON(w, http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("%s年度年休假剩余天数不足（剩余 %s 天，跨年申请需扣减 %s 天）", endYear, strconv.FormatFloat(remain2, 'f', -1, 64), strconv.FormatFloat(need2, 'f', -1, 64)),
				})
				return
			}
		}
	}

	_, err = tx.Exec(
		`INSERT INTO leave_records (user_id, leave_type, start_date, end_date, days, leave_hours, reason, status) VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		req.UserID, req.LeaveType, req.StartDate, req.EndDate, req.Days, req.LeaveHours, req.Reason)
	if err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "登记失败"})
		return
	}

	// 同步级联更新覆盖时间段内 attendances 表的记录（标记为请假）
	if err := syncAttendanceForDateRangeTx(tx, req.UserID, req.StartDate, req.EndDate); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "同步考勤状态失败"})
		return
	}

	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交事务失败"})
		return
	}

	var personName string
	database.DB.QueryRow("SELECT real_name FROM users WHERE id=?", req.UserID).Scan(&personName)
	logOperation(r, "请假管理", "新增", "登记请假："+personName+" "+leaveTypeLabel(req.LeaveType)+"（"+req.StartDate+" 至 "+req.EndDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "请假登记成功"})
}

// ListLeaveRecords 请假记录列表（分页）
func ListLeaveRecords(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	leaveType := r.URL.Query().Get("leave_type")
	userIDFilter := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month") // YYYY-MM 按开始日期所在月筛选

	where := ` WHERE 1=1`
	args := []interface{}{}
	if leaveType != "" {
		where += ` AND l.leave_type = ?`
		args = append(args, leaveType)
	}
	if month != "" {
		where += ` AND l.start_date LIKE ?`
		args = append(args, month+"%")
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
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM leave_records l"+where, args...).Scan(&total); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	query := `SELECT l.id, l.user_id, u.real_name, d.name, l.leave_type, l.start_date, l.end_date, l.days, l.leave_hours, l.reason, l.status, l.created_at, l.updated_at
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
		var userName, deptName, reason sql.NullString
		if err := rows.Scan(&l.ID, &l.UserID, &userName, &deptName, &l.LeaveType, &l.StartDate, &l.EndDate,
			&l.Days, &l.LeaveHours, &reason, &l.Status, &l.CreatedAt, &l.UpdatedAt); err != nil {
			continue
		}
		l.UserName = userName.String
		l.Department = deptName.String
		l.Reason = reason.String
		list = append(list, l)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, paginateResult(list, total, p.Page, p.PageSize))
}

// GetLeaveRecord 单条请假记录（打印页按 ID 获取，本人或管理员）
func GetLeaveRecord(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var l models.LeaveRecord
	var userName, deptName, reason sql.NullString
	err := database.DB.QueryRow(
		`SELECT l.id, l.user_id, u.real_name, d.name, l.leave_type, l.start_date, l.end_date, l.days, l.leave_hours, l.reason, l.status, l.created_at, l.updated_at
		 FROM leave_records l
		 LEFT JOIN users u ON l.user_id = u.id
		 LEFT JOIN departments d ON u.department_id = d.id
		 WHERE l.id = ?`, id).
		Scan(&l.ID, &l.UserID, &userName, &deptName, &l.LeaveType, &l.StartDate, &l.EndDate,
			&l.Days, &l.LeaveHours, &reason, &l.Status, &l.CreatedAt, &l.UpdatedAt)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "记录不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	if roleCode != "admin" && l.UserID != userID {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权查看该记录"})
		return
	}
	l.UserName = userName.String
	l.Department = deptName.String
	l.Reason = reason.String
	middleware.JSON(w, http.StatusOK, l)
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
	if !isValidLeaveType(req.LeaveType) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请假类型不合法"})
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
		// 普通用户不能篡改请假归属人（防止改到他人名下）
		req.UserID = userID
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if !isValidDate(req.StartDate) || !isValidDate(req.EndDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	// 起止日期合法性校验（防止 start>end 脏数据）
	if req.EndDate < req.StartDate {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "结束日期不能早于开始日期"})
		return
	}
	if req.Days <= 0 {
		req.Days = 1
	}
	// 查出修改前的旧记录信息，用于级联回溯更新原日期的考勤点到
	var oldUserID int64
	var oldStartDate, oldEndDate, oldLeaveType string
	err := database.DB.QueryRow("SELECT user_id, start_date, end_date, leave_type FROM leave_records WHERE id = ?", req.ID).
		Scan(&oldUserID, &oldStartDate, &oldEndDate, &oldLeaveType)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "请假记录不存在"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统繁忙，请稍后再试"})
		return
	}

	// 补休校验：改为补休类型时校验余额（排除本记录自身已占用的天数）
	if req.LeaveType == "comp" {
		remain := getCompRemainDaysTx(tx, req.UserID)
		var selfDays float64
		tx.QueryRow(
			`SELECT COALESCE(MIN(days, CAST(julianday(end_date)-julianday(start_date)+1 AS INTEGER)),0)
			 FROM leave_records WHERE id=? AND leave_type='comp' AND user_id=?`, req.ID, req.UserID).Scan(&selfDays)
		if remain+selfDays < req.Days {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "可补休天数不足"})
			return
		}
	}
	// 年休假校验：修改为年休假或调整天数时校验当年额度（排除本记录自身已占用的天数）
	if req.LeaveType == "annual" {
		if len(req.StartDate) < 4 {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "开始日期格式错误，无法提取年份"})
			return
		}
		year := req.StartDate[:4]
		remain, configDays := getAnnualLeaveRemainDaysTx(tx, req.UserID, year, req.ID)
		if configDays <= 0 {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": year + "年度尚未配置年休假额度，请联系管理员配置"})
			return
		}
		if remain < req.Days {
			tx.Rollback()
			middleware.JSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("%s年度年休假剩余天数不足（剩余 %s 天，申请 %.1f 天）", year, strconv.FormatFloat(remain, 'f', -1, 64), req.Days),
			})
			return
		}
	}
	_, err = tx.Exec(
		`UPDATE leave_records SET user_id=?, leave_type=?, start_date=?, end_date=?, days=?, leave_hours=?, reason=? WHERE id=?`,
		req.UserID, req.LeaveType, req.StartDate, req.EndDate, req.Days, req.LeaveHours, req.Reason, req.ID)
	if err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
		return
	}

	// 级联回溯更新受影响日期的考勤点到记录（旧区间恢复/重新计算，新区间标记请假）
	if err := syncAttendanceForDateRangeTx(tx, oldUserID, oldStartDate, oldEndDate); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "同步考勤记录失败"})
		return
	}
	if err := syncAttendanceForDateRangeTx(tx, req.UserID, req.StartDate, req.EndDate); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "同步考勤记录失败"})
		return
	}

	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交事务失败"})
		return
	}

	var personName string
	database.DB.QueryRow("SELECT real_name FROM users WHERE id=?", req.UserID).Scan(&personName)
	logOperation(r, "请假管理", "修改", "修改请假记录："+personName+" "+leaveTypeLabel(req.LeaveType)+"（"+req.StartDate+" 至 "+req.EndDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
}

// DeleteLeaveRecord 删除请假记录
func DeleteLeaveRecord(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}

	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	var ownerID int64
	var personName, ltype, sdate, edate string
	err := database.DB.QueryRow(
		`SELECT l.user_id, u.real_name, l.leave_type, l.start_date, l.end_date 
		 FROM leave_records l 
		 LEFT JOIN users u ON l.user_id=u.id 
		 WHERE l.id=?`, id).
		Scan(&ownerID, &personName, &ltype, &sdate, &edate)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "请假记录不存在"})
		return
	} else if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 权限与归属校验：非 admin 用户必须校验记录所有者为当前登录用户（ownerID == userID）
	if roleCode != "admin" && ownerID != userID {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权删除他人的请假记录"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统繁忙，请稍后再试"})
		return
	}

	_, err = tx.Exec("DELETE FROM leave_records WHERE id=?", id)
	if err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}

	// 撤销请假后回溯更新覆盖时间段内的点到记录（若无其它有效请假则恢复正常出勤，防止误记旷工）
	if err := syncAttendanceForDateRangeTx(tx, ownerID, sdate, edate); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "回溯考勤记录失败"})
		return
	}

	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交事务失败"})
		return
	}

	logOperation(r, "请假管理", "删除", "删除请假记录："+personName+" "+leaveTypeLabel(ltype)+"（"+sdate+" 至 "+edate+"）")
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

	// 各类型天数统计（跨年假按当年实际覆盖天数拆分，与年休假/考勤口径一致；支持半天假）
	// 请假笔数 = 当年有覆盖的记录数；天数 = MIN(登记days, 当年重叠整天)，保留小数(半天/小时假)
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	query := `SELECT leave_type, COUNT(*) as cnt, SUM(eff) as total_days FROM (
			SELECT id, leave_type,
				MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND start_date <= ? AND end_date >= ?`
	args := []interface{}{yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart}
	if roleCode != "admin" {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	query += ` GROUP BY id
		) WHERE eff > 0 GROUP BY leave_type`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	result := map[string]interface{}{"year": year}
	totalDays := 0.0
	totalCount := 0
	typeItemMap := map[string]map[string]interface{}{}
	for _, lt := range LeaveTypes {
		result[lt+"_count"] = 0
		result[lt+"_days"] = 0
		typeItemMap[lt] = map[string]interface{}{
			"type":  lt,
			"count": 0,
			"days":  0.0,
		}
	}
	for rows.Next() {
		var lt string
		var cnt int
		var days sql.NullFloat64
		if err := rows.Scan(&lt, &cnt, &days); err != nil {
			continue
		}
		d := 0.0
		if days.Valid {
			d = days.Float64
		}
		result[lt+"_count"] = cnt
		result[lt+"_days"] = d
		totalDays += d
		totalCount += cnt
		if item, ok := typeItemMap[lt]; ok {
			item["count"] = cnt
			item["days"] = d
		} else {
			typeItemMap[lt] = map[string]interface{}{
				"type":  lt,
				"count": cnt,
				"days":  d,
			}
		}
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	typesList := []map[string]interface{}{}
	for _, lt := range LeaveTypes {
		typesList = append(typesList, typeItemMap[lt])
	}
	result["types"] = typesList
	result["total_count"] = totalCount
	result["total_days"] = totalDays
	middleware.JSON(w, http.StatusOK, result)
}

// 请假明细列（除"培训"外的类型；培训单列、补休归出勤）
const leaveDetailCase = `COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='annual' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='sick' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='personal' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='marriage' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='maternity' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='bereavement' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='prenatal' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND a.leave_type='family' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND (a.leave_type IS NULL OR a.leave_type NOT IN ('annual','sick','personal','marriage','maternity','bereavement','prenatal','family','training')) THEN 1 ELSE 0 END),0)`

// AttendanceMonthly 月度考勤统计（按人员）
// 口径统一以"点到记录"为准：出勤=status1（补休已按出勤）、出差/未到/迟到=3/4/5、
// 培训=status6+培训假、请假=status2（不含培训）；请假明细按 status2 的 leave_type 展开
func AttendanceMonthly(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month") // YYYY-MM
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	query := `SELECT u.id, u.real_name, d.name,
			COALESCE(SUM(CASE WHEN a.status=1 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=3 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=4 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=5 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=6 OR (a.status=2 AND a.leave_type='training') THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND (a.leave_type IS NULL OR a.leave_type<>'training') THEN 1 ELSE 0 END),0),
			` + leaveDetailCase + `
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
	defer rows.Close()

	type Row struct {
		UserID          int64  `json:"user_id"`
		UserName        string `json:"user_name"`
		Department      string `json:"department"`
		Present         int    `json:"present"`
		Trip            int    `json:"trip"`
		Absent          int    `json:"absent"`
		Late            int    `json:"late"`
		Training        int    `json:"training"`
		Leave           int    `json:"leave"`
		AnnualDays      int    `json:"annual_days"`
		SickDays        int    `json:"sick_days"`
		PersonalDays    int    `json:"personal_days"`
		MarriageDays    int    `json:"marriage_days"`
		MaternityDays   int    `json:"maternity_days"`
		BereavementDays int    `json:"bereavement_days"`
		PrenatalDays    int    `json:"prenatal_days"`
		FamilyDays      int    `json:"family_days"`
		OtherDays       int    `json:"other_days"`
	}
	list := []Row{}
	for rows.Next() {
		var rw Row
		var userName, dept sql.NullString
		if err := rows.Scan(&rw.UserID, &userName, &dept, &rw.Present, &rw.Trip, &rw.Absent, &rw.Late, &rw.Training, &rw.Leave,
			&rw.AnnualDays, &rw.SickDays, &rw.PersonalDays, &rw.MarriageDays, &rw.MaternityDays,
			&rw.BereavementDays, &rw.PrenatalDays, &rw.FamilyDays, &rw.OtherDays); err != nil {
			continue
		}
		rw.UserName = userName.String
		rw.Department = dept.String
		list = append(list, rw)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	total := map[string]int{
		"present": 0, "trip": 0, "absent": 0, "late": 0, "training": 0, "leave": 0,
		"annual_days": 0, "sick_days": 0, "personal_days": 0, "marriage_days": 0,
		"maternity_days": 0, "bereavement_days": 0, "prenatal_days": 0, "family_days": 0, "other_days": 0,
	}
	for _, rw := range list {
		total["present"] += rw.Present
		total["trip"] += rw.Trip
		total["absent"] += rw.Absent
		total["late"] += rw.Late
		total["training"] += rw.Training
		total["leave"] += rw.Leave
		total["annual_days"] += rw.AnnualDays
		total["sick_days"] += rw.SickDays
		total["personal_days"] += rw.PersonalDays
		total["marriage_days"] += rw.MarriageDays
		total["maternity_days"] += rw.MaternityDays
		total["bereavement_days"] += rw.BereavementDays
		total["prenatal_days"] += rw.PrenatalDays
		total["family_days"] += rw.FamilyDays
		total["other_days"] += rw.OtherDays
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"month": month, "list": list, "total": total})
}

// AttendanceYearly 年度考勤统计
// 返回：按月出勤汇总 + 每个干部全年各类休假天数（口径统一以"点到记录"为准）
func AttendanceYearly(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year") // YYYY
	if year == "" {
		year = time.Now().Format("2006")
	}

	// 按月汇总（与月度同口径：培训单列、请假不含培训）
	query := `SELECT substr(a.attend_date, 1, 7) as ym,
			COALESCE(SUM(CASE WHEN a.status=1 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=3 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=4 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=5 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=6 OR (a.status=2 AND a.leave_type='training') THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN a.status=2 AND (a.leave_type IS NULL OR a.leave_type<>'training') THEN 1 ELSE 0 END),0)
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
		Month    string `json:"month"`
		Present  int    `json:"present"`
		Trip     int    `json:"trip"`
		Absent   int    `json:"absent"`
		Late     int    `json:"late"`
		Training int    `json:"training"`
		Leave    int    `json:"leave"`
	}
	monthly := []MonthRow{}
	var totalPresent, totalTrip, totalAbsent, totalLate, totalTraining, totalLeave int
	for rows.Next() {
		var rw MonthRow
		var ym sql.NullString
		if err := rows.Scan(&ym, &rw.Present, &rw.Trip, &rw.Absent, &rw.Late, &rw.Training, &rw.Leave); err != nil {
			continue
		}
		rw.Month = ym.String
		monthly = append(monthly, rw)
		totalPresent += rw.Present
		totalTrip += rw.Trip
		totalAbsent += rw.Absent
		totalLate += rw.Late
		totalTraining += rw.Training
		totalLeave += rw.Leave
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 每个干部全年各类休假天数（来自点到记录；培训单列）
	personQuery := `SELECT u.id, u.real_name, d.name, ` + leaveDetailCase + `,
			COALESCE(SUM(CASE WHEN a.status=6 OR (a.status=2 AND a.leave_type='training') THEN 1 ELSE 0 END),0)
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN attendances a ON a.user_id = u.id AND a.attend_date LIKE ?
		WHERE u.status = 1
		GROUP BY u.id ORDER BY u.id`
	lrows, err := database.DB.Query(personQuery, year+"%")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer lrows.Close()

	type PersonRow struct {
		UserID          int64  `json:"user_id"`
		UserName        string `json:"user_name"`
		Department      string `json:"department"`
		AnnualDays      int    `json:"annual_days"`
		SickDays        int    `json:"sick_days"`
		PersonalDays    int    `json:"personal_days"`
		MarriageDays    int    `json:"marriage_days"`
		MaternityDays   int    `json:"maternity_days"`
		BereavementDays int    `json:"bereavement_days"`
		PrenatalDays    int    `json:"prenatal_days"`
		FamilyDays      int    `json:"family_days"`
		OtherDays       int    `json:"other_days"`
		TrainingDays    int    `json:"training_days"`
		TotalDays       int    `json:"total_days"`
	}
	persons := []PersonRow{}
	total := map[string]int{
		"annual_days": 0, "sick_days": 0, "personal_days": 0, "marriage_days": 0,
		"maternity_days": 0, "bereavement_days": 0, "prenatal_days": 0, "family_days": 0,
		"other_days": 0, "training_days": 0, "total_days": 0,
	}
	for lrows.Next() {
		var p PersonRow
		var userName, dept sql.NullString
		if err := lrows.Scan(&p.UserID, &userName, &dept, &p.AnnualDays, &p.SickDays, &p.PersonalDays,
			&p.MarriageDays, &p.MaternityDays, &p.BereavementDays, &p.PrenatalDays, &p.FamilyDays,
			&p.OtherDays, &p.TrainingDays); err != nil {
			continue
		}
		p.UserName = userName.String
		p.Department = dept.String
		p.TotalDays = p.AnnualDays + p.SickDays + p.PersonalDays + p.MarriageDays + p.MaternityDays +
			p.BereavementDays + p.PrenatalDays + p.FamilyDays + p.OtherDays + p.TrainingDays
		if p.TotalDays == 0 {
			continue // 无休假记录的不列出
		}
		persons = append(persons, p)
		total["annual_days"] += p.AnnualDays
		total["sick_days"] += p.SickDays
		total["personal_days"] += p.PersonalDays
		total["marriage_days"] += p.MarriageDays
		total["maternity_days"] += p.MaternityDays
		total["bereavement_days"] += p.BereavementDays
		total["prenatal_days"] += p.PrenatalDays
		total["family_days"] += p.FamilyDays
		total["other_days"] += p.OtherDays
		total["training_days"] += p.TrainingDays
		total["total_days"] += p.TotalDays
	}
	if err := lrows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"year": year, "monthly": monthly,
		"total": map[string]int{
			"present": totalPresent, "leave": totalLeave, "trip": totalTrip,
			"absent": totalAbsent, "late": totalLate, "training": totalTraining,
		},
		"persons":     persons,
		"leave_total": total,
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
		if err := rows.Scan(&a.ID, &a.RealName, &dept); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
		if dept.Valid {
			a.Department = dept.String
		}
		list = append(list, a)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// GetUserAttendanceProfile 获取干部个人出勤与休假全息档案
// 支持参数：year=YYYY（默认当前年）
// 权限：attendance.view 权限点，若非 admin 且无 user.manage 权限则只能查看本人档案
func GetUserAttendanceProfile(w http.ResponseWriter, r *http.Request) {
	currentUserID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	targetUserID := pathID(r)
	if targetUserID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少用户ID"})
		return
	}

	// 权限控制：管理员或拥有 user.manage 权限者可查任意人，普通用户只能查本人
	canViewAll := (roleCode == "admin") || middleware.HasPermission(roleCode, "user.manage")
	if !canViewAll && targetUserID != currentUserID {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权查看该人员的出勤档案"})
		return
	}

	year := r.URL.Query().Get("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	// 1. 用户基础信息
	var userInfo struct {
		ID         int64  `json:"id"`
		Username   string `json:"username"`
		RealName   string `json:"real_name"`
		Department string `json:"department"`
		RoleName   string `json:"role_name"`
		Phone      string `json:"phone"`
		Status     int    `json:"status"`
	}
	var dept, roleName, phone sql.NullString
	err := database.DB.QueryRow(`
		SELECT u.id, u.username, u.real_name, d.name, r.name, u.phone, u.status
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN roles r ON u.role_id = r.id
		WHERE u.id = ?`, targetUserID).Scan(
		&userInfo.ID, &userInfo.Username, &userInfo.RealName, &dept, &roleName, &phone, &userInfo.Status,
	)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	} else if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询用户信息失败"})
		return
	}
	userInfo.Department = dept.String
	userInfo.RoleName = roleName.String
	userInfo.Phone = phone.String

	// 2. 年休假统计（当年）
	remainAnnual, configAnnual := getAnnualLeaveRemainDays(targetUserID, year, 0)
	usedAnnual := configAnnual - remainAnnual
	if usedAnnual < 0 {
		usedAnnual = 0
	}

	// 3. 加班与补休统计
	// 当年加班工时
	var yearOtHours float64
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(hours), 0)
		FROM overtime_records
		WHERE user_id = ? AND overtime_date LIKE ?`, targetUserID, year+"%").Scan(&yearOtHours)

	// 累计总加班工时与历史总补休
	var totalOtHours float64
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(hours), 0)
		FROM overtime_records
		WHERE user_id = ?`, targetUserID).Scan(&totalOtHours)

	var totalCompUsed float64
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(eff), 0) FROM (
			SELECT MIN(days, CAST(julianday(end_date) - julianday(start_date) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'comp' AND user_id = ?
		) WHERE eff > 0`, targetUserID).Scan(&totalCompUsed)

	// 当年已用补休
	yearStart := year + "-01-01"
	yearEnd := year + "-12-31"
	var yearCompUsed float64
	database.DB.QueryRow(`
		SELECT COALESCE(SUM(eff), 0) FROM (
			SELECT MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
				- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND leave_type = 'comp' AND user_id = ? AND start_date <= ? AND end_date >= ?
		) WHERE eff > 0`, yearEnd, yearEnd, yearStart, yearStart, targetUserID, yearEnd, yearStart).Scan(&yearCompUsed)

	totalCompDays := totalOtHours / OvertimeHoursPerDay
	remainCompDays := totalCompDays - totalCompUsed
	if remainCompDays < 0 {
		remainCompDays = 0
	}

	// 4. 当年考勤点到状态统计 (1出勤 2请假 3出差 4未到 5迟到 6培训)
	attendStats := map[string]int{
		"present": 0, "leave": 0, "trip": 0, "absent": 0, "late": 0, "training": 0, "total": 0,
	}
	attRows, err := database.DB.Query(`
		SELECT status, COUNT(*)
		FROM attendances
		WHERE user_id = ? AND attend_date LIKE ?
		GROUP BY status`, targetUserID, year+"%")
	if err == nil {
		defer attRows.Close()
		for attRows.Next() {
			var st, cnt int
			if err := attRows.Scan(&st, &cnt); err == nil {
				attendStats["total"] += cnt
				switch st {
				case 1:
					attendStats["present"] = cnt
				case 2:
					attendStats["leave"] = cnt
				case 3:
					attendStats["trip"] = cnt
				case 4:
					attendStats["absent"] = cnt
				case 5:
					attendStats["late"] = cnt
				case 6:
					attendStats["training"] = cnt
				}
			}
		}
	}

	// 5. 当年各类请假统计明细
	leaveSummary := make([]map[string]interface{}, 0, len(LeaveTypes))
	leaveTypeMap := map[string]float64{}
	for _, lt := range LeaveTypes {
		leaveTypeMap[lt] = 0
	}

	ltRows, err := database.DB.Query(`
		SELECT leave_type, SUM(eff) FROM (
			SELECT leave_type,
				MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE status = 1 AND user_id = ? AND start_date <= ? AND end_date >= ?
		) WHERE eff > 0 GROUP BY leave_type`,
		yearEnd, yearEnd, yearStart, yearStart, targetUserID, yearEnd, yearStart)
	if err == nil {
		defer ltRows.Close()
		for ltRows.Next() {
			var lt string
			var d float64
			if err := ltRows.Scan(&lt, &d); err == nil {
				leaveTypeMap[lt] = d
			}
		}
	}

	totalLeaveDays := 0.0
	for _, lt := range LeaveTypes {
		days := leaveTypeMap[lt]
		totalLeaveDays += days
		leaveSummary = append(leaveSummary, map[string]interface{}{
			"type":  lt,
			"label": leaveTypeLabel(lt),
			"days":  days,
		})
	}

	// 6. 当年全部请假流水记录 (时间倒序)
	leaveList := make([]models.LeaveRecord, 0)
	recRows, err := database.DB.Query(`
		SELECT id, user_id, leave_type, start_date, end_date, days, leave_hours, reason, status, created_at
		FROM leave_records
		WHERE status = 1 AND user_id = ? AND start_date <= ? AND end_date >= ?
		ORDER BY start_date DESC, id DESC`,
		targetUserID, yearEnd, yearStart)
	if err == nil {
		defer recRows.Close()
		for recRows.Next() {
			var rRec models.LeaveRecord
			var reason sql.NullString
			if err := recRows.Scan(&rRec.ID, &rRec.UserID, &rRec.LeaveType, &rRec.StartDate, &rRec.EndDate,
				&rRec.Days, &rRec.LeaveHours, &reason, &rRec.Status, &rRec.CreatedAt); err == nil {
				rRec.Reason = reason.String
				rRec.UserName = userInfo.RealName
				rRec.Department = userInfo.Department
				leaveList = append(leaveList, rRec)
			}
		}
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"year": year,
		"user": userInfo,
		"annual_leave": map[string]interface{}{
			"config_days": configAnnual,
			"used_days":   usedAnnual,
			"remain_days": remainAnnual,
		},
		"overtime": map[string]interface{}{
			"year_hours":       yearOtHours,
			"total_hours":      totalOtHours,
			"total_comp_days":  totalCompDays,
			"total_comp_used":  totalCompUsed,
			"year_comp_used":   yearCompUsed,
			"remain_comp_days": remainCompDays,
		},
		"attendance":       attendStats,
		"leave_summary":    leaveSummary,
		"total_leave_days": totalLeaveDays,
		"leave_records":    leaveList,
	})
}
