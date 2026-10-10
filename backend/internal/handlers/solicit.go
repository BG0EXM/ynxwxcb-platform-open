package handlers

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
	"ynxwxcb-platform/internal/secrecy"
)

// ListSolicits 征求意见任务列表（管理员/带权限用户）
func ListSolicits(w http.ResponseWriter, r *http.Request) {
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
	database.DB.QueryRow("SELECT COUNT(*) FROM solicits").Scan(&total)

	query := `SELECT s.id, s.title, s.doc_no, s.deadline, s.units, s.content,
			s.pdf_path, s.pdf_name, s.word_path, s.word_name,
			s.created_by, u.real_name, s.created_at, s.updated_at,
			COALESCE(COUNT(f.id), 0) AS feedback_count,
			COALESCE(SUM(CASE WHEN f.has_opinion = 1 THEN 1 ELSE 0 END), 0) AS opinion_count
		FROM solicits s
		LEFT JOIN users u ON s.created_by = u.id
		LEFT JOIN solicit_feedbacks f ON f.solicit_id = s.id
		GROUP BY s.id
		ORDER BY s.id DESC
		LIMIT ? OFFSET ?`

	rows, err := database.DB.Query(query, pageSize, offset)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()

	list := []models.Solicit{}
	for rows.Next() {
		var s models.Solicit
		var docNo, content, wordPath, wordName, creator sql.NullString
		var createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		var feedbackCount, opinionCount int

		if err := rows.Scan(
			&s.ID, &s.Title, &docNo, &s.Deadline, &s.Units, &content,
			&s.PdfPath, &s.PdfName, &wordPath, &wordName,
			&createdBy, &creator, &createdAt, &updatedAt,
			&feedbackCount, &opinionCount,
		); err != nil {
			continue
		}

		s.DocNo = docNo.String
		s.Content = content.String
		s.WordPath = wordPath.String
		s.WordName = wordName.String
		s.CreatedBy = createdBy.Int64
		s.CreatedName = creator.String
		if createdAt.Valid {
			s.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			s.UpdatedAt = updatedAt.Time
		}
		s.FeedbackCount = feedbackCount
		s.OpinionCount = opinionCount

		// 计算单位总数
		totalUnits := 0
		for _, u := range strings.Split(s.Units, "\n") {
			if strings.TrimSpace(u) != "" {
				totalUnits++
			}
		}
		s.TotalUnits = totalUnits

		list = append(list, s)
	}

	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "total": total})
}

// CreateSolicit 新增征求意见任务
func CreateSolicit(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		var req models.Solicit
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
			return
		}

		if strings.TrimSpace(req.Title) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件标题必填"})
			return
		}
		if strings.TrimSpace(req.Deadline) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "截止时间必填"})
			return
		}
		if strings.TrimSpace(req.Units) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "征求单位范围必填"})
			return
		}
		if strings.TrimSpace(req.PdfPath) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请上传正文 PDF 底稿"})
			return
		}

		res, err := database.DB.Exec(
			`INSERT INTO solicits (id, title, doc_no, deadline, units, content, pdf_path, pdf_name, word_path, word_name, created_by)
			 SELECT COALESCE(MAX(id), 0) + 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? FROM solicits`,
			req.Title, req.DocNo, req.Deadline, req.Units, req.Content, req.PdfPath, req.PdfName, req.WordPath, req.WordName, userID)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建失败"})
			return
		}

		lastID, _ := res.LastInsertId()
		logOperation(r, "征求意见", "新增", "新增征求意见「"+req.Title+"」")
		middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "创建成功", "id": lastID})
	}
}

