package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListMeetings 会议列表（管理员）
func ListMeetings(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM meetings").Scan(&total)

	query := `SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit,
			m.created_by, u.real_name, m.created_at, m.updated_at,
			COALESCE(SUM(CASE WHEN r.not_attend = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN r.not_attend = 1 THEN 1 ELSE 0 END), 0)
		FROM meetings m
		LEFT JOIN users u ON m.created_by = u.id
		LEFT JOIN meeting_registrations r ON r.meeting_id = m.id
		GROUP BY m.id ORDER BY m.meeting_date DESC, m.id DESC LIMIT ? OFFSET ?`
	rows, err := database.DB.Query(query, pageSize, offset)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.Meeting{}
	for rows.Next() {
		var m models.Meeting
		var creator, meetingDate, meetingTime, location, content, units sql.NullString
		var createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		var regCount, notAttend int
		if err := rows.Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &content, &units, &m.UnitLimit,
			&createdBy, &creator, &createdAt, &updatedAt, &regCount, &notAttend); err != nil {
			continue
		}
		m.MeetingDate = meetingDate.String
		m.MeetingTime = meetingTime.String
		m.Location = location.String
		m.Content = content.String
		m.Units = units.String
		m.CreatedBy = createdBy.Int64
		m.CreatedName = creator.String
		if createdAt.Valid {
			m.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			m.UpdatedAt = updatedAt.Time
		}
		m.RegCount = regCount
		m.NotAttend = notAttend
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "total": total})
}

// CreateMeeting 新增会议（管理员）
func CreateMeeting(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.Meeting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议标题必填"})
		return
	}
	if !isValidDate(req.MeetingDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择会议日期（YYYY-MM-DD）"})
		return
	}
	if strings.TrimSpace(req.Units) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写参会单位范围"})
		return
	}
	if req.UnitLimit < 0 {
		req.UnitLimit = 0
	}
	// 显式指定 id = 当前最大 id + 1，删除会议后 ID 不跳号（复用）；单条语句保证取号原子
	res, err := database.DB.Exec(
		`INSERT INTO meetings (id, title, meeting_date, meeting_time, location, content, units, unit_limit, created_by)
		 SELECT COALESCE(MAX(id), 0) + 1, ?, ?, ?, ?, ?, ?, ?, ? FROM meetings`,
		req.Title, req.MeetingDate, req.MeetingTime, req.Location, req.Content, req.Units, req.UnitLimit, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	lastID, _ := res.LastInsertId()
	logOperation(r, "会务管理", "新增", "新增会议「"+req.Title+"」（"+req.MeetingDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "创建成功", "id": lastID})
}

