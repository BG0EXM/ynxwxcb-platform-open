package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListMeetings 会议列表（管理员）
func ListMeetings(w http.ResponseWriter, r *http.Request) {
	query := `SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit,
			m.created_by, u.real_name, m.created_at, m.updated_at,
			COALESCE(SUM(CASE WHEN r.not_attend = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN r.not_attend = 1 THEN 1 ELSE 0 END), 0)
		FROM meetings m
		LEFT JOIN users u ON m.created_by = u.id
		LEFT JOIN meeting_registrations r ON r.meeting_id = m.id
		GROUP BY m.id ORDER BY m.meeting_date DESC, m.id DESC`
	rows, err := database.DB.Query(query)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.Meeting{}
	for rows.Next() {
		var m models.Meeting
		var creator sql.NullString
		var regCount, notAttend int
		rows.Scan(&m.ID, &m.Title, &m.MeetingDate, &m.MeetingTime, &m.Location, &m.Content, &m.Units, &m.UnitLimit,
			&m.CreatedBy, &creator, &m.CreatedAt, &m.UpdatedAt, &regCount, &notAttend)
		if creator.Valid {
			m.CreatedName = creator.String
		}
		m.RegCount = regCount
		m.NotAttend = notAttend
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
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
	// 显式指定 id = 当前最大 id + 1，删除会议后 ID 不跳号（复用）
	var nextID int64
	database.DB.QueryRow("SELECT COALESCE(MAX(id), 0) + 1 FROM meetings").Scan(&nextID)
	res, err := database.DB.Exec(
		`INSERT INTO meetings (id, title, meeting_date, meeting_time, location, content, units, unit_limit, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nextID, req.Title, req.MeetingDate, req.MeetingTime, req.Location, req.Content, req.Units, req.UnitLimit, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	lastID, _ := res.LastInsertId()
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
	_, err := database.DB.Exec(
		`UPDATE meetings SET title=?, meeting_date=?, meeting_time=?, location=?, content=?, units=?, unit_limit=?, updated_at=? WHERE id=?`,
		req.Title, req.MeetingDate, req.MeetingTime, req.Location, req.Content, req.Units, req.UnitLimit, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteMeeting 删除会议（管理员，级联删报名）
func DeleteMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=?", id)
	_, err := database.DB.Exec("DELETE FROM meetings WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	// 重置自增序列，使删除的会议 ID 可被复用（MAX(id)+1 连续不跳号）
	database.DB.Exec("DELETE FROM sqlite_sequence WHERE name='meetings'")
	database.DB.Exec("INSERT OR REPLACE INTO sqlite_sequence (name, seq) VALUES ('meetings', (SELECT COALESCE(MAX(id),0) FROM meetings))")
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
	var creator sql.NullString
	err := database.DB.QueryRow(
		`SELECT m.id, m.title, m.meeting_date, m.meeting_time, m.location, m.content, m.units, m.unit_limit,
			m.created_by, u.real_name, m.created_at, m.updated_at
		FROM meetings m LEFT JOIN users u ON m.created_by = u.id WHERE m.id = ?`, id).
		Scan(&m.ID, &m.Title, &m.MeetingDate, &m.MeetingTime, &m.Location, &m.Content, &m.Units, &m.UnitLimit,
			&m.CreatedBy, &creator, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if creator.Valid {
		m.CreatedName = creator.String
	}

	// 报名列表
	regRows, err := database.DB.Query(
		`SELECT id, meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason, created_at
		 FROM meeting_registrations WHERE meeting_id=? ORDER BY not_attend, id`, id)
	if err == nil {
		regs := []models.MeetingRegistration{}
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
			regs = append(regs, rg)
		}
		regRows.Close()
		m.RegCount = len(regs)
		middleware.JSON(w, http.StatusOK, map[string]interface{}{"meeting": m, "registrations": regs})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"meeting": m, "registrations": []models.MeetingRegistration{}})
}

// PublicMeeting 公开会议信息（匿名，供报名页加载）
func PublicMeeting(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}
	var m models.Meeting
	err := database.DB.QueryRow(
		`SELECT id, title, meeting_date, meeting_time, location, content, units, unit_limit FROM meetings WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &m.MeetingDate, &m.MeetingTime, &m.Location, &m.Content, &m.Units, &m.UnitLimit)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
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
	expired := false
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
			for rows.Next() {
				var rgID int64
				var name, title, phone, reason sql.NullString
				var na int
				rows.Scan(&rgID, &name, &title, &phone, &na, &reason)
				if na == 1 {
					notAttendAll = true
					notAttendReason = reason.String
					continue
				}
				regs = append(regs, map[string]interface{}{
					"id": rgID, "attendee_name": name.String, "attendee_title": title.String, "phone": phone.String,
				})
			}
			rows.Close()
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

// parseMeetingTime 解析会议开始时间（YYYY-MM-DD + HH:mm），返回时间与是否可解析
func parseMeetingTime(date, tm string) (time.Time, bool) {
	layout := "2006-01-02"
	full := date
	if tm != "" {
		if _, err := time.Parse("15:04", tm); err == nil {
			layout = "2006-01-02 15:04"
			full = date + " " + tm
		}
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
	var md, mt string
	var unitLimit int
	err := database.DB.QueryRow("SELECT meeting_date, meeting_time, unit_limit FROM meetings WHERE id=?", meetingID).
		Scan(&md, &mt, &unitLimit)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if t, ok := parseMeetingTime(md, mt); ok && time.Now().After(t) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议已开始，报名已截止"})
		return
	}
	// 校验该单位是否已整体不参加
	var notAll int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=1", meetingID, req.Unit).Scan(&notAll)

	if req.NotAttend == 1 {
		// 整体不参加：清掉该单位所有参加报名，插入一条不参加标记
		if req.Reason == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不参加请填写原因"})
			return
		}
		database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", meetingID, req.Unit)
		_, err = database.DB.Exec(
			`INSERT INTO meeting_registrations (meeting_id, unit, not_attend, reason) VALUES (?, ?, 1, ?)`,
			meetingID, req.Unit, req.Reason)
		if err != nil {
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
	if !isValidPhone(req.Phone) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请输入正确的11位手机号"})
		return
	}
	// 计算该单位当前已参加人数
	var attendCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit).Scan(&attendCount)
	// 若单位已整体不参加，切换为参加时先移除不参加标记
	if notAll > 0 {
		database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=1", meetingID, req.Unit)
		attendCount = 0
	}

	if req.RegID > 0 {
		// 修改已有报名
		_, err = database.DB.Exec(
			`UPDATE meeting_registrations SET attendee_name=?, attendee_title=?, phone=? WHERE id=? AND meeting_id=? AND unit=?`,
			req.AttendeeName, req.AttendeeTitle, req.Phone, req.RegID, meetingID, req.Unit)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
			return
		}
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
		return
	}
	// 新增人员：unit_limit 语义：1=单人替换；>1=多人上限；<=0=不限制
	if unitLimit == 1 && attendCount >= 1 {
		// 单人模式：替换（先删旧再加新）
		database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=? AND not_attend=0", meetingID, req.Unit)
	} else if unitLimit > 1 && attendCount >= unitLimit {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位报名人数已达上限"})
		return
	}
	_, err = database.DB.Exec(
		`INSERT INTO meeting_registrations (meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason) VALUES (?, ?, ?, ?, ?, 0, '')`,
		meetingID, req.Unit, req.AttendeeName, req.AttendeeTitle, req.Phone)
	if err != nil {
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
	var md, mt string
	err := database.DB.QueryRow("SELECT meeting_date, meeting_time FROM meetings WHERE id=?", id).Scan(&md, &mt)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "会议不存在"})
		return
	}
	if t, ok := parseMeetingTime(md, mt); ok && time.Now().After(t) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "会议已开始，报名已截止"})
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
	_, err := database.DB.Exec("DELETE FROM meeting_registrations WHERE meeting_id=? AND unit=?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "取消失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已取消报名"})
}