// UpdateSolicit 修改征求意见任务
func UpdateSolicit(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.Solicit
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
			return
		}
		if req.ID == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
			return
		}
		if strings.TrimSpace(req.Title) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件标题必填"})
			return
		}
		if strings.TrimSpace(req.Deadline) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "截止时间必填"})
			return
		}
		if strings.TrimSpace(req.Units) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "征求单位范围必填"})
			return
		}

		_, err := database.DB.Exec(
			`UPDATE solicits SET title=?, doc_no=?, deadline=?, units=?, content=?,
				pdf_path=?, pdf_name=?, word_path=?, word_name=?, updated_at=?
			 WHERE id=?`,
			req.Title, req.DocNo, req.Deadline, req.Units, req.Content,
			req.PdfPath, req.PdfName, req.WordPath, req.WordName, time.Now(), req.ID)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新失败"})
			return
		}

		logOperation(r, "征求意见", "修改", "修改征求意见「"+req.Title+"」")
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
	}
}

// DeleteSolicit 删除征求意见任务（级联删除反馈记录）
func DeleteSolicit(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
			return
		}

		var title string
		var pdfPath, wordPath sql.NullString
		if err := database.DB.QueryRow("SELECT title, pdf_path, word_path FROM solicits WHERE id=?", id).Scan(&title, &pdfPath, &wordPath); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "征求意见任务不存在"})
			return
		}

		// 收集该任务下所有反馈提交的回函文件与附件，以便事务提交后物理删除
		var feedbackFiles []string
		if fbRows, err := database.DB.Query("SELECT reply_doc_path, attachment_path FROM solicit_feedbacks WHERE solicit_id=?", id); err == nil {
			for fbRows.Next() {
				var rDoc, aDoc sql.NullString
				if err := fbRows.Scan(&rDoc, &aDoc); err == nil {
					if rDoc.Valid && rDoc.String != "" {
						feedbackFiles = append(feedbackFiles, rDoc.String)
					}
					if aDoc.Valid && aDoc.String != "" {
						feedbackFiles = append(feedbackFiles, aDoc.String)
					}
				}
			}
			fbRows.Close()
		}

		tx, err := database.DB.Begin()
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec("DELETE FROM solicit_feedbacks WHERE solicit_id=?", id); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}

		if _, err := tx.Exec("DELETE FROM solicits WHERE id=?", id); err != nil {
			tx.Rollback()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}

		// 重置自增序列，防止跳号
		tx.Exec("DELETE FROM sqlite_sequence WHERE name='solicits'")
		tx.Exec("INSERT OR REPLACE INTO sqlite_sequence (name, seq) VALUES ('solicits', (SELECT COALESCE(MAX(id),0) FROM solicits))")

		if err := tx.Commit(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}

		// 物理清理磁盘文件（草案、附件、所有单位盖章回函与修改稿）
		if pdfPath.Valid && pdfPath.String != "" {
			SafeRemoveUploadedFile(cfg, pdfPath.String)
		}
		if wordPath.Valid && wordPath.String != "" {
			SafeRemoveUploadedFile(cfg, wordPath.String)
		}
		for _, f := range feedbackFiles {
			SafeRemoveUploadedFile(cfg, f)
		}

		logOperation(r, "征求意见", "删除", fmt.Sprintf("删除征求意见「%s」(ID=%d)并物理清理关联草案与回函文件", title, id))
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "删除成功"})
	}
}

