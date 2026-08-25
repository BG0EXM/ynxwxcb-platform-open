package handlers

import (
	"encoding/json"
	"net/http"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListContacts 通讯录（分页）
func ListContacts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	departmentID := r.URL.Query().Get("department_id")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if keyword != "" {
		where += ` AND (c.name LIKE ? OR c.position LIKE ? OR c.phone LIKE ?)`
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if departmentID != "" {
		where += ` AND c.department_id = ?`
		args = append(args, departmentID)
	}

	p := parsePage(r)

	var total int
	database.DB.QueryRow("SELECT COUNT(*) FROM contacts c"+where, args...).Scan(&total)

	query := `SELECT c.id, c.name, c.phone, c.department_id, d.name, c.position, c.is_public, c.sort
		FROM contacts c LEFT JOIN departments d ON c.department_id = d.id` + where +
		` ORDER BY c.sort, c.id LIMIT ? OFFSET ?`
	args = append(args, p.PageSize, (p.Page-1)*p.PageSize)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	contacts := []models.Contact{}
	for rows.Next() {
		var c models.Contact
		rows.Scan(&c.ID, &c.Name, &c.Phone, &c.DepartmentID, &c.Department, &c.Position, &c.IsPublic, &c.Sort)
		contacts = append(contacts, c)
	}
	middleware.JSON(w, http.StatusOK, paginateResult(contacts, total, p.Page, p.PageSize))
}

// CreateContact 添加通讯录
func CreateContact(w http.ResponseWriter, r *http.Request) {
	var req models.Contact
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Name == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "姓名必填"})
		return
	}
	_, err := database.DB.Exec(
		"INSERT INTO contacts (name, phone, department_id, position, is_public, sort) VALUES (?, ?, ?, ?, ?, ?)",
		req.Name, req.Phone, req.DepartmentID, req.Position, req.IsPublic, req.Sort)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateContact 更新通讯录
func UpdateContact(w http.ResponseWriter, r *http.Request) {
	var req models.Contact
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	_, err := database.DB.Exec(
		"UPDATE contacts SET name=?, phone=?, department_id=?, position=?, is_public=?, sort=? WHERE id=?",
		req.Name, req.Phone, req.DepartmentID, req.Position, req.IsPublic, req.Sort, req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteContact 删除通讯录
func DeleteContact(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	_, err := database.DB.Exec("DELETE FROM contacts WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ListDutySchedules 排班列表
func ListDutySchedules(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month") // YYYY-MM
	userID := r.URL.Query().Get("user_id")

	query := `SELECT s.id, s.duty_date, s.user_id, u.real_name, s.is_dawangyuan, s.note, s.status
		FROM duty_schedules s LEFT JOIN users u ON s.user_id = u.id WHERE 1=1`
	args := []interface{}{}
	if month != "" {
		query += ` AND s.duty_date LIKE ?`
		args = append(args, month+"%")
	}
	if userID != "" {
		query += ` AND s.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY s.duty_date`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	schedules := []models.DutySchedule{}
	for rows.Next() {
		var s models.DutySchedule
		rows.Scan(&s.ID, &s.DutyDate, &s.UserID, &s.UserName, &s.IsDaWangYuan, &s.Note, &s.Status)
		schedules = append(schedules, s)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": schedules})
}

// SaveDutySchedule 保存排班（按日期+人员 upsert，支持一天多人）
func SaveDutySchedule(w http.ResponseWriter, r *http.Request) {
	var req models.DutySchedule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.DutyDate == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期必填"})
		return
	}
	if req.UserID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择值守人员"})
		return
	}
	_, err := database.DB.Exec(
		`INSERT INTO duty_schedules (duty_date, user_id, is_dawangyuan, note, status) VALUES (?, ?, ?, ?, 1)
		 ON CONFLICT(duty_date, user_id) DO UPDATE SET is_dawangyuan=?, note=?`,
		req.DutyDate, req.UserID, req.IsDaWangYuan, req.Note,
		req.IsDaWangYuan, req.Note)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "保存成功"})
}

// DeleteDutySchedule 删除排班
func DeleteDutySchedule(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	_, err := database.DB.Exec("DELETE FROM duty_schedules WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}
