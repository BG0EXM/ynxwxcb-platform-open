package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListWeeklySummaries 每周工作总结列表
// 各科室只能看到自己科室，管理员看全部
func ListWeeklySummaries(w http.ResponseWriter, r *http.Request) {
	weekStart := r.URL.Query().Get("week_start")
	weekEnd := r.URL.Query().Get("week_end")
	departmentID := r.URL.Query().Get("department_id")
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	where := ` WHERE 1=1`
	args := []interface{}{}
	if weekStart != "" {
		where += ` AND s.week_end >= ?`
		args = append(args, weekStart)
	}
	if weekEnd != "" {
		where += ` AND s.week_start <= ?`
		args = append(args, weekEnd)
	}
	if departmentID != "" {
		where += ` AND s.department_id = ?`
		args = append(args, departmentID)
	}
	if roleCode != "admin" {
		where += ` AND s.department_id = (SELECT department_id FROM users WHERE id = ?)`
		args = append(args, userID)
	}

	query := `SELECT s.id, s.department_id, d.name, s.week_start, s.week_end, s.content,
			s.created_by, u.real_name, s.created_at, s.updated_at
		FROM weekly_summaries s
		LEFT JOIN departments d ON s.department_id = d.id
		LEFT JOIN users u ON s.created_by = u.id` + where + ` ORDER BY s.week_start DESC, s.department_id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.WeeklySummary{}
	for rows.Next() {
		var s models.WeeklySummary
		var dept, creator, weekStart, weekEnd, content sql.NullString
		var deptID, createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&s.ID, &deptID, &dept, &weekStart, &weekEnd, &content,
			&createdBy, &creator, &createdAt, &updatedAt); err != nil {
			continue
		}
		s.DepartmentID = deptID.Int64
		s.Department = dept.String
		s.WeekStart = weekStart.String
		s.WeekEnd = weekEnd.String
		s.Content = content.String
		s.CreatedBy = createdBy.Int64
		s.CreatedName = creator.String
		if createdAt.Valid {
			s.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			s.UpdatedAt = updatedAt.Time
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateWeeklySummary 新增每周工作总结（各科室录自己的）
func CreateWeeklySummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.WeeklySummary
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Content == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "总结内容必填"})
		return
	}
	if !isValidDate(req.WeekStart) || !isValidDate(req.WeekEnd) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	if req.WeekEnd < req.WeekStart {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "结束日期不能早于开始日期"})
		return
	}
	if roleCode != "admin" {
		var deptID int64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&deptID)
		if deptID == 0 {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "请先设置所属科室"})
			return
		}
		req.DepartmentID = deptID
	}
	_, err := database.DB.Exec(
		`INSERT INTO weekly_summaries (department_id, week_start, week_end, content, created_by) VALUES (?, ?, ?, ?, ?)`,
		req.DepartmentID, req.WeekStart, req.WeekEnd, req.Content, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	logOperation(r, "每周工作总结", "新增", "录入每周工作总结（"+req.WeekStart+" 至 "+req.WeekEnd+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateWeeklySummary 修改每周工作总结（本科室或管理员）
func UpdateWeeklySummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.WeeklySummary
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.Content == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "总结内容必填"})
		return
	}
	if !isValidDate(req.WeekStart) || !isValidDate(req.WeekEnd) || req.WeekEnd < req.WeekStart {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期范围不合法"})
		return
	}
	if roleCode != "admin" {
		if !sameDept("weekly_summaries", req.ID, userID) {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权修改其他科室的记录"})
			return
		}
	}
	// 管理员可调整所属科室（传 0 时不改）
	if roleCode == "admin" && req.DepartmentID != 0 {
		if _, err := database.DB.Exec(
			`UPDATE weekly_summaries SET week_start=?, week_end=?, content=?, department_id=?, updated_at=? WHERE id=?`,
			req.WeekStart, req.WeekEnd, req.Content, req.DepartmentID, time.Now(), req.ID); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
			return
		}
		logOperation(r, "每周工作总结", "修改", "修改每周工作总结（"+req.WeekStart+" 至 "+req.WeekEnd+"）")
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
		return
	}
	if _, err := database.DB.Exec(
		`UPDATE weekly_summaries SET week_start=?, week_end=?, content=?, updated_at=? WHERE id=?`,
		req.WeekStart, req.WeekEnd, req.Content, time.Now(), req.ID); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	logOperation(r, "每周工作总结", "修改", "修改每周工作总结（"+req.WeekStart+" 至 "+req.WeekEnd+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteWeeklySummary 删除每周工作总结（本科室或管理员）
func DeleteWeeklySummary(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if roleCode != "admin" {
		if !sameDept("weekly_summaries", id, userID) {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权删除其他科室的记录"})
			return
		}
	}
	var ws, we string
	database.DB.QueryRow("SELECT week_start, week_end FROM weekly_summaries WHERE id=?", id).Scan(&ws, &we)
	_, err := database.DB.Exec("DELETE FROM weekly_summaries WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	logOperation(r, "每周工作总结", "删除", "删除每周工作总结（"+ws+" 至 "+we+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportWeeklySummaries 导出每周工作总结 Word（仅管理员）
// 抬头单位名称 + 周数（X月X日-X月X日）+ 各科室依次列出
func ExportWeeklySummaries(w http.ResponseWriter, r *http.Request) {
	weekStart := r.URL.Query().Get("week_start")
	weekEnd := r.URL.Query().Get("week_end")

	query := `SELECT s.department_id, d.name, s.week_start, s.week_end, s.content
		FROM weekly_summaries s
		LEFT JOIN departments d ON s.department_id = d.id WHERE 1=1`
	args := []interface{}{}
	if weekStart != "" {
		query += ` AND s.week_end >= ?`
		args = append(args, weekStart)
	}
	if weekEnd != "" {
		query += ` AND s.week_start <= ?`
		args = append(args, weekEnd)
	}
	query += ` ORDER BY s.department_id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type summaryItem struct {
		content string
	}
	grouped := map[string][]summaryItem{}
	var deptOrder []string
	for rows.Next() {
		var deptID int64
		var dept, content sql.NullString
		rows.Scan(&deptID, &dept, new(sql.NullString), new(sql.NullString), &content)
		name := dept.String
		if name == "" {
			name = fmt.Sprintf("科室%d", deptID)
		}
		if _, ok := grouped[name]; !ok {
			deptOrder = append(deptOrder, name)
		}
		grouped[name] = append(grouped[name], summaryItem{content: content.String})
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	sort.Strings(deptOrder)

	// 周数显示：X月X日-X月X日
	weekLabel := ""
	if weekStart != "" && weekEnd != "" {
		ws := parseShortDate(weekStart)
		we := parseShortDate(weekEnd)
		if ws != "" && we != "" {
			weekLabel = ws + " - " + we
		}
	}

	var builder docxBuilder
	builder.addTitle("中共伊宁县委宣传部")
	builder.addSubTitle(weekLabel + " 各科室每周工作总结")
	builder.addEmpty()
	for _, dept := range deptOrder {
		builder.addBold(dept)
		for _, it := range grouped[dept] {
			builder.add("　　" + it.content)
		}
		builder.addEmpty()
	}

	data, err := builder.build()
	if err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
		return
	}
	logOperation(r, "每周工作总结", "导出", "导出每周工作总结（"+weekLabel+"）")
	fileName := "每周工作总结-" + weekLabel + ".docx"
	writeDocx(w, fileName, data)
}

// parseShortDate 把 YYYY-MM-DD 转成 X月X日
func parseShortDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d月%d日", int(t.Month()), t.Day())
}