// GetSolicit 征求意见任务详情（管理员后台，含反馈名单与未反馈单位）
func GetSolicit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}

	var s models.Solicit
	var docNo, content, wordPath, wordName, creator sql.NullString
	var createdBy sql.NullInt64
	var createdAt, updatedAt sql.NullTime

	err := database.DB.QueryRow(
		`SELECT s.id, s.title, s.doc_no, s.deadline, s.units, s.content,
			s.pdf_path, s.pdf_name, s.word_path, s.word_name,
			s.created_by, u.real_name, s.created_at, s.updated_at
		FROM solicits s
		LEFT JOIN users u ON s.created_by = u.id
		WHERE s.id = ?`, id).
		Scan(
			&s.ID, &s.Title, &docNo, &s.Deadline, &s.Units, &content,
			&s.PdfPath, &s.PdfName, &wordPath, &wordName,
			&createdBy, &creator, &createdAt, &updatedAt,
		)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "征求意见不存在"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	s.DocNo = docNo.String
	s.Content = content.String
	s.WordPath = wordPath.String
	s.WordName = wordName.String
	s.CreatedBy = createdBy.Int64
	s.CreatedName = creator.String
	if createdAt.Valid {
		s.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		s.UpdatedAt = updatedAt.Time
	}

	// 查询反馈记录
	rows, err := database.DB.Query(
		`SELECT id, solicit_id, unit, has_opinion, opinion_detail,
			reply_doc_path, reply_doc_name, attachment_path, attachment_name,
			contact_name, contact_phone, created_at, updated_at
		FROM solicit_feedbacks WHERE solicit_id=? ORDER BY has_opinion DESC, id ASC`, id)
	feedbacks := []models.SolicitFeedback{}
	feedbackUnitSet := map[string]bool{}
	opinionCount := 0

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f models.SolicitFeedback
			var opDetail, replyPath, replyName, attPath, attName sql.NullString
			var crAt, upAt sql.NullTime
			if err := rows.Scan(
				&f.ID, &f.SolicitID, &f.Unit, &f.HasOpinion, &opDetail,
				&replyPath, &replyName, &attPath, &attName,
				&f.ContactName, &f.ContactPhone, &crAt, &upAt,
			); err != nil {
				continue
			}
			f.OpinionDetail = opDetail.String
			f.ReplyDocPath = replyPath.String
			f.ReplyDocName = replyName.String
			f.AttachmentPath = attPath.String
			f.AttachmentName = attName.String
			if crAt.Valid {
				f.CreatedAt = crAt.Time
			}
			if upAt.Valid {
				f.UpdatedAt = upAt.Time
			}
			if f.HasOpinion == 1 {
				opinionCount++
			}
			f.Unit = strings.TrimSpace(f.Unit)
			feedbacks = append(feedbacks, f)
			if f.Unit != "" {
				feedbackUnitSet[f.Unit] = true
			}
		}
	}

	// 计算未反馈单位
	unconfirmed := []string{}
	totalUnits := 0
	for _, u := range strings.Split(s.Units, "\n") {
		trimmed := strings.TrimSpace(u)
		if trimmed != "" {
			totalUnits++
			if !feedbackUnitSet[trimmed] {
				unconfirmed = append(unconfirmed, trimmed)
			}
		}
	}

	s.FeedbackCount = len(feedbacks)
	s.TotalUnits = totalUnits
	s.OpinionCount = opinionCount

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"solicit":           s,
		"feedbacks":         feedbacks,
		"unconfirmed_units": unconfirmed,
	})
}

