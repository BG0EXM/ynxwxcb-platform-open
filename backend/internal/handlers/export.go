package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xuri/excelize/v2"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

type sqlStr = sql.NullString

// formatDateStr 将 YYYY-MM-DD 转为 YYYY/M/D（去前导零），空串原样返回
func formatDateStr(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("2006/1/2")
}

// formatDateTime 将时间戳转为 YYYY/M/D 15:04
func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006/1/2 15:04")
}

// exportExcel 生成 Excel 并写入响应
// headers: 列标题; rows: 数据行（每行 []interface{}）; sheetName: 工作表名; fileName: 下载文件名
func exportExcel(w http.ResponseWriter, sheetName, fileName string, headers []string, rows [][]interface{}) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	if sheetName != "" {
		sheet = sheetName
	}
	f.SetSheetName("Sheet1", sheet)

	// 写表头
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	// 表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4472C4"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// 写数据
	for r, row := range rows {
		for c, val := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	// 数据行边框
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Style: 1}, {Type: "right", Style: 1},
			{Type: "top", Style: 1}, {Type: "bottom", Style: 1},
		},
	})
	if len(rows) > 0 {
		f.SetRowStyle(sheet, 2, len(rows)+1, dataStyle)
	}
	// 列宽自适应（粗略）
	f.SetColWidth(sheet, "A", "Z", 18)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fileName, url.PathEscape(fileName)))
	if err := f.Write(w); err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
	}
}

