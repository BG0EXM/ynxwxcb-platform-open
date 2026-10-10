package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// ListDispatches 材料下发任务列表（管理端）
func ListDispatches(w http.ResponseWriter, r *http.Request) {
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
	database.DB.QueryRow("SELECT COUNT(*) FROM dispatches").Scan(&total)

	query := `SELECT d.id, d.title, d.doc_no, d.units, d.content,
			d.pdf_path, d.pdf_name, d.attachment_path, d.attachment_name,
			d.deadline, d.created_by, u.real_name, d.created_at, d.updated_at,
			COALESCE(COUNT(r.id), 0) AS receipt_count
		FROM dispatches d
		LEFT JOIN users u ON d.created_by = u.id
		LEFT JOIN dispatch_receipts r ON r.dispatch_id = d.id
		GROUP BY d.id
		ORDER BY d.id DESC
		LIMIT ? OFFSET ?`

	rows, err := database.DB.Query(query, pageSize, offset)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询下发列表失败"})
		return
	}
	defer rows.Close()

	list := []models.Dispatch{}
	for rows.Next() {
		var d models.Dispatch
		var docNo, content, attPath, attName, deadline, creator sql.NullString
		var createdBy sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		var receiptCount int

		if err := rows.Scan(
			&d.ID, &d.Title, &docNo, &d.Units, &content,
			&d.PdfPath, &d.PdfName, &attPath, &attName,
			&deadline, &createdBy, &creator, &createdAt, &updatedAt,
			&receiptCount,
		); err != nil {
			continue
		}

		d.DocNo = docNo.String
		d.Content = content.String
		d.AttachmentPath = attPath.String
		d.AttachmentName = attName.String
		d.Deadline = deadline.String
		d.CreatedBy = createdBy.Int64
		d.CreatedName = creator.String
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			d.UpdatedAt = updatedAt.Time
		}
		d.ReceiptCount = receiptCount

		// 统计要求下发总单位数
		totalUnits := 0
		for _, u := range strings.Split(d.Units, "\n") {
			if strings.TrimSpace(u) != "" {
				totalUnits++
			}
		}
		d.TotalUnits = totalUnits

		list = append(list, d)
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetDispatch 材料下发任务详情（管理后台，含已查收明细与未查收名单）
func GetDispatch(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少材料ID"})
		return
	}

	var d models.Dispatch
	var docNo, content, attPath, attName, deadline, creator sql.NullString
	var createdBy sql.NullInt64
	var createdAt, updatedAt sql.NullTime

	query := `SELECT d.id, d.title, d.doc_no, d.units, d.content,
			d.pdf_path, d.pdf_name, d.attachment_path, d.attachment_name,
			d.deadline, d.created_by, u.real_name, d.created_at, d.updated_at
		FROM dispatches d
		LEFT JOIN users u ON d.created_by = u.id
		WHERE d.id = ?`

	err := database.DB.QueryRow(query, id).Scan(
		&d.ID, &d.Title, &docNo, &d.Units, &content,
		&d.PdfPath, &d.PdfName, &attPath, &attName,
		&deadline, &createdBy, &creator, &createdAt, &updatedAt,
	)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "材料下发任务不存在"})
		return
	}

	d.DocNo = docNo.String
	d.Content = content.String
	d.AttachmentPath = attPath.String
	d.AttachmentName = attName.String
	d.Deadline = deadline.String
	d.CreatedBy = createdBy.Int64
	d.CreatedName = creator.String
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		d.UpdatedAt = updatedAt.Time
	}

	// 读取已查收单位列表
	receiptRows, err := database.DB.Query(
		`SELECT id, dispatch_id, unit, receiver_name, receiver_phone, read_count, ip, received_at
		 FROM dispatch_receipts
		 WHERE dispatch_id = ?
		 ORDER BY received_at DESC`, id)
	receipts := []models.DispatchReceipt{}
	receiptedUnitMap := make(map[string]bool)

	if err == nil {
		defer receiptRows.Close()
		for receiptRows.Next() {
			var rc models.DispatchReceipt
			var ip sql.NullString
			var rcTime sql.NullTime
			if err := receiptRows.Scan(
				&rc.ID, &rc.DispatchID, &rc.Unit, &rc.ReceiverName,
				&rc.ReceiverPhone, &rc.ReadCount, &ip, &rcTime,
			); err == nil {
				rc.IP = ip.String
				if rcTime.Valid {
					rc.ReceivedAt = rcTime.Time
				}
				receipts = append(receipts, rc)
				receiptedUnitMap[strings.TrimSpace(rc.Unit)] = true
			}
		}
	}

	// 汇总所有待发单位与未查收单位
	allUnits := []string{}
	unreceiptedUnits := []string{}
	for _, u := range strings.Split(d.Units, "\n") {
		uTrim := strings.TrimSpace(u)
		if uTrim == "" {
			continue
		}
		allUnits = append(allUnits, uTrim)
		if !receiptedUnitMap[uTrim] {
			unreceiptedUnits = append(unreceiptedUnits, uTrim)
		}
	}

	d.ReceiptCount = len(receipts)
	d.TotalUnits = len(allUnits)

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"dispatch":          d,
		"receipts":          receipts,
		"unreceipted_units": unreceiptedUnits,
		"all_units":         allUnits,
		"receipt_count":     d.ReceiptCount,
		"total_units":       d.TotalUnits,
	})
}