// ResetUnitFeedback 管理员重置指定单位的反馈（允许单位重新提交，并级联物理清理旧文件）
func ResetUnitFeedback(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少任务ID"})
			return
		}
		var req struct {
			Unit string `json:"unit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Unit) == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请提供要重置的单位名称"})
			return
		}
		req.Unit = strings.TrimSpace(req.Unit)

		// 查出该单位已上传的旧回函文件与修改附件
		var rDoc, aDoc sql.NullString
		database.DB.QueryRow("SELECT reply_doc_path, attachment_path FROM solicit_feedbacks WHERE solicit_id=? AND unit=?", id, req.Unit).Scan(&rDoc, &aDoc)

		_, err := database.DB.Exec("DELETE FROM solicit_feedbacks WHERE solicit_id=? AND unit=?", id, req.Unit)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "重置失败"})
			return
		}

		if rDoc.Valid && rDoc.String != "" {
			SafeRemoveUploadedFile(cfg, rDoc.String)
		}
		if aDoc.Valid && aDoc.String != "" {
			SafeRemoveUploadedFile(cfg, aDoc.String)
		}

		logOperation(r, "征求意见", "重置", fmt.Sprintf("管理员重置了【%s】在任务ID=%d中的反馈状态并清理旧附件", req.Unit, id))
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "已重置该单位反馈状态"})
	}
}

// ExportSolicitFeedbacks 导出征求意见反馈汇总表（Excel）
func ExportSolicitFeedbacks(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
		return
	}

	var title, docNo, unitsRaw string
	err := database.DB.QueryRow("SELECT title, doc_no, units FROM solicits WHERE id=?", id).Scan(&title, &docNo, &unitsRaw)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}

	// 读取已反馈数据
	rows, err := database.DB.Query(
		`SELECT unit, has_opinion, opinion_detail, reply_doc_name, attachment_name, contact_name, contact_phone, created_at
		 FROM solicit_feedbacks WHERE solicit_id=?`, id)
	feedbackMap := map[string]models.SolicitFeedback{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f models.SolicitFeedback
			var opDetail, replyName, attName sql.NullString
			var crAt sql.NullTime
			if err := rows.Scan(&f.Unit, &f.HasOpinion, &opDetail, &replyName, &attName, &f.ContactName, &f.ContactPhone, &crAt); err == nil {
				f.Unit = strings.TrimSpace(f.Unit)
				f.ContactName = strings.TrimSpace(f.ContactName)
				f.ContactPhone = strings.TrimSpace(f.ContactPhone)
				f.OpinionDetail = strings.TrimSpace(opDetail.String)
				f.ReplyDocName = strings.TrimSpace(replyName.String)
				f.AttachmentName = strings.TrimSpace(attName.String)
				if crAt.Valid {
					f.CreatedAt = crAt.Time
				}
				if f.Unit != "" {
					feedbackMap[f.Unit] = f
				}
			}
		}
	}

	headers := []string{"序号", "单位名称", "反馈状态", "修改意见", "意见具体内容", "盖章红头回函", "修改稿附件", "经办人", "联系电话", "反馈时间"}
	data := [][]interface{}{}

	idx := 1
	for _, u := range strings.Split(unitsRaw, "\n") {
		unit := strings.TrimSpace(u)
		if unit == "" {
			continue
		}
		if fb, exists := feedbackMap[unit]; exists {
			opStatus := "原则同意，无意见"
			if fb.HasOpinion == 1 {
				opStatus = "提出修改意见"
			}
			fbTime := ""
			if !fb.CreatedAt.IsZero() {
				fbTime = fb.CreatedAt.Format("2006/01/02 15:04")
			}
			data = append(data, []interface{}{
				idx, unit, "已反馈", opStatus, fb.OpinionDetail, fb.ReplyDocName, fb.AttachmentName, fb.ContactName, fb.ContactPhone, fbTime,
			})
		} else {
			data = append(data, []interface{}{
				idx, unit, "未反馈（待催报）", "—", "—", "—", "—", "—", "—", "—",
			})
		}
		idx++
	}

	logOperation(r, "征求意见", "导出", "导出征求意见反馈汇总表「"+title+"」")
	fileName := "征求意见反馈汇总-" + title + ".xlsx"
	exportExcel(w, "反馈汇总", fileName, headers, data)
}

// DownloadSolicitRepliesZip 一键打包下载全部单位上传的盖章回函（ZIP 流式下载）
func DownloadSolicitRepliesZip(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少ID"})
			return
		}

		var title string
		if err := database.DB.QueryRow("SELECT title FROM solicits WHERE id=?", id).Scan(&title); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
			return
		}

		rows, err := database.DB.Query(
			`SELECT unit, reply_doc_path, reply_doc_name, attachment_path, attachment_name
			 FROM solicit_feedbacks WHERE solicit_id=?`, id)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
		}
		defer rows.Close()

		type FileEntry struct {
			zipName  string
			diskPath string
		}
		entries := []FileEntry{}

		for rows.Next() {
			var unit string
			var replyPath, replyName, attPath, attName sql.NullString
			if err := rows.Scan(&unit, &replyPath, &replyName, &attPath, &attName); err != nil {
				continue
			}

			if replyPath.Valid && replyPath.String != "" {
				fullReplyPath := resolveFilePath(cfg.Upload.Dir, replyPath.String)
				if _, err := os.Stat(fullReplyPath); err == nil {
					ext := filepath.Ext(replyName.String)
					if ext == "" {
						ext = filepath.Ext(replyPath.String)
					}
					cleanUnit := sanitizeFileName(strings.TrimSpace(unit))
					entries = append(entries, FileEntry{
						zipName:  fmt.Sprintf("%s_盖章回函%s", cleanUnit, ext),
						diskPath: fullReplyPath,
					})
				}
			}

			if attPath.Valid && attPath.String != "" {
				fullAttPath := resolveFilePath(cfg.Upload.Dir, attPath.String)
				if _, err := os.Stat(fullAttPath); err == nil {
					ext := filepath.Ext(attName.String)
					if ext == "" {
						ext = filepath.Ext(attPath.String)
					}
					cleanUnit := sanitizeFileName(unit)
					entries = append(entries, FileEntry{
						zipName:  fmt.Sprintf("%s_修改稿%s", cleanUnit, ext),
						diskPath: fullAttPath,
					})
				}
			}
		}

		if len(entries) == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "暂无可下载的单位回函附件"})
			return
		}

		zipFileName := fmt.Sprintf("各单位盖章回函汇总-%s.zip", title)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", zipFileName, url.PathEscape(zipFileName)))

		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		for _, e := range entries {
			f, err := os.Open(e.diskPath)
			if err != nil {
				continue
			}
			fh := &zip.FileHeader{
				Name:   e.zipName,
				Method: zip.Deflate,
				Flags:  0x800, // UTF-8 编码文件名
			}
			zf, err := zipWriter.CreateHeader(fh)
			if err != nil {
				f.Close()
				continue
			}
			_, _ = io.Copy(zf, f)
			f.Close()
		}

		logOperation(r, "征求意见", "导出", "批量下载各单位盖章回函压缩包「"+title+"」")
	}
}

// -------------------------------------------------------------
// 公开免密端 API
// -------------------------------------------------------------

// PublicSolicit 公开征求意见信息加载（匿名免密）
func PublicSolicit(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}

	var s models.Solicit
	var docNo, content, wordPath, wordName sql.NullString
	var createdAt sql.NullTime

	err := database.DB.QueryRow(
		`SELECT id, title, doc_no, deadline, units, content, pdf_path, pdf_name, word_path, word_name, created_at
		 FROM solicits WHERE id=?`, id).
		Scan(&s.ID, &s.Title, &docNo, &s.Deadline, &s.Units, &content, &s.PdfPath, &s.PdfName, &wordPath, &wordName, &createdAt)
	if err == sql.ErrNoRows {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "征求意见文件不存在或已下线"})
		return
	}
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}

	s.DocNo = docNo.String
	s.Content = content.String
	s.WordPath = wordPath.String
	s.WordName = wordName.String
	if createdAt.Valid {
		s.CreatedAt = createdAt.Time
	}

	// 拆分单位列表
	units := []string{}
	for _, u := range strings.Split(s.Units, "\n") {
		trimmed := strings.TrimSpace(u)
		if trimmed != "" {
			units = append(units, trimmed)
		}
	}

	// 校验是否截止
	expired := isDeadlineExpired(s.Deadline)

	// 若传了 unit 参数，返回该单位当前已提交的反馈（方便回显与重修）
	var currentFeedback map[string]interface{}
	if unit := r.URL.Query().Get("unit"); unit != "" {
		var fID, hasOp int64
		var opDetail, replyPath, replyName, attPath, attName, cName, cPhone sql.NullString
		var crAt, upAt sql.NullTime
		qerr := database.DB.QueryRow(
			`SELECT id, has_opinion, opinion_detail, reply_doc_path, reply_doc_name,
				attachment_path, attachment_name, contact_name, contact_phone, created_at, updated_at
			 FROM solicit_feedbacks WHERE solicit_id=? AND unit=?`, id, unit).
			Scan(&fID, &hasOp, &opDetail, &replyPath, &replyName, &attPath, &attName, &cName, &cPhone, &crAt, &upAt)
		if qerr == nil {
			var createdAtVal, updatedAtVal interface{}
			if crAt.Valid {
				createdAtVal = crAt.Time
			}
			if upAt.Valid {
				updatedAtVal = upAt.Time
			}
			hasReplyDoc := replyPath.Valid && replyPath.String != ""
			currentFeedback = map[string]interface{}{
				"id":              fID,
				"solicit_id":      id,
				"unit":            unit,
				"has_opinion":     hasOp,
				"opinion_detail":  opDetail.String,
				"has_reply_doc":   hasReplyDoc,
				"reply_doc_name":  replyName.String,
				"attachment_name": attName.String,
				"contact_name":    cName.String,
				"contact_phone":   maskPhoneNumber(cPhone.String),
				"created_at":      createdAtVal,
				"updated_at":      updatedAtVal,
			}
		}
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"solicit":          s,
		"units":            units,
		"expired":          expired,
		"has_word":         s.WordPath != "",
		"current_feedback": currentFeedback,
	})
}

// PublicServeSolicitFile 公开受控读取正文 PDF 或配套 Word（支持在线预览与下载）
func PublicServeSolicitFile(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		fileType := r.URL.Query().Get("type") // "pdf" 或 "word"
		mode := r.URL.Query().Get("mode")     // "inline" (内嵌查看) 或 "download" (下载)

		var pdfPath, pdfName, wordPath, wordName string
		err := database.DB.QueryRow(
			"SELECT pdf_path, pdf_name, word_path, word_name FROM solicits WHERE id=?", id).
			Scan(&pdfPath, &pdfName, &wordPath, &wordName)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		targetPath := pdfPath
		targetName := pdfName
		if fileType == "word" {
			targetPath = wordPath
			targetName = wordName
		}
		if targetPath == "" {
			http.NotFound(w, r)
			return
		}

		fullPath := resolveFilePath(cfg.Upload.Dir, targetPath)
		if _, err := os.Stat(fullPath); err != nil {
			http.NotFound(w, r)
			return
		}

		disp := "inline"
		if mode == "download" {
			disp = "attachment"
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=UTF-8''%s", disp, targetName, url.PathEscape(targetName)))

		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		ext := strings.ToLower(filepath.Ext(targetName))
		switch ext {
		case ".pdf":
			w.Header().Set("Content-Type", "application/pdf")
		case ".docx":
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		case ".doc":
			w.Header().Set("Content-Type", "application/msword")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		http.ServeFile(w, r, fullPath)
	}
}

// PublicSolicitUpload 公开上传盖章回函或修改稿附件
func PublicSolicitUpload(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
			return
		}

		// 检查任务存在且未截止
		var deadline string
		if err := database.DB.QueryRow("SELECT deadline FROM solicits WHERE id=?", id).Scan(&deadline); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "征求意见任务不存在"})
			return
		}
		if isDeadlineExpired(deadline) {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "意见征集已截止，无法再上传文件"})
			return
		}

		// 硬性限制请求体大小（单文件最大 20MB）
		const maxUploadMB = 20
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxUploadMB+2)<<20)
		if err := r.ParseMultipartForm(int64(maxUploadMB) << 20); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件过大，单文件不得超过 20MB"})
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "未选择文件"})
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		allowedExt := map[string]bool{
			".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".doc": true, ".docx": true,
		}
		if !allowedExt[ext] {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "格式不支持，仅支持 PDF、图片(JPG/PNG) 或 Word(DOC/DOCX)"})
			return
		}

		// 保密安全校验：严禁公开端上传国家秘密（秘密/机密/绝密）、工作秘密及内部级文件资料（含正文与附件深度解析）
		var fileReader io.Reader = file
		if res, restoredReader := secrecy.CheckFileSecrecy(header.Filename, file); res.Violated {
			unitName := r.FormValue("unit_name")
			if unitName == "" {
				unitName = "外网填报单位"
			}
			logWithUserDept(0, "外网填报经办人", unitName, "保密防线", "公开端涉密阻断",
				fmt.Sprintf("拦截【%s】上传涉密公函: %s (密级: %s, 规则: %s, 详情: %s)", unitName, header.Filename, res.Category, res.Rule, res.Detail),
				clientIP(r),
			)
			middleware.JSON(w, http.StatusForbidden, map[string]interface{}{
				"error":              "【国家保密安全警报】检测到文件带有国家秘密或内部保密标识，严禁上传涉密材料！",
				"security_violation": "SECRECY_LEAK_PREVENTED",
				"filename":           header.Filename,
				"matched_rule":       res.Rule,
				"category":           res.Category,
				"detail":             res.Detail,
			})
			return
		} else {
			fileReader = restoredReader
		}

		dateDir := time.Now().Format("2006/01")
		saveDir := filepath.Join(cfg.Upload.Dir, "solicits", dateDir)
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "存储目录创建失败"})
			return
		}

		fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(header.Filename))
		savePath := filepath.Join(saveDir, fileName)
		dst, err := os.Create(savePath)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, fileReader); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "写入失败"})
			return
		}

		webPath := "/uploads/solicits/" + dateDir + "/" + fileName
		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"message":   "上传成功",
			"file_path": webPath,
			"file_name": header.Filename,
			"file_size": header.Size,
		})
	}
}

// PublicSubmitFeedback 公开提交本单位反馈意见与红头回函
func PublicSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}

	var req struct {
		Unit           string `json:"unit"`
		HasOpinion     int    `json:"has_opinion"`
		OpinionDetail  string `json:"opinion_detail"`
		ReplyDocPath   string `json:"reply_doc_path"`
		ReplyDocName   string `json:"reply_doc_name"`
		AttachmentPath string `json:"attachment_path"`
		AttachmentName string `json:"attachment_name"`
		ContactName    string `json:"contact_name"`
		ContactPhone   string `json:"contact_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	// 校验任务存在与截止
	var deadline, unitsRaw string
	err := database.DB.QueryRow("SELECT deadline, units FROM solicits WHERE id=?", id).Scan(&deadline, &unitsRaw)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "征求意见任务不存在"})
		return
	}
	if isDeadlineExpired(deadline) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "意见征集已截止，无法再提交"})
		return
	}

	req.Unit = strings.TrimSpace(req.Unit)
	if req.Unit == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择所属单位"})
		return
	}

	// 校验单位在预设范围中
	unitValid := false
	for _, u := range strings.Split(unitsRaw, "\n") {
		if strings.TrimSpace(u) == req.Unit {
			unitValid = true
			break
		}
	}
	if !unitValid {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "所选单位不在本次征求意见范围内"})
		return
	}

	// 校验盖章红头回函必须上传
	if strings.TrimSpace(req.ReplyDocPath) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请上传经单位主要领导审签并加盖公章的红头回函"})
		return
	}
	if !strings.HasPrefix(req.ReplyDocPath, "/uploads/solicits/") || strings.Contains(req.ReplyDocPath, "..") {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "回函文件路径非法"})
		return
	}
	if strings.TrimSpace(req.AttachmentPath) != "" {
		if !strings.HasPrefix(req.AttachmentPath, "/uploads/solicits/") || strings.Contains(req.AttachmentPath, "..") {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "修改稿附件路径非法"})
			return
		}
	}

	req.ContactName = strings.TrimSpace(req.ContactName)
	if req.ContactName == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写经办人姓名"})
		return
	}
	req.ContactPhone = strings.TrimSpace(req.ContactPhone)
	if req.ContactPhone == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写经办人联系电话"})
		return
	}

	// 防恶意修改安全锁定：若该单位已正式提交过，公开端禁止随意篡改覆盖，需由管理员在后台重置
	var existingID int64
	err = database.DB.QueryRow("SELECT id FROM solicit_feedbacks WHERE solicit_id=? AND unit=?", id, req.Unit).Scan(&existingID)
	if err == nil && existingID > 0 {
		middleware.JSON(w, http.StatusForbidden, map[string]string{
			"error": "为确保公文征求意见的法律效力与严肃性，防止外部他人随意篡改，已提交的盖章回函与修改意见已即时存证归档并锁定。公开端已关闭自行撤回或编辑；如确因重大特殊情况需变更意见或补充材料，请联系宣传部办公室核实，由管理员在后台安全重置解锁后方可重新填报。",
		})
		return
	}

	clientIP := middleware.ClientIP(r)

	// 插入新记录
	query := `INSERT INTO solicit_feedbacks (
			solicit_id, unit, has_opinion, opinion_detail,
			reply_doc_path, reply_doc_name, attachment_path, attachment_name,
			contact_name, contact_phone, ip, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	_, err = database.DB.Exec(
		query,
		id, req.Unit, req.HasOpinion, req.OpinionDetail,
		req.ReplyDocPath, req.ReplyDocName, req.AttachmentPath, req.AttachmentName,
		req.ContactName, req.ContactPhone, clientIP, now, now,
	)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "提交保存失败"})
		return
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"message":   "提交成功",
		"unit":      req.Unit,
		"time":      now.Format("2006-01-02 15:04:05"),
		"timestamp": now.Unix(),
	})
}

// -------------------------------------------------------------
// 内部工具函数
// -------------------------------------------------------------

func isDeadlineExpired(deadlineStr string) bool {
	deadlineStr = strings.TrimSpace(deadlineStr)
	if deadlineStr == "" {
		return false
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, deadlineStr, time.Local); err == nil {
			if l == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Second)
			}
			return time.Now().After(t)
		}
	}
	return false
}

func maskPhoneNumber(phone string) string {
	phone = strings.TrimSpace(phone)
	runes := []rune(phone)
	n := len(runes)
	if n >= 11 {
		return string(runes[:3]) + "****" + string(runes[n-4:])
	} else if n >= 7 {
		return string(runes[:3]) + "****" + string(runes[n-2:])
	} else if n > 3 {
		return string(runes[:1]) + "****" + string(runes[n-1:])
	}
	return "****"
}

func resolveFilePath(baseDir, rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || baseDir == "" {
		return ""
	}
	// 严禁包含反斜杠或冒号（防止跨平台路径混淆或 Windows 绝对路径绕过）
	if strings.Contains(rawPath, "\\") || strings.Contains(rawPath, ":") {
		return ""
	}
	// 如果是 /api/uploads/{id} 格式，从 attachments 表查出文件真实路径
	if strings.HasPrefix(rawPath, "/api/uploads/") {
		attIDStr := strings.TrimPrefix(rawPath, "/api/uploads/")
		if id, err := strconv.ParseInt(attIDStr, 10, 64); err == nil && id > 0 {
			var actualPath string
			if err := database.DB.QueryRow("SELECT file_path FROM attachments WHERE id=?", id).Scan(&actualPath); err == nil {
				rawPath = actualPath
			}
		}
	}

	cleanBase, err := filepath.Abs(baseDir)
	if err != nil {
		cleanBase = filepath.Clean(baseDir)
	}

	var candidate string
	if strings.HasPrefix(rawPath, "/uploads/") {
		candidate = filepath.Join(cleanBase, strings.TrimPrefix(rawPath, "/uploads/"))
	} else if filepath.IsAbs(rawPath) {
		// 严禁直接返回任意绝对路径！
		candidate = rawPath
	} else {
		candidate = filepath.Join(cleanBase, rawPath)
	}

	cleanTarget := filepath.Clean(candidate)
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return ""
	}

	// 强校验解析后的路径在 baseDir 目录之内，若越界或包含 .. 必须拒绝返回空字符串
	rel, err := filepath.Rel(cleanBase, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.Contains(rel, "..") || rel == "." {
		return ""
	}

	return absTarget
}

func sanitizeFileName(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return r.Replace(name)
}