// ExportMeetingRegistration 导出会议签到单 Excel（管理员）
// 列出已报名人员（按单位排序），供线下签到使用
func ExportMeetingRegistration(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		http.Error(w, "缺少ID", http.StatusBadRequest)
		return
	}
	var m models.Meeting
	err := database.DB.QueryRow(
		`SELECT id, title, meeting_date, meeting_time, location FROM meetings WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &m.MeetingDate, &m.MeetingTime, &m.Location)
	if err != nil {
		http.Error(w, "会议不存在", http.StatusNotFound)
		return
	}
	regRows, err := database.DB.Query(
		`SELECT unit, attendee_name, attendee_title, phone, not_attend, reason
		 FROM meeting_registrations WHERE meeting_id=? AND not_attend=0 ORDER BY unit, id`, id)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer regRows.Close()

	headers := []string{"序号", "参会单位", "姓名", "职务", "电话", "签到", "备注"}
	data := [][]interface{}{}
	idx := 1
	for regRows.Next() {
		var unit, name, title, phone sql.NullString
		var notAttend int
		var reason sql.NullString
		regRows.Scan(&unit, &name, &title, &phone, &notAttend, &reason)
		data = append(data, []interface{}{
			idx, unit.String, name.String, title.String, phone.String, "", "",
		})
		idx++
	}
	if err := regRows.Err(); err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
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
	regRows, err := database.DB.Query(
		`SELECT id, meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason, created_at
		 FROM meeting_registrations WHERE meeting_id=? ORDER BY not_attend, id`, id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer regRows.Close()
	regs := []models.MeetingRegistration{}
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
		regs = append(regs, rg)
	}
	if err := regRows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": regs})
}

// isValidPhone 校验手机号：11 位、1 开头
func isValidPhone(p string) bool {
	if len(p) != 11 {
		return false
	}
	if p[0] != '1' {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] < '0' || p[i] > '9' {
			return false
		}
	}
	return true
}
