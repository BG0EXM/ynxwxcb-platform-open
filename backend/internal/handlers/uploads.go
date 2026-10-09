package handlers

import (
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

// UploadFile 文件上传
func UploadFile(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
		realName, _ := r.Context().Value(middleware.ContextRealName).(string)

		// 硬性限制请求体大小（防止超大文件打爆磁盘），略留 multipart 开销余量
		r.Body = http.MaxBytesReader(w, r.Body, int64(cfg.Upload.MaxMB+2)<<20)
		if err := r.ParseMultipartForm(cfg.Upload.MaxMB << 20); err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "文件过大，超过限制"})
			return
		}
		ownerType := r.FormValue("owner_type")
		ownerID := r.FormValue("owner_id")
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
		if res, restoredReader := secrecy.CheckFileSecrecy(header.Filename, file); res.Violated {
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

		// 记录到数据库
		oType := ownerType
		if oType == "" {
			oType = "document"
		}
		var oID int64
		if ownerID != "" {
			oID, _ = strconv.ParseInt(ownerID, 10, 64)
		}

		webPath := "/uploads/" + dateDir + "/" + fileName
		// 存储路径使用相对 webPath 便于访问
		dbPath := webPath
		absSavePath := savePath

		// 保存实际路径信息
		res, err := database.DB.Exec(
			"INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES (?, ?, ?, ?, ?, ?)",
			oType, oID, header.Filename, absSavePath, header.Size, userID)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "记录失败"})
			return
		}
		attID, _ := res.LastInsertId()
		_ = dbPath

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
		err := database.DB.QueryRow(
			"SELECT id, file_name, file_path, file_size, uploader_id, owner_id FROM attachments WHERE id=?", id).
			Scan(&a.ID, &a.FileName, &a.FilePath, &a.FileSize, &uploaderID, &ownerID)
		if err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "附件不存在"})
			return
		}
		// 权限：管理员、上传者本人、或已关联到业务对象的附件（业务数据已由各自接口鉴权）可下载
		if roleCode != "admin" && uploaderID != userID && ownerID == 0 {
			middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "无权下载该附件"})
			return
		}
		// 文件路径以 /uploads/ 开头则转实际路径（使用配置的上传目录，兼容旧数据）
		fullPath := a.FilePath
		if strings.HasPrefix(fullPath, "/uploads/") {
			fullPath = filepath.Join(cfg.Upload.Dir, strings.TrimPrefix(fullPath, "/uploads/"))
		}
		// 若数据库存的是绝对路径，直接使用
		if _, err := os.Stat(fullPath); err != nil {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "文件已被移除"})
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", a.FileName, url.PathEscape(a.FileName)))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, fullPath)
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
	if err := database.DB.QueryRow("SELECT uploader_id, owner_id FROM attachments WHERE id=?", req.ID).Scan(&uploaderID, &currentOwner); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "附件不存在"})
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
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances WHERE user_id=? AND attend_date LIKE ? AND status=1", userID, month+"%").Scan(&monthPresent)
	database.DB.QueryRow("SELECT COUNT(*) FROM attendances WHERE user_id=? AND attend_date LIKE ? AND status=2", userID, month+"%").Scan(&monthLeave)
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

	// 最新收文列表（工作台展示）
	rows, err := database.DB.Query(
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
		 WHERE s.duty_date >= date('now','weekday 0','-6 days') AND s.duty_date <= date('now','weekday 0')
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
	if fileURLOrPath == "" || cfg == nil {
		return
	}

	// 1. 如果是 /api/uploads/123 形式的附件引用
	if strings.HasPrefix(fileURLOrPath, "/api/uploads/") {
		idStr := strings.TrimPrefix(fileURLOrPath, "/api/uploads/")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil && id > 0 {
			var physicalPath string
			if err := database.DB.QueryRow("SELECT file_path FROM attachments WHERE id=?", id).Scan(&physicalPath); err == nil {
				deletePhysicalPath(cfg, physicalPath)
			}
			database.DB.Exec("DELETE FROM attachments WHERE id=?", id)
			return
		}
	}

	// 2. 如果是 web 访问路径，如 /uploads/2026/10/xxx 或 /uploads/solicits/2026/10/xxx
	cleanURL := strings.TrimPrefix(fileURLOrPath, "/")
	if strings.HasPrefix(cleanURL, "uploads/") {
		rel := strings.TrimPrefix(cleanURL, "uploads/")
		realPath := filepath.Join(cfg.Upload.Dir, rel)
		deletePhysicalPath(cfg, realPath)
		database.DB.Exec("DELETE FROM attachments WHERE file_path=? OR file_path=?", realPath, fileURLOrPath)
		return
	}

	// 3. 其它路径（如直接存的物理相对或绝对路径）
	deletePhysicalPath(cfg, fileURLOrPath)
	database.DB.Exec("DELETE FROM attachments WHERE file_path=?", fileURLOrPath)
}