// UpdateMeeting 修改会议（管理员）
func UpdateMeeting(w http.ResponseWriter, r *http.Request) {
	var req models.Meeting
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议标题必填"})
		return
	}
	if !isValidDate(req.MeetingDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择会议日期（YYYY-MM-DD）"})
		return
	}
	if strings.TrimSpace(req.Units) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写参会单位范围"})
		return
	}
	if req.UnitLimit < 0 {
		req.UnitLimit = 0
	}
	_, err := database.DB.Exec(
		`UPDATE meetings SET title=?, meeting_date=?, meeting_time=?, location=?, content=?, units=?, unit_limit=?, updated_at=? WHERE id=?`,
		req.Title, req.MeetingDate, req.MeetingTime, req.Location, req.Content, req.Units, req.UnitLimit, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	logOperation(r, "会务管理", "修改", "修改会议「"+req.Title+"」（"+req.MeetingDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteMeeting 删除会议（管理员，级联删报名）
func DeleteMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var meetTitle string
	database.DB.QueryRow("SELECT title FROM meetings WHERE id=?", id).Scan(&meetTitle)
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if _, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=?", id); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if _, err := tx.Exec("DELETE FROM meetings WHERE id=?", id); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	// 重置自增序列，使删除的会议 ID 可被复用（MAX(id)+1 连续不跳号）
	if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name='meetings'"); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if _, err := tx.Exec("INSERT OR REPLACE INTO sqlite_sequence (name, seq) VALUES ('meetings', (SELECT COALESCE(MAX(id),0) FROM meetings))"); err != nil {
		tx.Rollback()
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	logOperation(r, "会务管理", "删除", "删除会议「"+meetTitle+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// GetMeeting 会议详情（管理员，含报名列表）
func GetMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var m models.Meeting
	var creator, meetingDate, meetingTime, location, content, units sql.NullString
	var createdBy sql.NullInt64
	var createdAt, updatedAt sql.NullTime
	err := database.DB.QueryRow(
		`SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit,
			m.created_by, u.real_name, m.created_at, m.updated_at
		FROM meetings m LEFT JOIN users u ON m.created_by = u.id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &content, &units, &m.UnitLimit,
			&createdBy, &creator, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	m.MeetingDate = meetingDate.String
	m.MeetingTime = meetingTime.String
	m.Location = location.String
	m.Content = content.String
	m.Units = units.String
	m.CreatedBy = createdBy.Int64
	m.CreatedName = creator.String
	if createdAt.Valid {
		m.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		m.UpdatedAt = updatedAt.Time
	}

	// 报名列表
	regRows, err := database.DB.Query(
		`SELECT id, meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason, created_at
		 FROM meeting_registrations WHERE meeting_id=? ORDER BY not_attend, id`, id)
	regs := []models.MeetingRegistration{}
	registeredUnits := map[string]bool{}
	attendCount := 0
	if err == nil {
		defer regRows.Close()
		for regRows.Next() {
			var rg models.MeetingRegistration
			var unit, name, title, phone, reason sql.NullString
			var createdAt sql.NullTime
			scanErr := regRows.Scan(&rg.ID, &rg.MeetingID, &unit, &name, &title, &phone, &rg.NotAttend, &reason, &createdAt)
			if scanErr != nil {
				continue
			}
			rg.Unit = unit.String
			rg.AttendeeName = name.String
			rg.AttendeeTitle = title.String
			rg.Phone = phone.String
			rg.Reason = reason.String
			if createdAt.Valid {
				rg.CreatedAt = createdAt.Time
			}
			if rg.NotAttend == 0 {
				rg.IsAttending = 1
				attendCount++
			} else {
				rg.IsAttending = 0
			}
			regs = append(regs, rg)
			registeredUnits[unit.String] = true
		}
		if err := regRows.Err(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
		}
	}
	// 报名人数只统计实际参加（不含整体不参加标记）
	m.RegCount = attendCount

	// 未确认单位：会议录入的全部单位中，尚无任何报名记录（未确认）的
	unconfirmed := []string{}
	for _, u := range strings.Split(m.Units, "\n") {
		u = strings.TrimSpace(u)
		if u != "" && !registeredUnits[u] {
			unconfirmed = append(unconfirmed, u)
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"meeting":           m,
		"registrations":     regs,
		"list":              regs,
		"unconfirmed":       unconfirmed,
		"unconfirmed_units": unconfirmed,
	})
}

// PublicMeeting 公开会议信息（匿名，供报名页加载）
func PublicMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	var m models.Meeting
	var meetingDate, meetingTime, location, content, unitsRaw sql.NullString
	err := database.DB.QueryRow(
		`SELECT id, title, meeting_date, meeting_time, location, content, units, unit_limit FROM meetings WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &content, &unitsRaw, &m.UnitLimit)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	m.MeetingDate = meetingDate.String
	m.MeetingTime = meetingTime.String
	m.Location = location.String
	m.Content = content.String
	m.Units = unitsRaw.String
	// unit_limit: 1=单人  >1=多人上限  0/空=不限制（前端据此展示）
	// 单位列表转数组
	units := []string{}
	for _, u := range strings.Split(m.Units, "\n") {
		u = strings.TrimSpace(u)
		if u != "" {
			units = append(units, u)
		}
	}
	// 会议是否已过期（会议开始时间=meeting_date + meeting_time，过后报名截止）
	// 日期缺失/非法时视为已过期，避免无法判定截止时间而永久开放
	expired := true
	if t, ok := parseMeetingTime(m.MeetingDate, m.MeetingTime); ok {
		expired = time.Now().After(t)
	}
	// 该单位报名情况：返回已报人员列表（参加），及是否整体不参加
	regs := []map[string]interface{}{}
	notAttendAll := false
	notAttendReason := ""
	if unit := r.URL.Query().Get("unit"); unit != "" {
		rows, qerr := database.DB.Query(
			`SELECT id, attendee_name, attendee_title, phone, not_attend, reason FROM meeting_registrations
			 WHERE meeting_id=? AND unit=? ORDER BY id`, id, unit)
		if qerr == nil {
			defer rows.Close()
			for rows.Next() {
				var rgID int64
				var name, title, phone, reason sql.NullString
				var na int
				if err := rows.Scan(&rgID, &name, &title, &phone, &na, &reason); err != nil {
					continue
				}
				if na == 1 {
					notAttendAll = true
					notAttendReason = reason.String
					continue
				}
				regs = append(regs, map[string]interface{}{
					"id": rgID, "attendee_name": name.String, "attendee_title": title.String, "phone": maskPhone(phone.String),
				})
			}
		}
	}
	// 剩余可报 = 上限 - 已参加人数；unit_limit<=0 表示不限制（remain=-1）
	remain := -1
	if m.UnitLimit > 0 {
		remain = m.UnitLimit - len(regs)
		if remain < 0 {
			remain = 0
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"meeting": m, "units": units, "expired": expired,
		"registrations": regs, "remain": remain,
		"not_attend_all": notAttendAll, "not_attend_reason": notAttendReason,
	})
}

// parseMeetingTime 解析会议开始时间（YYYY-MM-DD + HH:mm）
// 未填时间时截止到当天 23:59:59（当天报名仍有效），返回时间与是否可解析
func parseMeetingTime(date, tm string) (time.Time, bool) {
	full := date
	layout := "2006-01-02"
	if tm != "" {
		if _, err := time.Parse("15:04", tm); err == nil {
			layout = "2006-01-02 15:04"
			full = date + " " + tm
		} else {
			// 时间格式非法：按当天末
			t, err := time.ParseInLocation("2006-01-02", date, time.Local)
			if err != nil {
				return time.Time{}, false
			}
			return t.Add(24*time.Hour - time.Second), true
		}
	} else {
		// 未填时间：截止到当天 23:59:59
		t, err := time.ParseInLocation("2006-01-02", date, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return t.Add(24*time.Hour - time.Second), true
	}
	t, err := time.ParseInLocation(layout, full, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// PublicRegisterMeeting 公开报名（匿名）
// body: {"meeting_id":1,"unit":"xx","attendee_name":"","attendee_title":"","phone":"","not_attend":0,"reason":"",
//
//	"reg_id":0}   reg_id>0 表示修改某条已有报名
func PublicRegisterMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var req struct {
		MeetingID     int64  `json:"meeting_id"`
		RegID         int64  `json:"reg_id"`
		Unit          string `json:"unit"`
		AttendeeName  string `json:"attendee_name"`
		AttendeeTitle string `json:"attendee_title"`
		Phone         string `json:"phone"`
		NotAttend     int    `json:"not_attend"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	meetingID := req.MeetingID
	if meetingID == 0 {
		meetingID = id
	}
	if meetingID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	if req.Unit == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择参会单位"})
		return
	}
	// 校验会议存在 + 过期 + 上限
	var md, mt, unitsRaw sql.NullString
	var unitLimit int
	err := database.DB.QueryRow("SELECT meeting_date, meeting_time, unit_limit, units FROM meetings WHERE id=?", meetingID).
		Scan(&md, &mt, &unitLimit, &unitsRaw)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	t, ok := parseMeetingTime(md.String, mt.String)
	if !ok || time.Now().After(t) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议已开始，报名已截止"})
		return
	}
	// 校验单位属于会议参会单位范围
	unitOK := false
	for _, u := range strings.Split(unitsRaw.String, "\n") {
		if strings.TrimSpace(u) == req.Unit {
			unitOK = true
			break
		}
	}
	if !unitOK {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位不在本次会议参会范围内"})
		return
	}
	// 校验该单位当前报名状态
	var attendCount, notAll int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit).Scan(&attendCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=1", meetingID, req.Unit).Scan(&notAll)

	if req.NotAttend == 1 {
		if req.Reason == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不参加请填写原因"})
			return
		}
		// 方案 A：该单位若已有参会人员，禁止公开端一键清空覆写为不参加，须联系管理员
		if attendCount > 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位已报名参会人员，如需变更为不参加请联系会务组管理员"})
			return
		}
		tx, err := database.DB.Begin()
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交失败"})
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", meetingID, req.Unit); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交失败"})
			return
		}
		if _, err := tx.Exec(
			`INSERT INTO meeting_registrations (meeting_id, unit, not_attend, reason) VALUES (?, ?, 1, ?)`,
			meetingID, req.Unit, req.Reason); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交失败"})
			return
		}
		if err := tx.Commit(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交失败"})
			return
		}
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "已确认不参加"})
		return
	}
	// 参加：
	if req.AttendeeName == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写参会人员姓名"})
		return
	}
	if req.AttendeeTitle == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写职务"})
		return
	}
	// 修改已有报名时允许手机号留空（保持原号码，因匿名接口返回的是脱敏号）
	keepPhone := req.RegID > 0 && req.Phone == ""
	if !keepPhone && !isValidPhone(req.Phone) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请输入正确的11位手机号"})
		return
	}

	if req.RegID > 0 {
		// 修改已有报名（手机号留空则保留原号码）
		if keepPhone {
			_, err = database.DB.Exec(
				`UPDATE meeting_registrations SET attendee_name=?, attendee_title=? WHERE id=? AND meeting_id=? AND unit=?`,
				req.AttendeeName, req.AttendeeTitle, req.RegID, meetingID, req.Unit)
		} else {
			_, err = database.DB.Exec(
				`UPDATE meeting_registrations SET attendee_name=?, attendee_title=?, phone=? WHERE id=? AND meeting_id=? AND unit=?`,
				req.AttendeeName, req.AttendeeTitle, req.Phone, req.RegID, meetingID, req.Unit)
		}
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
			return
		}
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
		return
	}

	// 新增人员：使用事务包裹并在排他锁保护下核验人数，彻底消除 Race Condition 并发超额
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
		return
	}
	defer tx.Rollback()

	// 事务内重新核验当前人数，防止并发超额
	var curAttendCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit).Scan(&curAttendCount); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
		return
	}
	if unitLimit > 1 && curAttendCount >= unitLimit {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位报名人数已达上限"})
		return
	}

	if notAll > 0 {
		if _, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=1", meetingID, req.Unit); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
			return
		}
	}
	if unitLimit == 1 && curAttendCount >= 1 {
		if _, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
			return
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO meeting_registrations (meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason) VALUES (?, ?, ?, ?, ?, 0, '')`,
		meetingID, req.Unit, req.AttendeeName, req.AttendeeTitle, req.Phone); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "报名成功"})
}

// PublicRemoveAttendee 移除某条参会报名（匿名，仅同单位可操作）
// body: {"reg_id":123}
func PublicRemoveAttendee(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var req struct {
		RegID int64  `json:"reg_id"`
		Unit  string `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.RegID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	// 校验会议未过期
	var md, mt sql.NullString
	err := database.DB.QueryRow("SELECT meeting_date, meeting_time FROM meetings WHERE id=?", id).Scan(&md, &mt)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	if t, ok := parseMeetingTime(md.String, mt.String); !ok || time.Now().After(t) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议已开始，报名已截止"})
		return
	}
	// 方案 A：若该单位仅余一名参会人员，禁止直接移除清空为 0 人；换人请点修改，全员取消联系管理员
	var attendCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", id, req.Unit).Scan(&attendCount)
	if attendCount <= 1 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位仅余一名参会人员，不可直接移除；如需更换人员请直接点击修改，如需取消参会请联系会务组管理员"})
		return
	}

	res, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE id=? AND meeting_id=? AND unit=? AND not_attend=0", req.RegID, id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "记录不存在"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已移除"})
}

// PublicCancelMeetingRegister 取消报名（匿名，反悔机会）
// body: {"unit":"xx"}
func PublicCancelMeetingRegister(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	var req struct {
		Unit string `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Unit == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少单位"})
		return
	}
	// 校验会议未过期（会议开始后不可再取消）
	var md, mt sql.NullString
	err := database.DB.QueryRow("SELECT meeting_date, meeting_time FROM meetings WHERE id=?", id).Scan(&md, &mt)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	if t, ok := parseMeetingTime(md.String, mt.String); !ok || time.Now().After(t) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议已开始，报名已截止"})
		return
	}
	// 方案 A：若该单位已报参会人员，禁止公开端一键全员取消清空，防止被他人恶意破坏
	var attendCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", id, req.Unit).Scan(&attendCount)
	if attendCount > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位已提交参会人员名单，如需全员取消请联系会务组管理员"})
		return
	}

	res, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "取消失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "该单位没有报名记录"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已取消"})
}