// ExportVehicleApplies 导出用车报备 Excel
func ExportVehicleApplies(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	query := `SELECT a.id, a.use_date, a.use_time, v.plate_no, v.brand, a.user_name,
			a.destination, a.purpose, a.passengers, u.real_name, a.created_at
		FROM vehicle_applies a
		LEFT JOIN vehicles v ON a.vehicle_id = v.id
		LEFT JOIN users u ON a.reporter_id = u.id
		WHERE 1=1`
	args := []interface{}{}
	if date := r.URL.Query().Get("date"); date != "" {
		query += ` AND a.use_date = ?`
		args = append(args, date)
	}
	if roleCode != "admin" {
		query += ` AND a.reporter_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY a.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	headers := []string{"序号", "用车日期", "用车时间", "车牌号", "车型", "用车人", "目的地", "事由", "人数", "报备人", "报备时间"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var useDate, useTime, plateNo, brand, userName, destination, purpose sqlStr
		var passengers int
		var reporter sqlStr
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &useDate, &useTime, &plateNo, &brand, &userName, &destination, &purpose, &passengers, &reporter, &createdAt); err != nil {
			continue
		}
		data = append(data, []interface{}{
			idx, formatDateStr(useDate.String), useTime.String, plateNo.String, brand.String,
			userName.String, destination.String, purpose.String, passengers, reporter.String,
			formatDateTime(createdAt.Time),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "用车管理", "导出", "导出用车报备台账")
	exportExcel(w, "用车报备", "用车报备台账.xlsx", headers, data)
}

// ExportLeaveRecords 导出请假记录 Excel
func ExportLeaveRecords(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	query := `SELECT l.id, u.real_name, l.leave_type, l.start_date, l.end_date, l.days, l.reason, l.created_at
		FROM leave_records l LEFT JOIN users u ON l.user_id = u.id WHERE 1=1`
	args := []interface{}{}
	if roleCode != "admin" {
		query += ` AND l.user_id = ?`
		args = append(args, userID)
	} else if uid := r.URL.Query().Get("user_id"); uid != "" {
		query += ` AND l.user_id = ?`
		args = append(args, uid)
	}
	if t := r.URL.Query().Get("leave_type"); t != "" {
		query += ` AND l.leave_type = ?`
		args = append(args, t)
	}
	if m := r.URL.Query().Get("month"); m != "" {
		query += ` AND l.start_date LIKE ?`
		args = append(args, m+"%")
	}
	query += ` ORDER BY l.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	leaveTypeNames := map[string]string{
		"annual": "年假", "sick": "病假", "personal": "事假", "marriage": "婚假",
		"maternity": "产假", "bereavement": "丧假", "prenatal": "产检假", "family": "探亲假", "training": "培训",
		"comp": "补休", "other": "其他",
	}

	headers := []string{"序号", "姓名", "请假类型", "开始日期", "结束日期", "天数", "事由", "登记时间"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var userName, leaveType, startDate, endDate, reason sqlStr
		var days float64
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &userName, &leaveType, &startDate, &endDate, &days, &reason, &createdAt); err != nil {
			continue
		}
		data = append(data, []interface{}{
			idx, userName.String, leaveTypeNames[leaveType.String], formatDateStr(startDate.String),
			formatDateStr(endDate.String), days, reason.String, formatDateTime(createdAt.Time),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "请假管理", "导出", "导出请假记录台账")
	exportExcel(w, "请假记录", "请假记录台账.xlsx", headers, data)
}

// ExportAttendances 导出考勤记录 Excel
func ExportAttendances(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	date := r.URL.Query().Get("date")
	month := r.URL.Query().Get("month")

	query := `SELECT a.id, u.real_name, a.attend_date, a.status, a.leave_type, a.remark, a.created_at
		FROM attendances a LEFT JOIN users u ON a.user_id = u.id WHERE 1=1`
	args := []interface{}{}
	if date != "" {
		query += ` AND a.attend_date = ?`
		args = append(args, date)
	}
	if month != "" {
		query += ` AND a.attend_date LIKE ?`
		args = append(args, month+"%")
	}
	if roleCode != "admin" {
		query += ` AND a.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY a.attend_date DESC, a.user_id`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	statusNames := map[int]string{1: "出勤", 2: "请假", 3: "出差", 4: "未到", 5: "迟到", 6: "培训"}
	leaveTypeNames := map[string]string{
		"annual": "年假", "sick": "病假", "personal": "事假", "marriage": "婚假",
		"maternity": "产假", "bereavement": "丧假", "prenatal": "产检假", "family": "探亲假", "training": "培训",
		"comp": "补休", "other": "其他",
	}

	headers := []string{"序号", "姓名", "日期", "状态", "请假类型", "备注"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var userName, attendDate, leaveType, remark sqlStr
		var status int
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &userName, &attendDate, &status, &leaveType, &remark, &createdAt); err != nil {
			continue
		}
		statusText := statusNames[status]
		if status == 2 && leaveType.Valid && leaveType.String != "" {
			statusText += "（" + leaveTypeNames[leaveType.String] + "）"
		}
		data = append(data, []interface{}{
			idx, userName.String, formatDateStr(attendDate.String), statusText, leaveTypeNames[leaveType.String], remark.String,
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "考勤管理", "导出", "导出考勤记录台账")
	exportExcel(w, "考勤记录", "考勤记录台账.xlsx", headers, data)
}

// ExportDutySchedules 导出值守排班 Excel
func ExportDutySchedules(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")

	query := `SELECT s.duty_date, u.real_name, s.is_dawangyuan, s.note, s.created_at
		FROM duty_schedules s LEFT JOIN users u ON s.user_id = u.id WHERE 1=1`
	args := []interface{}{}
	if month != "" {
		query += ` AND s.duty_date LIKE ?`
		args = append(args, month+"%")
	}
	query += ` ORDER BY s.duty_date`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	headers := []string{"序号", "值守日期", "值守人员", "县委大院", "备注"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var dutyDate, userName, note sqlStr
		var isDaWangYuan int
		var createdAt sql.NullTime
		if err := rows.Scan(&dutyDate, &userName, &isDaWangYuan, &note, &createdAt); err != nil {
			continue
		}
		dwy := ""
		if isDaWangYuan == 1 {
			dwy = "是"
		}
		data = append(data, []interface{}{
			idx, formatDateStr(dutyDate.String), userName.String, dwy, note.String,
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "值守排班", "导出", "导出值守排班台账")
	exportExcel(w, "值守排班", "值守排班台账.xlsx", headers, data)
}

// ExportIncomingDocs 导出收文登记 Excel
func ExportIncomingDocs(w http.ResponseWriter, r *http.Request) {
	query := `SELECT d.id, d.receive_no, d.received_date, d.from_unit, d.from_doc_no, d.title,
		d.secret_level, d.urgency, d.copies, d.status, u.real_name, d.created_at
		FROM incoming_docs d LEFT JOIN users u ON d.registrar_id = u.id WHERE 1=1`
	args := []interface{}{}
	if keyword := r.URL.Query().Get("keyword"); keyword != "" {
		query += ` AND (d.title LIKE ? OR d.from_unit LIKE ? OR d.receive_no LIKE ?)`
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query += ` AND d.status = ?`
		args = append(args, status)
	}
	if returned := r.URL.Query().Get("returned"); returned != "" {
		query += ` AND d.returned = ?`
		args = append(args, returned)
	}
	if needReturn := r.URL.Query().Get("need_return"); needReturn != "" {
		query += ` AND d.need_return = ?`
		args = append(args, needReturn)
	}
	if start := r.URL.Query().Get("start"); start != "" {
		query += ` AND d.received_date >= ?`
		args = append(args, start)
	}
	if end := r.URL.Query().Get("end"); end != "" {
		query += ` AND d.received_date <= ?`
		args = append(args, end)
	}
	query += ` ORDER BY d.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	statusNames := map[int]string{1: "待登记", 2: "拟办中", 3: "待批示", 4: "办理中", 5: "已办结"}

	headers := []string{"序号", "收文编号", "收文日期", "来文单位", "来文字号", "文件标题", "密级", "紧急程度", "份数", "状态", "登记人", "登记时间"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var receiveNo, receivedDate, fromUnit, fromDocNo, title, secretLevel, urgency, realName sqlStr
		var copies, status int
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &receiveNo, &receivedDate, &fromUnit, &fromDocNo, &title,
			&secretLevel, &urgency, &copies, &status, &realName, &createdAt); err != nil {
			continue
		}
		data = append(data, []interface{}{
			idx, receiveNo.String, formatDateStr(receivedDate.String), fromUnit.String, fromDocNo.String,
			title.String, secretLevel.String, urgency.String, copies, statusNames[status],
			realName.String, formatDateTime(createdAt.Time),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "收文管理", "导出", "导出收文登记台账")
	exportExcel(w, "收文登记", "收文登记台账.xlsx", headers, data)
}
