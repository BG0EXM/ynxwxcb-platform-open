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

// ListCalendarTasks 工作日历任务列表（按起止日期范围 + 可选部门）
func ListCalendarTasks(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	departmentID := r.URL.Query().Get("department_id")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if start != "" {
		where += ` AND t.end_date >= ?`
		args = append(args, start)
	}
	if end != "" {
		where += ` AND t.start_date <= ?`
		args = append(args, end)
	}
	if departmentID != "" {
		where += ` AND t.department_id = ?`
		args = append(args, departmentID)
	}

	query := `SELECT t.id, t.department_id, d.name, t.title, t.content, t.start_date, t.end_date,
			t.created_by, u.real_name, t.created_at, t.updated_at
		FROM calendar_tasks t
		LEFT JOIN departments d ON t.department_id = d.id
		LEFT JOIN users u ON t.created_by = u.id` + where + ` ORDER BY t.start_date, t.id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.CalendarTask{}
	for rows.Next() {
		var t models.CalendarTask
		var dept, creator, title, content, startDate, endDate sql.NullString
		var deptID, createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&t.ID, &deptID, &dept, &title, &content, &startDate, &endDate,
			&createdBy, &creator, &createdAt, &updatedAt); err != nil {
			continue
		}
		t.DepartmentID = deptID.Int64
		t.Department = dept.String
		t.Title = title.String
		t.Content = content.String
		t.StartDate = startDate.String
		t.EndDate = endDate.String
		t.CreatedBy = createdBy.Int64
		t.CreatedName = creator.String
		if createdAt.Valid {
			t.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			t.UpdatedAt = updatedAt.Time
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateCalendarTask 新增工作日历任务（各科室录自己的，管理员可任选科室）
func CreateCalendarTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.CalendarTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "工作内容必填"})
		return
	}
	// 非管理员强制本科室，防止把任务建到别的科室
	if roleCode != "admin" {
		var deptID sql.NullInt64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&deptID)
		if !deptID.Valid || deptID.Int64 == 0 {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "请先设置所属科室"})
			return
		}
		req.DepartmentID = deptID.Int64
	}
	if !isValidDate(req.StartDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "开始日期格式应为 YYYY-MM-DD"})
		return
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if !isValidDate(req.EndDate) || req.EndDate < req.StartDate {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "结束日期不合法"})
		return
	}
	_, err := database.DB.Exec(
		`INSERT INTO calendar_tasks (department_id, title, content, start_date, end_date, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		req.DepartmentID, req.Title, req.Content, req.StartDate, req.EndDate, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	logOperation(r, "工作日历", "新增", "新增工作：「"+req.Title+"」（"+req.StartDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateCalendarTask 修改工作日历任务（本科室或管理员）
func UpdateCalendarTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.CalendarTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "工作内容必填"})
		return
	}
	// 权限校验：管理员可改任意，普通用户只能改本科室的任务
	if roleCode != "admin" {
		if !sameDept("calendar_tasks", req.ID, userID) {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权修改其他科室的工作"})
			return
		}
		// 普通用户不能把任务改到其他科室
		var deptID sql.NullInt64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&deptID)
		req.DepartmentID = deptID.Int64
	}
	if !isValidDate(req.StartDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "开始日期格式应为 YYYY-MM-DD"})
		return
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	if !isValidDate(req.EndDate) || req.EndDate < req.StartDate {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "结束日期不合法"})
		return
	}
	_, err := database.DB.Exec(
		`UPDATE calendar_tasks SET department_id=?, title=?, content=?, start_date=?, end_date=?, updated_at=? WHERE id=?`,
		req.DepartmentID, req.Title, req.Content, req.StartDate, req.EndDate, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	logOperation(r, "工作日历", "修改", "修改工作：「"+req.Title+"」（"+req.StartDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteCalendarTask 删除工作日历任务（本科室或管理员）
func DeleteCalendarTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if roleCode != "admin" {
		if !sameDept("calendar_tasks", id, userID) {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权删除其他科室的工作"})
			return
		}
	}
	var title string
	database.DB.QueryRow("SELECT title FROM calendar_tasks WHERE id=?", id).Scan(&title)
	_, err := database.DB.Exec("DELETE FROM calendar_tasks WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	logOperation(r, "工作日历", "删除", "删除工作：「"+title+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportCalendarTasks 导出工作日历 Excel（可按部门与日期范围筛选，默认全部）
func ExportCalendarTasks(w http.ResponseWriter, r *http.Request) {
	departmentID := r.URL.Query().Get("department_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	query := `SELECT t.id, d.name, t.title, t.content, t.start_date, t.end_date, u.real_name, t.created_at
		FROM calendar_tasks t
		LEFT JOIN departments d ON t.department_id = d.id
		LEFT JOIN users u ON t.created_by = u.id WHERE 1=1`
	args := []interface{}{}
	if departmentID != "" {
		query += ` AND t.department_id = ?`
		args = append(args, departmentID)
	}
	if start != "" {
		query += ` AND t.end_date >= ?`
		args = append(args, start)
	}
	if end != "" {
		query += ` AND t.start_date <= ?`
		args = append(args, end)
	}
	query += ` ORDER BY t.start_date, t.id`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	headers := []string{"序号", "科室", "工作内容", "详情", "开始日期", "结束日期", "录入人", "录入时间"}
	data := [][]interface{}{}
	idx := 1
	for rows.Next() {
		var id int64
		var dept, title, content, startDate, endDate, creator sql.NullString
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &dept, &title, &content, &startDate, &endDate, &creator, &createdAt); err != nil {
			continue
		}
		data = append(data, []interface{}{
			idx, dept.String, title.String, content.String, formatDateStr(startDate.String),
			formatDateStr(endDate.String), creator.String, formatDateTime(createdAt.Time),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	logOperation(r, "工作日历", "导出", "导出工作日历")
	exportExcel(w, "工作日历", "工作日历.xlsx", headers, data)
}