// ExportMeetingRegistration 导出会议签到单 Excel（管理员）
// 列出会议全部参会单位：已确认的填人员信息，未确认的留空（供现场签到），不参加备注
func ExportMeetingRegistration(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		if qid := r.URL.Query().Get("meeting_id"); qid != "" {
			if parsed, err := strconv.ParseInt(qid, 10, 64); err == nil {
				id = parsed
			}
		}
	}
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var m models.Meeting
	var meetingDate, meetingTime, location, unitsRaw sql.NullString
	err := database.DB.QueryRow(
		`SELECT id, title, meeting_date, meeting_time, location, units FROM meetings WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &unitsRaw)
	if err != nil {
		http.Error(w, "会议不存在", http.StatusNotFound)
		return
	}
	m.MeetingDate = meetingDate.String
	m.MeetingTime = meetingTime.String
	m.Location = location.String
	m.Units = unitsRaw.String

	// 读取该会议所有报名记录，按单位归类
	type regInfo struct {
		name, title, phone string
		notAttend          int
		reason             string
	}
	regMap := map[string][]regInfo{}
	regRows, err := database.DB.Query(
		`SELECT unit, attendee_name, attendee_title, phone, not_attend, reason
		 FROM meeting_registrations WHERE meeting_id=? ORDER BY id`, id)
	if err == nil {
		defer regRows.Close()
		for regRows.Next() {
			var unit, name, title, phone, reason sql.NullString
			var na int
			if err := regRows.Scan(&unit, &name, &title, &phone, &na, &reason); err != nil {
				continue
			}
			regMap[unit.String] = append(regMap[unit.String], regInfo{
				name: name.String, title: title.String, phone: phone.String, notAttend: na, reason: reason.String,
			})
		}
	}

	// 参会单位列表（按录入顺序）
	units := []string{}
	for _, u := range strings.Split(m.Units, "\n") {
		u = strings.TrimSpace(u)
		if u != "" {
			units = append(units, u)
		}
	}

	headers := []string{"序号", "参会单位", "姓名", "职务", "电话", "签到", "备注"}
	data := [][]interface{}{}
	idx := 1
	for _, unit := range units {
		regs := regMap[unit]
		if len(regs) == 0 {
			// 未确认：单位名列出，人员留空供现场签到
			data = append(data, []interface{}{idx, unit, "", "", "", "", ""})
			idx++
			continue
		}
		for _, rg := range regs {
			if rg.notAttend == 1 {
				// 已确认不参加
				remark := "已确认不参加"
				if rg.reason != "" {
					remark += "（" + rg.reason + "）"
				}
				data = append(data, []interface{}{idx, unit, "", "", "", "", remark})
			} else {
				data = append(data, []interface{}{idx, unit, rg.name, rg.title, rg.phone, "", ""})
			}
			idx++
		}
	}
	logOperation(r, "会务管理", "导出", "导出会议签到单「"+m.Title+"」")
	fileName := "会议签到单-" + m.Title + ".xlsx"
	exportExcel(w, "会议签到单", fileName, headers, data)
}

// MeetingRegistrations 某会议报名列表（管理员）
func MeetingRegistrations(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var unitsRaw sql.NullString
	database.DB.QueryRow(`SELECT units FROM meetings WHERE id=?`, id).Scan(&unitsRaw)

	regRows, err := database.DB.Query(
		`SELECT id, meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason, created_at
		 FROM meeting_registrations WHERE meeting_id=? ORDER BY not_attend, id`, id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer regRows.Close()
	regs := []models.MeetingRegistration{}
	registeredUnits := map[string]bool{}
	for regRows.Next() {
		var rg models.MeetingRegistration
		var unit, name, title, phone, reason sql.NullString
		scanErr := regRows.Scan(&rg.ID, &rg.MeetingID, &unit, &name, &title, &phone, &rg.NotAttend, &reason, &rg.CreatedAt)
		if scanErr != nil {
			continue
		}
		rg.Unit = unit.String
		rg.AttendeeName = name.String
		rg.AttendeeTitle = title.String
		rg.Phone = phone.String
		rg.Reason = reason.String
		if rg.NotAttend == 0 {
			rg.IsAttending = 1
		} else {
			rg.IsAttending = 0
		}
		regs = append(regs, rg)
		registeredUnits[unit.String] = true
	}
	if err := regRows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	unconfirmed := []string{}
	for _, u := range strings.Split(unitsRaw.String, "\n") {
		u = strings.TrimSpace(u)
		if u != "" && !registeredUnits[u] {
			unconfirmed = append(unconfirmed, u)
		}
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"list":              regs,
		"registrations":     regs,
		"unconfirmed":       unconfirmed,
		"unconfirmed_units": unconfirmed,
	})
}

// AdminDeleteMeetingRegistration 管理员移除单个参会人席位
func AdminDeleteMeetingRegistration(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var req struct {
		RegID int64 `json:"reg_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RegID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少席位记录ID"})
		return
	}
	res, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE id=? AND meeting_id=?", req.RegID, id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "席位记录不存在"})
		return
	}
	logOperation(r, "会务管理", "删除", "管理员移除会议参会人席位")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已移除该参会席位"})
}

