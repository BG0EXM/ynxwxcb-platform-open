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

// ListStandingCommitteeEvents 常委大事记列表（按年/月展示，仅管理员）
func ListStandingCommitteeEvents(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if year != "" {
		where += ` AND e.event_date LIKE ?`
		args = append(args, year+"%")
	}
	if month != "" {
		where += ` AND e.event_date LIKE ?`
		args = append(args, month+"%")
	}

	query := `SELECT e.id, e.event_date, e.title, e.created_by,
			u.real_name, e.created_at, e.updated_at
		FROM standing_committee_events e
		LEFT JOIN users u ON e.created_by = u.id` + where + ` ORDER BY e.event_date DESC, e.id DESC`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.StandingCommitteeEvent{}
	for rows.Next() {
		var e models.StandingCommitteeEvent
		var creator, eventDate, title sql.NullString
		var createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&e.ID, &eventDate, &title, &createdBy,
			&creator, &createdAt, &updatedAt); err != nil {
			continue
		}
		e.EventDate = eventDate.String
		e.Title = title.String
		e.CreatedBy = createdBy.Int64
		e.CreatedName = creator.String
		if createdAt.Valid {
			e.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			e.UpdatedAt = updatedAt.Time
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateStandingCommitteeEvent 新增常委大事记（仅管理员）
func CreateStandingCommitteeEvent(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.StandingCommitteeEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "标题必填"})
		return
	}
	if req.EventDate == "" {
		req.EventDate = time.Now().Format("2006-01-02")
	}
	if !isValidDate(req.EventDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	_, err := database.DB.Exec(
		`INSERT INTO standing_committee_events (event_date, title, created_by) VALUES (?, ?, ?)`,
		req.EventDate, req.Title, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	logOperation(r, "常委管理", "新增", "录入常委大事记：「"+req.Title+"」（"+req.EventDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateStandingCommitteeEvent 修改常委大事记（仅管理员）
func UpdateStandingCommitteeEvent(w http.ResponseWriter, r *http.Request) {
	var req models.StandingCommitteeEvent
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "标题必填"})
		return
	}
	if req.EventDate == "" {
		req.EventDate = time.Now().Format("2006-01-02")
	}
	if !isValidDate(req.EventDate) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	_, err := database.DB.Exec(
		`UPDATE standing_committee_events SET event_date=?, title=?, updated_at=? WHERE id=?`,
		req.EventDate, req.Title, time.Now(), req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
		return
	}
	logOperation(r, "常委管理", "修改", "修改常委大事记：「"+req.Title+"」（"+req.EventDate+"）")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

// DeleteStandingCommitteeEvent 删除常委大事记（仅管理员）
func DeleteStandingCommitteeEvent(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var title string
	database.DB.QueryRow("SELECT title FROM standing_committee_events WHERE id=?", id).Scan(&title)
	_, err := database.DB.Exec("DELETE FROM standing_committee_events WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	logOperation(r, "常委管理", "删除", "删除常委大事记：「"+title+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// ExportStandingCommitteeEvents 导出常委大事记 Word（仅管理员）
func ExportStandingCommitteeEvents(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")

	where := ` WHERE 1=1`
	args := []interface{}{}
	if year != "" {
		where += ` AND e.event_date LIKE ?`
		args = append(args, year+"%")
	}
	if month != "" {
		where += ` AND e.event_date LIKE ?`
		args = append(args, month+"%")
	}
	query := `SELECT e.event_date, e.title
		FROM standing_committee_events e` + where + ` ORDER BY e.event_date, e.id`
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type scItem struct {
		eventDate string
		month     int
		title     string
	}
	list := []scItem{}
	for rows.Next() {
		var eventDate, t sql.NullString
		if err := rows.Scan(&eventDate, &t); err != nil {
			continue
		}
		it := scItem{eventDate: eventDate.String, title: t.String}
		if len(it.eventDate) >= 7 {
			fmt.Sscanf(it.eventDate, "%d-%d", new(int), &it.month)
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	// 按日期/月份顺序排列
	sort.Slice(list, func(i, j int) bool {
		return list[i].eventDate < list[j].eventDate
	})

	var builder docxBuilder
	builder.addTitle("常委大事记")
	title := year
	if month != "" {
		title = month
	}
	builder.addSubTitle(title + "年度")
	builder.addEmpty()
	idx := 1
	var curMonth int
	for _, it := range list {
		if it.month > 0 && it.month != curMonth {
			curMonth = it.month
			builder.addBold(fmt.Sprintf("%d月", curMonth))
		}
		line := fmt.Sprintf("%d. %s　%s", idx, it.eventDate, it.title)
		builder.add(line)
		idx++
	}

	data, err := builder.build()
	if err != nil {
		http.Error(w, "导出失败", http.StatusInternalServerError)
		return
	}
	logOperation(r, "常委管理", "导出", "导出常委大事记（"+title+"）")
	fileName := "常委大事记-" + title + ".docx"
	writeDocx(w, fileName, data)
}
