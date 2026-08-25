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
		var dept, creator sql.NullString
		var createdAt, updatedAt time.Time
		rows.Scan(&t.ID, &t.DepartmentID, &dept, &t.Title, &t.Content, &t.StartDate, &t.EndDate,
			&t.CreatedBy, &creator, &createdAt, &updatedAt)
		if dept.Valid {
			t.Department = dept.String
		}
		if creator.Valid {
			t.CreatedName = creator.String
		}
		t.CreatedAt = createdAt
		t.UpdatedAt = updatedAt
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateCalendarTask 新增工作日历任务（权限全开：办公室与各科室均可录入）
func CreateCalendarTask(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.CalendarTask
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "工作内容必填"})
		return
	}
	if req.StartDate == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "开始日期必填"})
		return
	}
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	_, err := database.DB.Exec(
		`INSERT INTO calendar_tasks (department_id, title, content, start_date, end_date, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		req.DepartmentID, req.Title, req.Content, req.StartDate, req.EndDate, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateCalendarTask 修改工作日历任务（权限全开）
func UpdateCalendarTask(w http.ResponseWriter, r *http.Request) {
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
	if req.EndDate == "" {
		req.EndDate = req.StartDate
	}
	_, err := database.DB.Exec(
		`UPDATE calendar_tasks SET department_id=?, title=?, content=?, start_date=?, end_date=?, updated_at=? WHERE id=?`,
		req.DepartmentID, req.Title, req.Content, req.StartDate, req.EndDate, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteCalendarTask 删除工作日历任务（权限全开）
func DeleteCalendarTask(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	_, err := database.DB.Exec("DELETE FROM calendar_tasks WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportCalendarTasks 导出工作日历 Excel（可按部门筛选，默认全部）
func ExportCalendarTasks(w http.ResponseWriter, r *http.Request) {
	departmentID := r.URL.Query().Get("department_id")

	query := `SELECT t.id, d.name, t.title, t.content, t.start_date, t.end_date, u.real_name, t.created_at
		FROM calendar_tasks t
		LEFT JOIN departments d ON t.department_id = d.id
		LEFT JOIN users u ON t.created_by = u.id WHERE 1=1`
	args := []interface{}{}
	if departmentID != "" {
		query += ` AND t.department_id = ?`
		args = append(args, departmentID)
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
		var createdAt time.Time
		rows.Scan(&id, &dept, &title, &content, &startDate, &endDate, &creator, &createdAt)
		data = append(data, []interface{}{
			idx, dept.String, title.String, content.String, startDate.String,
			endDate.String, creator.String, createdAt.Format("2006-01-02 15:04"),
		})
		idx++
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	exportExcel(w, "工作日历", "工作日历.xlsx", headers, data)
}
