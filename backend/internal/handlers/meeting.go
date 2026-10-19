package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
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
	// 每页最多 100 条（与前端分页选项一致），防止一次请求拉取全表
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM meetings").Scan(&total); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	query := `SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit, IFNULL(m.public_token, ''),
			m.created_by, u.real_name, m.created_at, m.updated_at,
			COALESCE(SUM(CASE WHEN r.not_attend = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN r.not_attend = 1 THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN r.id IS NOT NULL AND TRIM(r.unit) != '' THEN TRIM(r.unit) END) as confirmed_units
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
		var regCount, notAttend, confirmedUnits int
		if err := rows.Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &content, &units, &m.UnitLimit, &m.PublicToken,
			&createdBy, &creator, &createdAt, &updatedAt, &regCount, &notAttend, &confirmedUnits); err != nil {
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
		m.ConfirmedUnits = confirmedUnits
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "total": total, "summary": meetingSummary()})
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
	// 会议开始时间即报名截止时间：格式非法会导致截止时间被误判为当天 23:59，必须拦截
	if req.MeetingTime = strings.TrimSpace(req.MeetingTime); req.MeetingTime != "" {
		if _, err := time.Parse("15:04", req.MeetingTime); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议时间格式应为 HH:mm（如 09:30）"})
			return
		}
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
		`INSERT INTO meetings (id, title, meeting_date, meeting_time, location, content, units, unit_limit, created_by, public_token)
		 SELECT COALESCE(MAX(id), 0) + 1, ?, ?, ?, ?, ?, ?, ?, ?, ? FROM meetings`,
		req.Title, req.MeetingDate, req.MeetingTime, req.Location, req.Content, req.Units, req.UnitLimit, userID, newPublicToken())
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
	// 会议开始时间即报名截止时间：格式非法会导致截止时间被误判为当天 23:59，必须拦截
	if req.MeetingTime = strings.TrimSpace(req.MeetingTime); req.MeetingTime != "" {
		if _, err := time.Parse("15:04", req.MeetingTime); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议时间格式应为 HH:mm（如 09:30）"})
			return
		}
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
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM meeting_unit_tokens WHERE meeting_id=?", id); err != nil {
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
		`SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit, IFNULL(m.public_token, ''),
			m.created_by, u.real_name, m.created_at, m.updated_at
		FROM meetings m LEFT JOIN users u ON m.created_by = u.id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.Title, &meetingDate, &meetingTime, &location, &content, &units, &m.UnitLimit, &m.PublicToken,
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
			cleanUnit := strings.TrimSpace(unit.String)
			rg.Unit = cleanUnit
			rg.AttendeeName = strings.TrimSpace(name.String)
			rg.AttendeeTitle = strings.TrimSpace(title.String)
			rg.Phone = strings.TrimSpace(phone.String)
			rg.Reason = strings.TrimSpace(reason.String)
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
			if cleanUnit != "" {
				registeredUnits[cleanUnit] = true
			}
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
	id, ok := publicID(w, r, "meetings")
	if !ok {
		return
	}
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
			if err := rows.Err(); err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
				return
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
	id, ok := publicID(w, r, "meetings")
	if !ok {
		return
	}
	var req struct {
		MeetingID     int64  `json:"meeting_id"`
		RegID         int64  `json:"reg_id"`
		Unit          string `json:"unit"`
		AttendeeName  string `json:"attendee_name"`
		AttendeeTitle string `json:"attendee_title"`
		Phone         string `json:"phone"`
		NotAttend     int    `json:"not_attend"`
		Reason        string `json:"reason"`
		EditToken     string `json:"edit_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	// 会议 ID 只认公开链接令牌解析结果，严禁请求体自带 meeting_id 绕过令牌
	meetingID := id
	if meetingID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	req.Unit = strings.TrimSpace(req.Unit)
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
	var attendCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit).Scan(&attendCount)

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
		newToken, msg := meetingUnitEditAuth(tx, meetingID, req.Unit, req.EditToken)
		if msg != "" {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg, "code": "MEETING_EDIT_TOKEN_REQUIRED"})
			return
		}
		// 事务内复核：防止并发时有人刚报名参会人员又被“不参加”整单位清空
		if !meetingCountTx(tx, w, "SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit, func(n int) bool { return n == 0 },
			"该单位已报名参会人员，如需变更为不参加请联系会务组管理员") {
			return
		}
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
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "已确认不参加", "edit_token": newToken})
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
		// 修改已有报名（手机号留空则保留原号码），必须持有本单位首次报名时下发的编辑凭证
		tx, err := database.DB.Begin()
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
			return
		}
		defer tx.Rollback()
		if _, msg := meetingUnitEditAuth(tx, meetingID, req.Unit, req.EditToken); msg != "" {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg, "code": "MEETING_EDIT_TOKEN_REQUIRED"})
			return
		}
		var res sql.Result
		if keepPhone {
			res, err = tx.Exec(
				`UPDATE meeting_registrations SET attendee_name=?, attendee_title=? WHERE id=? AND meeting_id=? AND unit=?`,
				req.AttendeeName, req.AttendeeTitle, req.RegID, meetingID, req.Unit)
		} else {
			res, err = tx.Exec(
				`UPDATE meeting_registrations SET attendee_name=?, attendee_title=?, phone=? WHERE id=? AND meeting_id=? AND unit=?`,
				req.AttendeeName, req.AttendeeTitle, req.Phone, req.RegID, meetingID, req.Unit)
		}
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "报名记录不存在"})
			return
		}
		if err := tx.Commit(); err != nil {
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

	// 本单位已有报名（或已领取编辑凭证）时，继续增补/替换人员必须持有编辑凭证；首次报名则签发新凭证
	newToken, msg := meetingUnitEditAuth(tx, meetingID, req.Unit, req.EditToken)
	if msg != "" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg, "code": "MEETING_EDIT_TOKEN_REQUIRED"})
		return
	}

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

	// 改为参加：清除本单位原“不参加”记录（事务内执行，无需事务外预读）
	if _, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=1", meetingID, req.Unit); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "报名失败"})
		return
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
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "报名成功", "edit_token": newToken})
}

