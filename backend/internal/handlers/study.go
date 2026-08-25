package handlers

import (
	"encoding/json"
	"net/http"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListStudyMaterials 公共资料列表
func ListStudyMaterials(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	category := r.URL.Query().Get("category")

	query := `SELECT s.id, s.title, s.content, s.category, s.publisher_id, u.real_name, s.read_count, s.created_at, s.updated_at
		FROM study_materials s LEFT JOIN users u ON s.publisher_id = u.id WHERE 1=1`
	args := []interface{}{}
	if keyword != "" {
		query += ` AND s.title LIKE ?`
		args = append(args, "%"+keyword+"%")
	}
	if category != "" {
		query += ` AND s.category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY s.id DESC`

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	materials := []models.StudyMaterial{}
	for rows.Next() {
		var s models.StudyMaterial
		rows.Scan(&s.ID, &s.Title, &s.Content, &s.Category, &s.PublisherID, &s.Publisher,
			&s.ReadCount, &s.CreatedAt, &s.UpdatedAt)
		materials = append(materials, s)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": materials})
}

// CreateStudyMaterial 发布公共资料
func CreateStudyMaterial(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	var req models.StudyMaterial
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Title == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "标题必填"})
		return
	}
	res, err := database.DB.Exec(
		`INSERT INTO study_materials (title, content, category, publisher_id) VALUES (?, ?, ?, ?)`,
		req.Title, req.Content, req.Category, userID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
		return
	}
	id, _ := res.LastInsertId()
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "发布成功", "id": id})
}

// GetStudyMaterial 公共资料详情
func GetStudyMaterial(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少资料ID"})
		return
	}
	var s models.StudyMaterial
	err := database.DB.QueryRow(
		`SELECT s.id, s.title, s.content, s.category, s.publisher_id, u.real_name, s.read_count, s.created_at
		 FROM study_materials s LEFT JOIN users u ON s.publisher_id = u.id WHERE s.id = ?`, id).
		Scan(&s.ID, &s.Title, &s.Content, &s.Category, &s.PublisherID, &s.Publisher,
			&s.ReadCount, &s.CreatedAt)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "资料不存在"})
		return
	}
	database.DB.Exec("UPDATE study_materials SET read_count = read_count + 1 WHERE id = ?", id)

	// 附件
	rows, err := database.DB.Query("SELECT id, file_name, file_path, file_size, created_at FROM attachments WHERE owner_type='study' AND owner_id=?", id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var a models.Attachment
			rows.Scan(&a.ID, &a.FileName, &a.FilePath, &a.FileSize, &a.CreatedAt)
			s.Attachments = append(s.Attachments, a)
		}
		if err := rows.Err(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
		}

	}

	middleware.JSON(w, http.StatusOK, s)
}

// DeleteStudyMaterial 删除公共资料
func DeleteStudyMaterial(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	_, err := database.DB.Exec("DELETE FROM study_materials WHERE id=?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}

// StudyCategory 公共资料分类
type StudyCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
	Sort int    `json:"sort"`
}

// ListStudyCategories 公共资料分类列表
func ListStudyCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, code, sort FROM study_categories ORDER BY sort")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	list := []StudyCategory{}
	for rows.Next() {
		var c StudyCategory
		rows.Scan(&c.ID, &c.Name, &c.Code, &c.Sort)
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// CreateStudyCategory 新增分类（管理员）
func CreateStudyCategory(w http.ResponseWriter, r *http.Request) {
	var req StudyCategory
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Name == "" || req.Code == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "分类名称和标识不能为空"})
		return
	}
	var cnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM study_categories WHERE code = ?", req.Code).Scan(&cnt)
	if cnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "分类标识已存在"})
		return
	}
	var maxSort int
	database.DB.QueryRow("SELECT COALESCE(MAX(sort), 0) FROM study_categories").Scan(&maxSort)
	_, err := database.DB.Exec("INSERT INTO study_categories (name, code, sort) VALUES (?, ?, ?)", req.Name, req.Code, maxSort+1)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "添加失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "添加成功"})
}

// UpdateStudyCategory 修改分类（管理员）
func UpdateStudyCategory(w http.ResponseWriter, r *http.Request) {
	var req StudyCategory
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 || req.Name == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数不完整"})
		return
	}
	_, err := database.DB.Exec("UPDATE study_categories SET name = ? WHERE id = ?", req.Name, req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "修改失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "修改成功"})
}

// DeleteStudyCategory 删除分类（管理员，分类下有资料时禁止删除）
func DeleteStudyCategory(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}
	var code string
	err := database.DB.QueryRow("SELECT code FROM study_categories WHERE id = ?", id).Scan(&code)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "分类不存在"})
		return
	}
	var cnt int
	database.DB.QueryRow("SELECT COUNT(*) FROM study_materials WHERE category = ?", code).Scan(&cnt)
	if cnt > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该分类下仍有资料，请先调整后再删除"})
		return
	}
	_, err = database.DB.Exec("DELETE FROM study_categories WHERE id = ?", id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
}
