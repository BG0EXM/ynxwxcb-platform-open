package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

// allowedOwnerTypes 附件可声明的业务类型白名单
var allowedOwnerTypes = map[string]bool{"document": true, "study": true, "solicit": true, "dispatch": true}

// UploadFile 文件上传
func UploadFile(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		realName, _ := r.Context().Value(middleware.ContextRealName).(string)

		// 硬性限制请求体大小（防止超大文件打爆磁盘），略留 multipart 开销余量
		r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Upload.MaxMB+2)<<20)
		// 仅 8MB 驻留内存，超出部分由标准库自动落盘临时文件，防止并发大文件上传打爆内存
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件过大，超过限制"})
			return
		}
		ownerType := r.FormValue("owner_type")
		ownerID := r.FormValue("owner_id")
		if ownerType != "" && !allowedOwnerTypes[ownerType] {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不支持的业务类型"})
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "未选择文件"})
			return
		}
		defer file.Close()

		// 安全校验：文件扩展名白名单
		ext := strings.ToLower(filepath.Ext(header.Filename))
		allowedExt := map[string]bool{".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
			".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".txt": true, ".zip": true, ".rar": true, ".ppt": true, ".pptx": true}
		if !allowedExt[ext] {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "不支持的文件类型"})
			return
		}

		// 保密安全校验：严禁上传国家秘密（秘密/机密/绝密）、工作秘密及内部级文件（支持正文/页眉内容深度解构）
		var fileReader io.Reader = file
		if res, restoredReader := secrecy.CheckFileSecrecy(header.Filename, file); res.Unreadable {
			// 无法完整检测的 Office 文档一律拒绝（失败即关闭），不能绕过涉密审查
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件无法解析或超过检测上限，请用 Office/WPS 另存为标准格式后重新上传"})
			return
		} else if res.Violated {
			logOperation(r, "保密防线", "涉密阻断", fmt.Sprintf("拦截涉密文件上传: %s (密级: %s, 规则: %s, 详情: %s)", header.Filename, res.Category, res.Rule, res.Detail))
			middleware.JSON(w, http.StatusForbidden, map[string]interface{}{
				"error":              "【国家保密安全警报】检测到文件带有国家秘密或内部保密标识，严禁在非涉密系统上传传输！",
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

		// 校验大小
		if header.Size > cfg.Upload.MaxMB<<20 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件超过大小限制"})
			return
		}

		// 保存文件
		dateDir := time.Now().Format("2006/01")
		saveDir := filepath.Join(cfg.Upload.Dir, dateDir)
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建目录失败"})
			return
		}

		fileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizeFileName(filepath.Base(header.Filename)))
		savePath := filepath.Join(saveDir, fileName)
		dst, err := os.Create(savePath)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
			return
		}
		if _, err := io.Copy(dst, fileReader); err != nil {
			dst.Close()
			os.Remove(savePath)
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "写入失败"})
			return
		}
		if err := dst.Close(); err != nil {
			os.Remove(savePath)
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "写入失败"})
			return
		}

		// 记录到数据库：新上传附件一律处于未关联状态（owner_id=0），由各业务保存时校验归属后再绑定，
		// 杜绝上传时直接把文件挂到他人业务记录上
		oType := ownerType
		if oType == "" {
			oType = "document"
		}
		_ = ownerID
		var oID int64

		// 统一存储“相对上传目录”的路径（如 2026/10/xxx.pdf），upload.dir 相对/绝对、数据迁移目录均可正确解析
		relPath := dateDir + "/" + fileName
		res, err := database.DB.Exec(
			"INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES (?, ?, ?, ?, ?, ?)",
			oType, oID, header.Filename, relPath, header.Size, userID)
		if err != nil {
			os.Remove(savePath)
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "记录失败"})
			return
		}
		attID, _ := res.LastInsertId()

		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"message":   "上传成功",
			"id":        attID,
			"file_name": header.Filename,
			"file_path": "/api/uploads/" + strconv.FormatInt(attID, 10),
			"uploader":  realName,
		})
	}
}