// PublicRemoveAttendee 移除某条参会报名（匿名，仅同单位可操作）
// body: {"reg_id":123}
func PublicRemoveAttendee(w http.ResponseWriter, r *http.Request) {
	id, ok := publicID(w, r, "meetings")
	if !ok {
		return
	}
	var req struct {
		RegID     int64  `json:"reg_id"`
		Unit      string `json:"unit"`
		EditToken string `json:"edit_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	req.Unit = strings.TrimSpace(req.Unit)
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

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	defer tx.Rollback()
	if _, msg := meetingUnitEditAuth(tx, id, req.Unit, req.EditToken); msg != "" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg, "code": "MEETING_EDIT_TOKEN_REQUIRED"})
		return
	}
	// 事务内复核：并发移除时不得把单位人数减到 0
	if !meetingCountTx(tx, w, "SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", id, req.Unit, func(n int) bool { return n > 1 },
		"该单位仅余一名参会人员，不可直接移除；如需更换人员请直接点击修改，如需取消参会请联系会务组管理员") {
		return
	}
	res, err := tx.Exec("DELETE FROM meeting_registrations WHERE id=? AND meeting_id=? AND unit=? AND not_attend=0", req.RegID, id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "记录不存在"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已移除"})
}

// PublicCancelMeetingRegister 取消报名（匿名，反悔机会）
// body: {"unit":"xx"}
func PublicCancelMeetingRegister(w http.ResponseWriter, r *http.Request) {
	id, ok := publicID(w, r, "meetings")
	if !ok {
		return
	}
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	var req struct {
		Unit      string `json:"unit"`
		EditToken string `json:"edit_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	req.Unit = strings.TrimSpace(req.Unit)
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

	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "取消失败"})
		return
	}
	defer tx.Rollback()
	if _, msg := meetingUnitEditAuth(tx, id, req.Unit, req.EditToken); msg != "" {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg, "code": "MEETING_EDIT_TOKEN_REQUIRED"})
		return
	}
	if !meetingCountTx(tx, w, "SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", id, req.Unit, func(n int) bool { return n == 0 },
		"该单位已提交参会人员名单，如需全员取消请联系会务组管理员") {
		return
	}
	res, err := tx.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "取消失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "该单位没有报名记录"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "取消失败"})
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
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
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
			cleanUnit := strings.TrimSpace(unit.String)
			regMap[cleanUnit] = append(regMap[cleanUnit], regInfo{
				name:      strings.TrimSpace(name.String),
				title:     strings.TrimSpace(title.String),
				phone:     strings.TrimSpace(phone.String),
				notAttend: na,
				reason:    strings.TrimSpace(reason.String),
			})
		}
		if err := regRows.Err(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
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
		cleanUnit := strings.TrimSpace(unit.String)
		rg.Unit = cleanUnit
		rg.AttendeeName = strings.TrimSpace(name.String)
		rg.AttendeeTitle = strings.TrimSpace(title.String)
		rg.Phone = strings.TrimSpace(phone.String)
		rg.Reason = strings.TrimSpace(reason.String)
		if rg.NotAttend == 0 {
			rg.IsAttending = 1
		} else {
			rg.IsAttending = 0
		}
		regs = append(regs, rg)
		if cleanUnit != "" {
			registeredUnits[cleanUnit] = true
		}
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
	releaseMeetingUnitTokenIfEmpty(id)
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
	req.Unit = strings.TrimSpace(req.Unit)
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
		VALUES (?, ?, '', '', '', 1, ?, CURRENT_TIMESTAMP)`, id, req.Unit, reason)
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
	req.Unit = strings.TrimSpace(req.Unit)
	res, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "撤销失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "该单位未找到报名记录"})
		return
	}
	// 管理员重置后释放该单位编辑凭证，允许单位重新首次报名
	database.DB.Exec("DELETE FROM meeting_unit_tokens WHERE meeting_id=? AND unit=?", id, req.Unit)
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

// meetingUnitEditAuth 公开报名的单位编辑凭证校验（须在写事务内调用）：
//   - 本单位已领取凭证：必须提交匹配的凭证；
//   - 本单位无凭证但已有报名记录（升级前的历史报名）：公开端不得修改，须联系管理员；
//   - 首次报名：签发新凭证并写入，返回给前端保存在报名人浏览器中。
//
// 返回新签发的凭证（非首次为空）与拒绝原因（放行时为空）。
func meetingUnitEditAuth(tx *sql.Tx, meetingID int64, unit, provided string) (string, string) {
	var hash string
	err := tx.QueryRow("SELECT token_hash FROM meeting_unit_tokens WHERE meeting_id=? AND unit=?", meetingID, unit).Scan(&hash)
	if err == nil {
		sum := sha256.Sum256([]byte(strings.TrimSpace(provided)))
		if provided == "" || subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(hash)) != 1 {
			return "", "本单位已完成报名，仅限首次报名的手机或电脑修改；如需调整请联系会务组管理员"
		}
		return "", ""
	}
	if err != sql.ErrNoRows {
		return "", "系统繁忙，请稍后重试"
	}
	var existing int
	if err := tx.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=?", meetingID, unit).Scan(&existing); err != nil {
		return "", "系统繁忙，请稍后重试"
	}
	if existing > 0 {
		return "", "本单位已完成报名，如需调整请联系会务组管理员"
	}
	token := newPublicToken()
	sum := sha256.Sum256([]byte(token))
	if _, err := tx.Exec("INSERT INTO meeting_unit_tokens (meeting_id, unit, token_hash) VALUES (?, ?, ?)", meetingID, unit, hex.EncodeToString(sum[:])); err != nil {
		return "", "系统繁忙，请稍后重试"
	}
	return token, ""
}

// releaseMeetingUnitTokenIfEmpty 管理员移除席位后，若某单位已无任何报名记录则释放其编辑凭证
func releaseMeetingUnitTokenIfEmpty(meetingID int64) {
	database.DB.Exec(`DELETE FROM meeting_unit_tokens WHERE meeting_id=? AND NOT EXISTS (
		SELECT 1 FROM meeting_registrations r WHERE r.meeting_id=meeting_unit_tokens.meeting_id AND r.unit=meeting_unit_tokens.unit)`, meetingID)
}

// meetingCountTx 在事务内统计并校验人数条件；不满足时写出 400（查询失败写出 500）并返回 false
func meetingCountTx(tx *sql.Tx, w http.ResponseWriter, query string, meetingID int64, unit string, ok func(int) bool, failMsg string) bool {
	var n int
	if err := tx.QueryRow(query, meetingID, unit).Scan(&n); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统繁忙，请稍后重试"})
		return false
	}
	if !ok(n) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": failMsg})
		return false
	}
	return true
}