func deletePhysicalPath(cfg *config.Config, p string) {
	if p == "" || cfg == nil {
		return
	}
	cleanP := filepath.Clean(p)
	absUploadDir, err1 := filepath.Abs(cfg.Upload.Dir)
	absTarget, err2 := filepath.Abs(cleanP)
	if err1 != nil || err2 != nil {
		return
	}

	// 安全边界校验：必须严格在 cfg.Upload.Dir 目录下，防止路径穿越删除系统关键文件
	rel, err := filepath.Rel(absUploadDir, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return
	}

	if fi, err := os.Stat(absTarget); err == nil && !fi.IsDir() {
		if err := os.Remove(absTarget); err == nil {
			log.Printf("[文件清理] 成功物理清理磁盘文件: %s", absTarget)
		}
	}
}

// CleanupOrphanFiles 扫描并清理磁盘上超过24小时且无任何数据库引用的孤儿残留文件
func CleanupOrphanFiles(cfg *config.Config) {
	if cfg == nil || cfg.Upload.Dir == "" {
		return
	}

	absUploadDir, err := filepath.Abs(cfg.Upload.Dir)
	if err != nil {
		return
	}
	if fi, err := os.Stat(absUploadDir); err != nil || !fi.IsDir() {
		return
	}

	activeFiles := make(map[string]bool)

	// 1. 收集 attachments 表活跃文件
	if rows, err := database.DB.Query("SELECT file_path FROM attachments"); err == nil {
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil && p != "" {
				if absP, err := filepath.Abs(p); err == nil {
					activeFiles[absP] = true
				}
				if strings.HasPrefix(p, "/uploads/") {
					realP := filepath.Join(absUploadDir, strings.TrimPrefix(p, "/uploads/"))
					activeFiles[realP] = true
				}
			}
		}
		rows.Close()
	}

	// 2. 收集征求意见 solicits / solicit_feedbacks 活跃文件
	if rows, err := database.DB.Query("SELECT draft_file_url FROM solicits WHERE draft_file_url != '' UNION SELECT attachment_url FROM solicits WHERE attachment_url != '' UNION SELECT file_url FROM solicit_feedbacks WHERE file_url != ''"); err == nil {
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil && p != "" {
				clean := strings.TrimPrefix(strings.TrimPrefix(p, "/"), "uploads/")
				activeFiles[filepath.Join(absUploadDir, clean)] = true
			}
		}
		rows.Close()
	}

	// 3. 收集材料下发 dispatches 活跃文件
	if rows, err := database.DB.Query("SELECT file_url FROM dispatches WHERE file_url != '' UNION SELECT attachment_url FROM dispatches WHERE attachment_url != ''"); err == nil {
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err == nil && p != "" {
				clean := strings.TrimPrefix(strings.TrimPrefix(p, "/"), "uploads/")
				activeFiles[filepath.Join(absUploadDir, clean)] = true
			}
		}
		rows.Close()
	}

	cleanedCount := 0
	cutoff := time.Now().Add(-24 * time.Hour)

	filepath.Walk(absUploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		// 仅清理修改时间在 24 小时之前的孤儿文件（防止误删当前正在并发上传的临时文件）
		if info.ModTime().Before(cutoff) {
			absP, _ := filepath.Abs(path)
			if !activeFiles[absP] {
				if err := os.Remove(absP); err == nil {
					cleanedCount++
					log.Printf("[孤儿文件清理] 清理无引用历史残留文件: %s", absP)
				}
			}
		}
		return nil
	})

	if cleanedCount > 0 {
		log.Printf("[孤儿文件清理] 完成历史残留文件大扫除，共物理释放 %d 个孤儿文件", cleanedCount)
	}
}