// DownloadAttachment 下载附件（通过数据库记录 ID）
func DownloadAttachment(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
		id := pathID(r)
		if id == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少附件ID"})
			return
		}
		var a models.Attachment
		var uploaderID, ownerID int64
		var ownerType string
		err := database.DB.QueryRow(
			"SELECT id, owner_type, file_name, file_path, file_size, uploader_id, owner_id FROM attachments WHERE id=?", id).
			Scan(&a.ID, &ownerType, &a.FileName, &a.FilePath, &a.FileSize, &uploaderID, &ownerID)
		if err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "附件不存在"})
			return
		}

		if msg := attachmentDenyReason(userID, roleCode, ownerType, uploaderID, ownerID); msg != "" {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg})
			return
		}

		// 文件路径安全解析（防目录穿越与相对路径逃逸）
		fullPath := resolveFilePath(cfg.Upload.Dir, a.FilePath)
		if fullPath == "" {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "附件路径非法或已被移除"})
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "文件已被移除"})
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", a.FileName, url.PathEscape(a.FileName)))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, fullPath)
	}
}

// attachmentDenyReason 附件下载鉴权：管理员与上传者本人直接放行；其余按附件业务类型校验对应功能权限。
// 返回空字符串表示放行，否则为拒绝原因。
func attachmentDenyReason(userID int64, roleCode, ownerType string, uploaderID, ownerID int64) string {
	if roleCode == "admin" || (uploaderID != 0 && uploaderID == userID) {
		return ""
	}
	if ownerID == 0 {
		return "无权下载该未关联附件"
	}
	perm := ""
	switch ownerType {
	case "document":
		perm = "incoming.view"
	case "study":
		perm = "study.view"
	case "solicit":
		perm = "solicit.manage"
	case "dispatch":
		perm = "dispatch.manage"
	}
	if perm == "" || !middleware.HasPermission(roleCode, perm) {
		return "无权下载该业务附件"
	}
	return ""
}

// ServeUploadFile 受控直读 /uploads/ 下的文件：
//   - 征求意见各单位回函（solicits/ 目录）需征求意见管理权限（大小写不敏感比对，防 macOS 大小写绕过）；
//   - 其余文件必须能反查到附件记录，并套用与 /api/uploads/{id} 相同的权限校验；
//   - 隔离区（.trash）一律禁止访问。
func ServeUploadFile(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
		abs := resolveFilePath(cfg.Upload.Dir, r.URL.Path)
		if abs == "" || !fileExists(abs) {
			http.NotFound(w, r)
			return
		}
		rel := relToBase(uploadBase(cfg.Upload.Dir), abs)
		first := strings.ToLower(strings.SplitN(rel, "/", 2)[0])
		if first == trashDirName {
			http.NotFound(w, r)
			return
		}
		if first == "solicits" {
			if roleCode != "admin" && !middleware.HasPermission(roleCode, "solicit.manage") {
				middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权访问此敏感公函附件"})
				return
			}
		} else {
			var ownerType string
			var uploaderID, ownerID int64
			err := database.DB.QueryRow("SELECT owner_type, uploader_id, owner_id FROM attachments WHERE file_path IN (?, ?) LIMIT 1", rel, abs).
				Scan(&ownerType, &uploaderID, &ownerID)
			if err != nil && roleCode != "admin" {
				middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该文件"})
				return
			}
			if err == nil {
				if msg := attachmentDenyReason(userID, roleCode, ownerType, uploaderID, ownerID); msg != "" {
					middleware.JSON(w, http.StatusForbidden, map[string]string{"error": msg})
					return
				}
			}
		}

		fileName := filepath.Base(abs)
		if idx := strings.Index(fileName, "_"); idx > 0 && idx < len(fileName)-1 {
			fileName = fileName[idx+1:]
		}
		disp := "attachment"
		switch strings.ToLower(filepath.Ext(abs)) {
		case ".pdf":
			w.Header().Set("Content-Type", "application/pdf")
			disp = "inline"
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
			disp = "inline"
		case ".png":
			w.Header().Set("Content-Type", "image/png")
			disp = "inline"
		case ".docx":
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		case ".doc":
			w.Header().Set("Content-Type", "application/msword")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=UTF-8''%s", disp, fileName, url.PathEscape(fileName)))
		http.ServeFile(w, r, abs)
	}
}