// CreateDispatch 新增材料下发任务
func CreateDispatch(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.Dispatch
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求参数解析失败"})
			return
		}

		req.Title = strings.TrimSpace(req.Title)
		req.Units = strings.TrimSpace(req.Units)
		req.PdfPath = strings.TrimSpace(req.PdfPath)
		req.PdfName = strings.TrimSpace(req.PdfName)

		if req.Title == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写材料标题"})
			return
		}
		if req.Units == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择或指定下发单位范围"})
			return
		}
		if req.PdfPath == "" || req.PdfName == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请上传正文 PDF 文件"})
			return
		}

		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		res, err := database.DB.Exec(
			`INSERT INTO dispatches (title, doc_no, units, content, pdf_path, pdf_name, attachment_path, attachment_name, deadline, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			req.Title, req.DocNo, req.Units, req.Content, req.PdfPath, req.PdfName, req.AttachmentPath, req.AttachmentName, req.Deadline, userID,
		)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "发布材料失败: " + err.Error()})
			return
		}

		newID, _ := res.LastInsertId()
		logOperation(r, "材料下发", "发布", fmt.Sprintf("发布材料通知「%s」(ID=%d)", req.Title, newID))

		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"message": "材料下发任务发布成功",
			"id":      newID,
		})
	}
}

// UpdateDispatch 编辑材料下发任务
func UpdateDispatch(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.Dispatch
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求参数解析失败"})
			return
		}

		if req.ID == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少材料ID"})
			return
		}

		req.Title = strings.TrimSpace(req.Title)
		req.Units = strings.TrimSpace(req.Units)
		req.PdfPath = strings.TrimSpace(req.PdfPath)
		req.PdfName = strings.TrimSpace(req.PdfName)

		if req.Title == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写材料标题"})
			return
		}
		if req.Units == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请指定下发单位范围"})
			return
		}
		if req.PdfPath == "" || req.PdfName == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请上传正文 PDF 文件"})
			return
		}

		_, err := database.DB.Exec(
			`UPDATE dispatches SET
				title = ?, doc_no = ?, units = ?, content = ?,
				pdf_path = ?, pdf_name = ?, attachment_path = ?, attachment_name = ?,
				deadline = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			req.Title, req.DocNo, req.Units, req.Content,
			req.PdfPath, req.PdfName, req.AttachmentPath, req.AttachmentName,
			req.Deadline, req.ID,
		)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "更新材料失败: " + err.Error()})
			return
		}

		logOperation(r, "材料下发", "修改", fmt.Sprintf("修改材料通知「%s」(ID=%d)", req.Title, req.ID))

		middleware.JSON(w, http.StatusOK, map[string]string{"message": "材料信息更新成功"})
	}
}