// AdminChangeUnitToAbsent 管理员将整单位变更为请假不参加
func AdminChangeUnitToAbsent(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var req struct {
		Unit   string `json:"unit"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Unit) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少单位名称"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "因公请假（管理员协调代登）"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	// 清除该单位原有的所有记录（包括参会人员或旧不参加记录）
	_, err = tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "清理原报名失败"})
		return
	}

	// 插入一条整体不参加记录
	_, err = tx.Exec(`INSERT INTO meeting_registrations 
		(meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason, created_at)
		VALUES (?, ?, '', '', '', 1, ?, datetime('now','localtime'))`, id, req.Unit, reason)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "记录请假信息失败"})
		return
	}

	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交事务失败"})
		return
	}

	logOperation(r, "会务管理", "修改", fmt.Sprintf("管理员将【%s】变更为请假不参加：%s", req.Unit, reason))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已成功变更为请假不参加"})
}

// AdminResetUnitRegistration 管理员重置/撤销该单位的报名状态（转为未确认状态，允许重新报名）
func AdminResetUnitRegistration(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var req struct {
		Unit string `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Unit) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少单位名称"})
		return
	}
	res, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "撤销失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "该单位未找到报名记录"})
		return
	}
	logOperation(r, "会务管理", "修改", fmt.Sprintf("管理员重置【%s】报名状态为待确认", req.Unit))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已重置该单位报名状态，现已恢复为待确认"})
}

// isValidPhone 校验手机号：11 位、1 开头、第二位 3-9（与前端一致）
func isValidPhone(p string) bool {
	if len(p) != 11 {
		return false
	}
	if p[0] != '1' || p[1] < '3' || p[1] > '9' {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] < '0' || p[i] > '9' {
			return false
		}
	}
	return true
}

// maskPhone 手机号脱敏（匿名接口展示用）：138****0001
func maskPhone(p string) string {
	if len(p) == 11 {
		return p[:3] + "****" + p[7:]
	}
	if p == "" {
		return ""
	}
	return "****"
}