// LinkAttachment 将已上传附件关联到业务对象
func LinkAttachment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      int64 `json:"id"`
		OwnerID int64 `json:"owner_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.ID == 0 || req.OwnerID == 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "参数不完整"})
		return
	}
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	var uploaderID, currentOwner int64
	var ownerType string
	if err := database.DB.QueryRow("SELECT uploader_id, owner_id, owner_type FROM attachments WHERE id=?", req.ID).Scan(&uploaderID, &currentOwner, &ownerType); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "附件不存在"})
		return
	}
	// 本接口仅用于公共资料附件关联；征求意见、材料下发由各自保存接口校验并绑定
	if ownerType != "study" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该附件不支持此关联方式"})
		return
	}
	var exists int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM study_materials WHERE id=?", req.OwnerID).Scan(&exists); err != nil || exists == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "资料不存在"})
		return
	}
	// 权限：管理员可关联任意附件；普通用户只能关联自己上传、且未被他人占用的附件
	if roleCode != "admin" {
		if uploaderID != userID {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权关联该附件"})
			return
		}
		if currentOwner != 0 && currentOwner != req.OwnerID {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "该附件已被占用"})
			return
		}
	}
	_, err := database.DB.Exec("UPDATE attachments SET owner_id=? WHERE id=?", req.OwnerID, req.ID)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "关联失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "关联成功"})
}

// DashboardStats 首页统计
func DashboardStats(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)

	result := map[string]interface{}{}
	today := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")
	year := time.Now().Format("2006")

	// 今日值守（支持一天多人，姓名用「、」连接）
	dutyRows, err := database.DB.Query(
		"SELECT u.real_name, s.is_dawangyuan FROM duty_schedules s JOIN users u ON s.user_id=u.id WHERE s.duty_date=? ORDER BY s.id",
		today)
	dutyNames := []string{}
	dutyDaWangYuan := 0
	if err == nil {
		defer dutyRows.Close()
		for dutyRows.Next() {
			var name string
			var isDa int
			if e := dutyRows.Scan(&name, &isDa); e != nil {
				continue
			}
			dutyNames = append(dutyNames, name)
			if isDa == 1 {
				dutyDaWangYuan = 1
			}
		}
		if err := dutyRows.Err(); err != nil {
			log.Printf("[查询] 遍历结果失败: %v", err)
		}
	}
	result["today_duty"] = strings.Join(dutyNames, "、")
	result["today_duty_dawangyuan"] = dutyDaWangYuan

	// 今日考勤状态（当前用户）
	var todayStatus int
	var todayCheckin string
	err = database.DB.QueryRow(
		"SELECT status, IFNULL(remark,'') FROM attendances WHERE user_id=? AND attend_date=?", userID, today).Scan(&todayStatus, &todayCheckin)
	if err == nil {
		result["today_attendance"] = todayStatus
	} else {
		result["today_attendance"] = 0
	}

	// 本月考勤汇总（当前用户）
	var monthPresent, monthLeave int
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances WHERE user_id=? AND attend_date GLOB ? AND status=1", userID, month+"*").Scan(&monthPresent)
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances WHERE user_id=? AND attend_date GLOB ? AND status=2", userID, month+"*").Scan(&monthLeave)
	result["month_present"] = monthPresent
	result["month_leave"] = monthLeave

	// 本月请假天数（跨月假期按当月实际覆盖天数计算）
	var monthLeaveDays float64
	monthStart := month + "-01"
	// 计算月末
	ym, _ := time.Parse("2006-01", month)
	monthEnd := ym.AddDate(0, 1, -1).Format("2006-01-02")
	database.DB.QueryRow(
		`SELECT COALESCE(SUM(eff),0) FROM (
			SELECT MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
				- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
			FROM leave_records
			WHERE user_id = ? AND status = 1 AND start_date <= ? AND end_date >= ?
			GROUP BY id
		) WHERE eff > 0`,
		monthEnd, monthEnd, monthStart, monthStart, userID, monthEnd, monthStart).Scan(&monthLeaveDays)
	result["month_leave_days"] = monthLeaveDays

	// 今日用车报备
	var todayVehicle int
	database.DB.QueryRow("SELECT COUNT(*) FROM vehicle_applies WHERE use_date=?", today).Scan(&todayVehicle)
	result["today_vehicle"] = todayVehicle

	// 待处理收文（今日收到的未办结收文，管理员看全部）
	var pendingIncoming int
	if roleCode == "admin" {
		database.DB.QueryRow("SELECT COUNT(*) FROM incoming_docs WHERE status < 5").Scan(&pendingIncoming)
	} else {
		database.DB.QueryRow("SELECT COUNT(*) FROM incoming_docs WHERE status < 5 AND registrar_id=?", userID).Scan(&pendingIncoming)
	}
	result["pending_incoming"] = pendingIncoming

	// 最新收文列表（工作台展示，仅具备收文查看权限者可见）
	rows, err := permQuery(roleCode, "incoming.view",
		`SELECT id, receive_no, received_date, from_unit, from_doc_no, title, status
		 FROM incoming_docs ORDER BY id DESC LIMIT 6`)
	if err == nil {
		defer rows.Close()
		type IncomingBrief struct {
			ID           int64  `json:"id"`
			ReceiveNo    string `json:"receive_no"`
			ReceivedDate string `json:"received_date"`
			FromUnit     string `json:"from_unit"`
			FromDocNo    string `json:"from_doc_no"`
			Title        string `json:"title"`
			Status       int    `json:"status"`
		}
		latest := []IncomingBrief{}
		for rows.Next() {
			var b IncomingBrief
			if err := rows.Scan(&b.ID, &b.ReceiveNo, &b.ReceivedDate, &b.FromUnit, &b.FromDocNo, &b.Title, &b.Status); err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
				return
			}
			latest = append(latest, b)
		}
		if err := rows.Err(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
		}

		result["latest_incoming"] = latest
	} else {
		result["latest_incoming"] = []interface{}{}
	}

	// 本周排班预览
	weekRows, err := database.DB.Query(
		`SELECT s.duty_date, u.real_name, s.is_dawangyuan FROM duty_schedules s
		 JOIN users u ON s.user_id=u.id
		 WHERE s.duty_date >= date('now','localtime','weekday 0','-6 days') AND s.duty_date <= date('now','localtime','weekday 0')
		 ORDER BY s.duty_date`)
	if err == nil {
		defer weekRows.Close()
		type DutyBrief struct {
			DutyDate     string `json:"duty_date"`
			UserName     string `json:"user_name"`
			IsDaWangYuan int    `json:"is_dawangyuan"`
		}
		weekDuty := []DutyBrief{}
		for weekRows.Next() {
			var d DutyBrief
			if err := weekRows.Scan(&d.DutyDate, &d.UserName, &d.IsDaWangYuan); err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
				return
			}
			weekDuty = append(weekDuty, d)
		}
		if err := weekRows.Err(); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
			return
		}

		result["week_duty"] = weekDuty
	} else {
		result["week_duty"] = []interface{}{}
	}

	// 本周工作总结是否已提交（本月）
	// 全年请假汇总（管理员看全员，跨年假期按当年实际覆盖天数计算）
	if roleCode == "admin" {
		var annualDays float64
		yearStart := year + "-01-01"
		yearEnd := year + "-12-31"
		database.DB.QueryRow(
			`SELECT COALESCE(SUM(eff),0) FROM (
				SELECT MIN(days, CAST(julianday(CASE WHEN end_date < ? THEN end_date ELSE ? END)
					- julianday(CASE WHEN start_date > ? THEN start_date ELSE ? END) + 1 AS INTEGER)) as eff
				FROM leave_records
				WHERE status = 1 AND start_date <= ? AND end_date >= ?
				GROUP BY id
			) WHERE eff > 0`,
			yearEnd, yearEnd, yearStart, yearStart, yearEnd, yearStart).Scan(&annualDays)
		result["year_leave_days"] = annualDays
	}

	// 近 6 个月业务趋势（收文量按收文日期、用车量按用车日期），空月份补 0
	now := time.Now()
	firstMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -5, 0)
	trendMonths := make([]string, 6)
	for i := 0; i < 6; i++ {
		trendMonths[i] = firstMonth.AddDate(0, i, 0).Format("2006-01")
	}
	countByMonth := func(query string) []int {
		counts := make([]int, 6)
		idx := map[string]int{}
		for i, m := range trendMonths {
			idx[m] = i
		}
		rows, err := database.DB.Query(query, trendMonths[0]+"-01")
		if err != nil {
			return counts
		}
		defer rows.Close()
		for rows.Next() {
			var m string
			var c int
			if rows.Scan(&m, &c) == nil {
				if i, ok := idx[m]; ok {
					counts[i] = c
				}
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[查询] 遍历结果失败: %v", err)
		}
		return counts
	}
	result["trend"] = map[string]interface{}{
		"months":   trendMonths,
		"incoming": countByMonth("SELECT substr(received_date,1,7) AS m, COUNT(*) FROM incoming_docs WHERE received_date >= ? GROUP BY m"),
		"vehicle":  countByMonth("SELECT substr(use_date,1,7) AS m, COUNT(*) FROM vehicle_applies WHERE use_date >= ? GROUP BY m"),
	}

	middleware.JSON(w, http.StatusOK, result)
}

// Health 健康检查
func Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "time": time.Now().Format("2006-01-02 15:04:05")})
}

// pathID 提取路径中的ID
func pathID(r *http.Request) int64 {
	if v := r.PathValue("id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			return id
		}
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return id
}

// SafeRemoveUploadedFile 安全删除上传目录下的物理文件并同步清理 attachments 关联记录
func SafeRemoveUploadedFile(cfg *config.Config, fileURLOrPath string) {
	fileURLOrPath = strings.TrimSpace(fileURLOrPath)
	if fileURLOrPath == "" || cfg == nil {
		return
	}
	base := uploadBase(cfg.Upload.Dir)

	// 1. /api/uploads/{id} 形式：删除该附件的物理文件与记录
	if strings.HasPrefix(fileURLOrPath, "/api/uploads/") {
		idStr := strings.TrimPrefix(fileURLOrPath, "/api/uploads/")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id > 0 {
			deletePhysicalPath(cfg, resolveFilePath(cfg.Upload.Dir, fileURLOrPath))
			database.DB.Exec("DELETE FROM attachments WHERE id=?", id)
		}
		return
	}

	// 2. Web 路径或存储路径：解析为上传目录内的真实文件后删除，并清理指向同一文件的附件记录
	abs := resolveFilePath(cfg.Upload.Dir, fileURLOrPath)
	if abs == "" {
		return
	}
	deletePhysicalPath(cfg, abs)
	database.DB.Exec("DELETE FROM attachments WHERE file_path IN (?, ?, ?)", fileURLOrPath, abs, relToBase(base, abs))
}

// deletePhysicalPath 删除上传目录内的文件；p 必须为已解析的绝对路径，越界一律拒绝
func deletePhysicalPath(cfg *config.Config, p string) {
	if p == "" || cfg == nil {
		return
	}
	base := uploadBase(cfg.Upload.Dir)
	target := filepath.Clean(p)
	if !filepath.IsAbs(target) || !withinBase(base, target) {
		return
	}
	if fileExists(target) {
		if err := os.Remove(target); err == nil {
			log.Printf("[文件清理] 成功物理清理磁盘文件: %s", target)
		}
	}
}

// 孤儿清扫熔断阈值：候选文件较多且超过一半时，判定为路径解析异常，整轮放弃
const (
	orphanBreakerMinCount = 10
	trashDirName          = ".trash"
	trashKeepDays         = 30
)

// CleanupOrphanFiles 扫描超过 24 小时且无任何数据库引用的孤儿文件，移入 .trash/日期/ 隔离区（保留 30 天后才物理删除）
func CleanupOrphanFiles(cfg *config.Config) {
	if cfg == nil || cfg.Upload.Dir == "" {
		return
	}
	base := uploadBase(cfg.Upload.Dir)
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		return
	}

	activeFiles := make(map[string]bool)
	totalRefs, danglingRefs := 0, 0
	addActivePath := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		totalRefs++
		abs := resolveFilePath(cfg.Upload.Dir, p)
		if abs == "" || !fileExists(abs) {
			danglingRefs++
			return
		}
		activeFiles[abs] = true
	}

	// 白名单：attachments 及各业务表引用的全部文件；任一查询失败即熔断
	queries := []struct{ name, sql string }{
		{"attachments", "SELECT file_path FROM attachments WHERE file_path != ''"},
		// solicits 为 pdf_path, word_path；solicit_feedbacks 为 reply_doc_path, attachment_path
		{"solicits/solicit_feedbacks", `
		SELECT pdf_path FROM solicits WHERE pdf_path != ''
		UNION SELECT word_path FROM solicits WHERE word_path != ''
		UNION SELECT reply_doc_path FROM solicit_feedbacks WHERE reply_doc_path != ''
		UNION SELECT attachment_path FROM solicit_feedbacks WHERE attachment_path != ''`},
		// dispatches 为 pdf_path, attachment_path
		{"dispatches", `
		SELECT pdf_path FROM dispatches WHERE pdf_path != ''
		UNION SELECT attachment_path FROM dispatches WHERE attachment_path != ''`},
	}
	for _, q := range queries {
		rows, err := database.DB.Query(q.sql)
		if err != nil {
			log.Printf("[孤儿文件清理] 熔断防护触发：查询 %s 表失败: %v，已终止清理以保护数据安全", q.name, err)
			return
		}
		for rows.Next() {
			var p sql.NullString
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				log.Printf("[孤儿文件清理] 熔断防护触发：扫描 %s 数据失败: %v，已终止清理", q.name, err)
				return
			}
			addActivePath(p.String)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			log.Printf("[孤儿文件清理] 熔断防护触发：遍历 %s 发生错误: %v，已终止清理", q.name, err)
			return
		}
		rows.Close()
	}

	// 大量引用解析不到真实文件，说明路径口径异常（如上传目录配置变更），整轮放弃以免误删
	if danglingRefs >= orphanBreakerMinCount && danglingRefs*2 > totalRefs {
		log.Printf("[孤儿文件清理] 熔断防护触发：%d/%d 条文件引用无法解析到真实文件，疑似上传目录配置异常，已终止清理", danglingRefs, totalRefs)
		return
	}

	trashRoot := filepath.Join(base, trashDirName)
	cutoff := time.Now().Add(-24 * time.Hour)
	totalFiles := 0
	var candidates []string
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if path == trashRoot {
				return filepath.SkipDir
			}
			return nil
		}
		totalFiles++
		// 仅清理修改时间在 24 小时之前的孤儿文件（防止误删当前正在并发上传的临时文件）
		if info.ModTime().Before(cutoff) && !activeFiles[filepath.Clean(path)] {
			candidates = append(candidates, filepath.Clean(path))
		}
		return nil
	})

	if len(candidates) >= orphanBreakerMinCount && len(candidates)*2 > totalFiles {
		log.Printf("[孤儿文件清理] 熔断防护触发：待清理 %d 个 / 共 %d 个文件，比例异常，已终止清理以保护数据安全", len(candidates), totalFiles)
		return
	}

	moved := 0
	dayDir := filepath.Join(trashRoot, time.Now().Format("20060102"))
	for _, p := range candidates {
		dst := filepath.Join(dayDir, relToBase(base, p))
		if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
			continue
		}
		if err := os.Rename(p, dst); err == nil {
			moved++
			log.Printf("[孤儿文件清理] 无引用历史残留文件已移入隔离区: %s", dst)
		}
	}
	if moved > 0 {
		log.Printf("[孤儿文件清理] 完成历史残留文件大扫除，共隔离 %d 个孤儿文件（%d 天后物理删除）", moved, trashKeepDays)
	}

	purgeOldTrash(trashRoot)
}

// purgeOldTrash 物理删除隔离区中超过保留期的日期目录
func purgeOldTrash(trashRoot string) {
	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		return
	}
	limit := time.Now().AddDate(0, 0, -trashKeepDays)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		day, err := time.ParseInLocation("20060102", e.Name(), time.Local)
		if err != nil || !day.Before(limit) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(trashRoot, e.Name())); err == nil {
			log.Printf("[孤儿文件清理] 隔离区过期目录已物理删除: %s", e.Name())
		}
	}
}

// incomingSummary 收文列表顶部统计（全量口径，不随分页与筛选变化）
func incomingSummary() map[string]int {
	var total, pending, done, needReturn sql.NullInt64
	database.DB.QueryRow(`SELECT COUNT(*),
		SUM(CASE WHEN status < 5 THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 5 THEN 1 ELSE 0 END),
		SUM(CASE WHEN need_return = 1 AND IFNULL(returned, 0) != 1 THEN 1 ELSE 0 END)
		FROM incoming_docs`).Scan(&total, &pending, &done, &needReturn)
	return map[string]int{"total": int(total.Int64), "pending": int(pending.Int64), "done": int(done.Int64), "need_return": int(needReturn.Int64)}
}

// deadlineSummary 征求意见/材料下发列表顶部统计：进行中与已截止数量（按截止时间实时判断）
func deadlineSummary(table string) map[string]int {
	q := "SELECT IFNULL(deadline, '') FROM solicits"
	if table == "dispatches" {
		q = "SELECT IFNULL(deadline, '') FROM dispatches"
	}
	out := map[string]int{"total": 0, "active": 0, "expired": 0}
	rows, err := database.DB.Query(q)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if rows.Scan(&d) != nil {
			continue
		}
		out["total"]++
		if d != "" && isDeadlineExpired(d) {
			out["expired"]++
		} else {
			out["active"]++
		}
	}
	return out
}

// meetingSummary 会务列表顶部统计：待召开与已召开数量
func meetingSummary() map[string]int {
	out := map[string]int{"total": 0, "upcoming": 0, "finished": 0}
	rows, err := database.DB.Query("SELECT IFNULL(meeting_date, ''), IFNULL(meeting_time, '') FROM meetings")
	if err != nil {
		return out
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var d, t string
		if rows.Scan(&d, &t) != nil {
			continue
		}
		out["total"]++
		if mt, ok := parseMeetingTime(d, t); ok && now.Before(mt) {
			out["upcoming"]++
		} else {
			out["finished"]++
		}
	}
	return out
}