// DeleteDispatch 删除材料下发任务
func DeleteDispatch(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少材料ID"})
			return
		}

		var title string
		var pdfPath, attachPath sql.NullString
		if err := database.DB.QueryRow("SELECT title, pdf_path, attachment_path FROM dispatches WHERE id = ?", id).Scan(&title, &pdfPath, &attachPath); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "材料下发任务不存在"})
			return
		}

		// 级联删除查收记录与任务（事务包裹）
		tx, err := database.DB.Begin()
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec("DELETE FROM dispatch_receipts WHERE dispatch_id = ?", id); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
		if _, err := tx.Exec("DELETE FROM dispatches WHERE id = ?", id); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}
		if err := tx.Commit(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
			return
		}

		// 物理删除材料文件与附件
		if pdfPath.Valid && pdfPath.String != "" {
			SafeRemoveUploadedFile(cfg, pdfPath.String)
		}
		if attachPath.Valid && attachPath.String != "" {
			SafeRemoveUploadedFile(cfg, attachPath.String)
		}

		logOperation(r, "材料下发", "删除", fmt.Sprintf("删除材料通知「%s」(ID=%d)并物理清理附件文件", title, id))
		middleware.JSON(w, http.StatusOK, map[string]string{"message": "材料已成功删除"})
	}
}

