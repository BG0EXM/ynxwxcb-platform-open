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

// ListMajorEvents 大事记列表（各科室按月记录的重大事项，精确到日）
// 各科室只能看到自己科室的记录，管理员看全部
func ListMajorEvents(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month") // YYYY-MM
	departmentID := r.URL.Query().Get("department_id")
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	where := ` WHERE 1=1`
	args := []interface{}{}
	if year != "" {
		where += ` AND e.period LIKE ?`
		args = append(args, year+"%")
	}
	if month != "" {
		where += ` AND e.period LIKE ?`
		args = append(args, month+"%")
	}
	if departmentID != "" {
		where += ` AND e.department_id = ?`
		args = append(args, departmentID)
	}
	// 各科室只能看自己科室
	if roleCode != "admin" {
		where += ` AND e.department_id = (SELECT department_id FROM users WHERE id = ?)`
		args = append(args, userID)
	}

	query := `SELECT e.id, e.department_id, d.name, e.event_type, e.period, e.title, e.content,
			e.created_by, u.real_name, e.created_at, e.updated_at
		FROM major_events e
		LEFT JOIN departments d ON e.department_id = d.id
		LEFT JOIN users u ON e.created_by = u.id` + where + ` ORDER BY e.period DESC, e.department_id, e.id DESC`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.MajorEvent{}
	for rows.Next() {
		var e models.MajorEvent
		var dept, creator sql.NullString
		var createdAt, updatedAt time.Time
		rows.Scan(&e.ID, &e.DepartmentID, &dept, &e.EventType, &e.Period, &e.Title, &e.Content,
			&e.CreatedBy, &creator, &createdAt, &updatedAt)
		if dept.Valid {
			e.Department = dept.String
		}
		if creator.Valid {
			e.CreatedName = creator.String
		}
		e.CreatedAt = createdAt
		e.UpdatedAt = updatedAt
		list = append(list, e)
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateMajorEvent 新增大事记（各科室录自己的）
func CreateMajorEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.MajorEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "事项必填"})
		return
	}
	if req.Period == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择周期"})
		return
	}
	// 非管理员只能录自己科室
	if roleCode != "admin" {
		var deptID int64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&deptID)
		if deptID == 0 {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "请先设置所属科室"})
			return
		}
		req.DepartmentID = deptID
	}
	if req.EventType == "" {
		req.EventType = "monthly"
	}
	// V1.3.6.1：大事记仅按月录入，年度类型不再使用
	req.EventType = "monthly"
	_, err := database.DB.Exec(
		`INSERT INTO major_events (department_id, event_type, period, title, content, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		req.DepartmentID, req.EventType, req.Period, req.Title, req.Content, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateMajorEvent 修改大事记（本科室或管理员）
func UpdateMajorEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var req models.MajorEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "事项必填"})
		return
	}
	// 权限校验：管理员可改任意，普通用户只能改自己科室的记录
	if roleCode != "admin" {
		var ownerDept int64
		err := database.DB.QueryRow("SELECT department_id FROM major_events WHERE id = ?", req.ID).Scan(&ownerDept)
		if err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "记录不存在"})
			return
		}
		var myDept int64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&myDept)
		if ownerDept != myDept {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权修改其他科室的记录"})
			return
		}
	}
	_, err := database.DB.Exec(
		`UPDATE major_events SET event_type=?, period=?, title=?, content=?, updated_at=? WHERE id=?`,
		req.EventType, req.Period, req.Title, req.Content, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteMajorEvent 删除大事记（本科室或管理员）
func DeleteMajorEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if roleCode != "admin" {
		var ownerDept int64
		err := database.DB.QueryRow("SELECT department_id FROM major_events WHERE id = ?", id).Scan(&ownerDept)
		if err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "记录不存在"})
			return
		}
		var myDept int64
		database.DB.QueryRow("SELECT department_id FROM users WHERE id = ?", userID).Scan(&myDept)
		if ownerDept != myDept {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权删除其他科室的记录"})
			return
		}
	}
	_, err := database.DB.Exec("DELETE FROM major_events WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportMajorEvents 导出大事记 Word（仅管理员）
// 按年汇总整个宣传部：抬头单位名称 + 年份，全部记录按年月日顺序排列（不分科室、不带科室名）
func ExportMajorEvents(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	if year == "" {
		year = time.Now().Format("2006")
	}

	query := `SELECT e.period, e.title
		FROM major_events e
		WHERE e.period LIKE ? ORDER BY e.period`
	rows, err := database.DB.Query(query, year+"%")
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// 全部记录按日期收集（period 为 YYYY-MM-DD，精确到日）
	type eventItem struct {
		period string
		month  int
		day    string
		title  string
	}
	list := []eventItem{}
	for rows.Next() {
		var period, title sql.NullString
		rows.Scan(&period, &title)
		it := eventItem{period: period.String, title: title.String}
		// 从 YYYY-MM-DD 解析月份序号与日号用于排序/显示
		if len(it.period) == 10 {
			var y, m, d int
			fmt.Sscanf(it.period, "%d-%d-%d", &y, &m, &d)
			it.month = m
			it.day = fmt.Sprintf("%d月%d日", m, d)
		} else if len(it.period) == 7 {
			fmt.Sscanf(it.period, "%d-%d", new(int), &it.month)
			it.day = it.period
		}
		list = append(list, it)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].period < list[j].period
	})

	var builder docxBuilder
	builder.addTitle("中共伊宁县委宣传部")
	builder.addSubTitle(year + "年大事记")
	builder.addEmpty()
	var curMonth int
	for _, it := range list {
		if it.month != curMonth {
			curMonth = it.month
			builder.addBold(fmt.Sprintf("%d月", curMonth))
		}
		line := fmt.Sprintf("　　%s　%s", it.day, it.title)
		builder.add(line)
	}

	data, err := builder.build()
	if err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
		return
	}
	writeDocx(w, year+"年宣传部大事记.docx", data)
}