// ResetUnitDispatchReceipt 管理员重置指定单位的查收状态
func ResetUnitDispatchReceipt(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少材料ID"})
		return
	}

	var req struct {
		Unit string `json:"unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Unit) == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请指定要重置查收状态的单位"})
		return
	}

	req.Unit = strings.TrimSpace(req.Unit)
	res, err := database.DB.Exec("DELETE FROM dispatch_receipts WHERE dispatch_id = ? AND unit = ?", id, req.Unit)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "重置失败"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该单位尚未查收，无需重置"})
		return
	}

	logOperation(r, "材料下发", "重置", fmt.Sprintf("管理员重置了【%s】在材料任务ID=%d中的查收状态", req.Unit, id))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已重置该单位查收状态"})
}

// ExportDispatchReceipts 导出材料查收台账 Excel
func ExportDispatchReceipts(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少材料ID"})
		return
	}

	var title, docNo, unitsRaw string
	err := database.DB.QueryRow("SELECT title, doc_no, units FROM dispatches WHERE id=?", id).Scan(&title, &docNo, &unitsRaw)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "材料不存在"})
		return
	}

	// 读取已查收记录
	rows, err := database.DB.Query(
		`SELECT unit, receiver_name, receiver_phone, read_count, ip, received_at
		 FROM dispatch_receipts WHERE dispatch_id=?`, id)
	receiptMap := map[string]models.DispatchReceipt{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rc models.DispatchReceipt
			var ip sql.NullString
			var rcTime sql.NullTime
			if err := rows.Scan(&rc.Unit, &rc.ReceiverName, &rc.ReceiverPhone, &rc.ReadCount, &ip, &rcTime); err == nil {
				rc.IP = ip.String
				if rcTime.Valid {
					rc.ReceivedAt = rcTime.Time
				}
				receiptMap[strings.TrimSpace(rc.Unit)] = rc
			}
		}
	}

	headers := []string{"序号", "单位名称", "查收状态", "签收经办人", "联系电话", "查收时间", "客户端IP"}
	data := [][]interface{}{}

	idx := 1
	for _, u := range strings.Split(unitsRaw, "\n") {
		unit := strings.TrimSpace(u)
		if unit == "" {
			continue
		}
		if rc, exists := receiptMap[unit]; exists {
			rcTimeStr := ""
			if !rc.ReceivedAt.IsZero() {
				rcTimeStr = rc.ReceivedAt.Format("2006/01/02 15:04:05")
			}
			data = append(data, []interface{}{
				idx, unit, "已查收", rc.ReceiverName, rc.ReceiverPhone, rcTimeStr, rc.IP,
			})
		} else {
			data = append(data, []interface{}{
				idx, unit, "未查收（待催查收）", "—", "—", "—", "—",
			})
		}
		idx++
	}

	logOperation(r, "材料下发", "导出", "导出材料查收台账「"+title+"」")
	fileName := "材料查收台账-" + title + ".xlsx"
	exportExcel(w, "材料查收台账", fileName, headers, data)
}

// PublicDispatch 公开端加载材料信息（匿名免密）
func PublicDispatch(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}

	var d models.Dispatch
	var docNo, content, attPath, attName, deadline sql.NullString
	var createdAt sql.NullTime

	query := `SELECT id, title, doc_no, units, content, pdf_path, pdf_name, attachment_path, attachment_name, deadline, created_at
		FROM dispatches WHERE id = ?`

	err := database.DB.QueryRow(query, id).Scan(
		&d.ID, &d.Title, &docNo, &d.Units, &content,
		&d.PdfPath, &d.PdfName, &attPath, &attName, &deadline, &createdAt,
	)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "该材料不存在或已被撤回"})
		return
	}

	d.DocNo = docNo.String
	d.Content = content.String
	d.AttachmentPath = attPath.String
	d.AttachmentName = attName.String
	d.Deadline = deadline.String
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}

	// 整理单位列表
	units := []string{}
	for _, u := range strings.Split(d.Units, "\n") {
		uTrim := strings.TrimSpace(u)
		if uTrim != "" {
			units = append(units, uTrim)
		}
	}

	expired := false
	if d.Deadline != "" {
		expired = isDeadlineExpired(d.Deadline)
	}

	// 若前端传递了 ?unit=xxx，回查该单位是否已查收
	queryUnit := strings.TrimSpace(r.URL.Query().Get("unit"))
	var currentReceipt *models.DispatchReceipt
	if queryUnit != "" {
		var rc models.DispatchReceipt
		var ip sql.NullString
		var rcTime sql.NullTime
		err := database.DB.QueryRow(
			`SELECT id, dispatch_id, unit, receiver_name, receiver_phone, read_count, ip, received_at
			 FROM dispatch_receipts WHERE dispatch_id = ? AND unit = ?`, id, queryUnit).
			Scan(&rc.ID, &rc.DispatchID, &rc.Unit, &rc.ReceiverName, &rc.ReceiverPhone, &rc.ReadCount, &ip, &rcTime)
		if err == nil {
			rc.IP = ip.String
			if rcTime.Valid {
				rc.ReceivedAt = rcTime.Time
			}
			currentReceipt = &rc
		}
	}

	// 返回数据，隐藏具体内部存储全路径，仅保留下载/预览标志
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"dispatch": map[string]interface{}{
			"id":              d.ID,
			"title":           d.Title,
			"doc_no":          d.DocNo,
			"content":         d.Content,
			"pdf_name":        d.PdfName,
			"attachment_name": d.AttachmentName,
			"has_attachment":  d.AttachmentPath != "",
			"deadline":        d.Deadline,
			"created_at":      d.CreatedAt,
		},
		"units":           units,
		"expired":         expired,
		"has_attachment":  d.AttachmentPath != "",
		"current_receipt": currentReceipt,
	})
}

// PublicServeDispatchFile 公开受控读取正文 PDF 或配套附件（在线预览与下载）
func PublicServeDispatchFile(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		fileType := r.URL.Query().Get("type") // "pdf" 或 "attachment"
		mode := r.URL.Query().Get("mode")     // "inline" (内嵌查看) 或 "download" (下载)

		var pdfPath, pdfName, attPath, attName string
		err := database.DB.QueryRow(
			"SELECT pdf_path, pdf_name, attachment_path, attachment_name FROM dispatches WHERE id=?", id).
			Scan(&pdfPath, &pdfName, &attPath, &attName)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		targetPath := pdfPath
		targetName := pdfName
		if fileType == "attachment" {
			targetPath = attPath
			targetName = attName
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
		case ".xlsx":
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		case ".xls":
			w.Header().Set("Content-Type", "application/vnd.ms-excel")
		case ".zip":
			w.Header().Set("Content-Type", "application/zip")
		case ".rar":
			w.Header().Set("Content-Type", "application/x-rar-compressed")
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		http.ServeFile(w, r, fullPath)
	}
}

// PublicConfirmDispatchReceipt 公开确认查收接口
func PublicConfirmDispatchReceipt(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if id == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数错误"})
		return
	}

	var req struct {
		Unit          string `json:"unit"`
		ReceiverName  string `json:"receiver_name"`
		ReceiverPhone string `json:"receiver_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	req.Unit = strings.TrimSpace(req.Unit)
	req.ReceiverName = strings.TrimSpace(req.ReceiverName)
	req.ReceiverPhone = strings.TrimSpace(req.ReceiverPhone)

	if req.Unit == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择所属单位"})
		return
	}
	if req.ReceiverName == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写经办人姓名"})
		return
	}
	if req.ReceiverPhone == "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写经办人联系电话"})
		return
	}

	// 校验材料任务存在
	var unitsRaw, deadline string
	err := database.DB.QueryRow("SELECT units, deadline FROM dispatches WHERE id = ?", id).Scan(&unitsRaw, &deadline)
	if err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "材料下发任务不存在"})
		return
	}

	// 校验截止时限：若已设置截止时限且已过期，直接拦截
	if deadline != "" && isDeadlineExpired(deadline) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "公文材料查收时限已截止"})
		return
	}

	// 校验单位在下发范围内
	unitValid := false
	for _, u := range strings.Split(unitsRaw, "\n") {
		if strings.TrimSpace(u) == req.Unit {
			unitValid = true
			break
		}
	}
	if !unitValid {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "所选单位不在本材料下发范围内"})
		return
	}

	ip := clientIP(r)

	// 插入或更新查收记录
	_, err = database.DB.Exec(
		`INSERT INTO dispatch_receipts (dispatch_id, unit, receiver_name, receiver_phone, read_count, ip, received_at)
		 VALUES (?, ?, ?, ?, 1, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(dispatch_id, unit) DO UPDATE SET
			receiver_name = excluded.receiver_name,
			receiver_phone = excluded.receiver_phone,
			read_count = dispatch_receipts.read_count + 1,
			ip = excluded.ip,
			received_at = CURRENT_TIMESTAMP`,
		id, req.Unit, req.ReceiverName, req.ReceiverPhone, ip,
	)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查收登记失败: " + err.Error()})
		return
	}

	// 查询返回最新的查收记录
	var rc models.DispatchReceipt
	var ipNull sql.NullString
	var rcTime sql.NullTime
	database.DB.QueryRow(
		`SELECT id, dispatch_id, unit, receiver_name, receiver_phone, read_count, ip, received_at
		 FROM dispatch_receipts WHERE dispatch_id = ? AND unit = ?`, id, req.Unit).
		Scan(&rc.ID, &rc.DispatchID, &rc.Unit, &rc.ReceiverName, &rc.ReceiverPhone, &rc.ReadCount, &ipNull, &rcTime)
	rc.IP = ipNull.String
	if rcTime.Valid {
		rc.ReceivedAt = rcTime.Time
	}

	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "查收成功",
		"receipt": rc,
	})
}
