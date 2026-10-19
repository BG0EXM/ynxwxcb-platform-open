package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// 附件路径统一规范：
//   - attachments.file_path 存“相对上传目录”的路径（如 2026/10/xxx.pdf），迁移目录、相对/绝对 upload.dir 均不受影响；
//   - 业务表引用使用 /api/uploads/{id} 或 /uploads/<相对路径>；
//   - 下载、预览、打包、删除、孤儿清扫全部只经由 resolveFilePath 解析，杜绝多套解析口径不一致导致误删。

// uploadBase 返回上传目录的绝对规范路径
func uploadBase(baseDir string) string {
	if baseDir == "" {
		return ""
	}
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return filepath.Clean(baseDir)
	}
	return abs
}

// withinBase 判断 target 是否严格位于 base 目录之内（不含 base 本身）
func withinBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// attachmentStoredPath 按 /api/uploads/{id} 反查附件表中存储的路径
func attachmentStoredPath(ref string) (string, bool) {
	idStr := strings.TrimPrefix(ref, "/api/uploads/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return "", false
	}
	var p string
	if err := database.DB.QueryRow("SELECT file_path FROM attachments WHERE id=?", id).Scan(&p); err != nil {
		return "", false
	}
	return p, true
}

// resolveFilePath 将任意形式的存储路径解析为上传目录内的绝对路径，非法或越界返回空字符串。
// 兼容：/api/uploads/{id}、/uploads/<rel>、相对上传目录的路径、历史遗留的“相对工作目录”路径与绝对路径，
// 以及数据恢复到其他目录后的旧绝对路径（按尾部路径段在当前上传目录内回查）。
func resolveFilePath(baseDir, rawPath string) string {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" || baseDir == "" {
		return ""
	}
	if strings.HasPrefix(rawPath, "/api/uploads/") {
		p, ok := attachmentStoredPath(rawPath)
		if !ok {
			return ""
		}
		rawPath = strings.TrimSpace(p)
		if rawPath == "" || strings.HasPrefix(rawPath, "/api/uploads/") {
			return ""
		}
	}
	// 严禁包含反斜杠或冒号（防止跨平台路径混淆或 Windows 绝对路径绕过）
	if strings.Contains(rawPath, "\\") || strings.Contains(rawPath, ":") {
		return ""
	}

	base := uploadBase(baseDir)

	// 1. Web 路径 /uploads/<rel>
	if strings.HasPrefix(rawPath, "/uploads/") {
		t := filepath.Join(base, strings.TrimPrefix(rawPath, "/uploads/"))
		if withinBase(base, t) {
			return t
		}
		return ""
	}

	// 2. 绝对路径：在上传目录内直接采用；否则视为旧目录遗留，按尾部路径段回查当前上传目录
	if filepath.IsAbs(rawPath) {
		t := filepath.Clean(rawPath)
		if withinBase(base, t) {
			return t
		}
		return relocateBySuffix(base, t)
	}

	// 3. 相对路径：优先按“相对上传目录”解析
	t := filepath.Join(base, rawPath)
	if !withinBase(base, t) {
		return ""
	}
	if fileExists(t) {
		return t
	}
	// 历史遗留：旧版本把 data/uploads/2026/10/x.pdf 这类“相对工作目录”的保存路径直接入库
	if legacy, err := filepath.Abs(rawPath); err == nil && withinBase(base, legacy) && fileExists(legacy) {
		return legacy
	}
	if moved := relocateBySuffix(base, filepath.Clean("/"+rawPath)); moved != "" {
		return moved
	}
	return t
}

// relocateBySuffix 对不在当前上传目录内的旧路径，按尾部路径段（由长到短）在上传目录中查找真实存在的文件
func relocateBySuffix(base, p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	var segs []string
	for _, s := range parts {
		if s != "" {
			segs = append(segs, s)
		}
	}
	for i := 0; i < len(segs); i++ {
		if segs[i] == ".." {
			continue
		}
		t := filepath.Join(append([]string{base}, segs[i:]...)...)
		if withinBase(base, t) && fileExists(t) {
			return t
		}
	}
	return ""
}

// relToBase 返回相对上传目录的斜杠路径
func relToBase(base, abs string) string {
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// NormalizeAttachmentPaths 启动时把 attachments.file_path 统一改写为“相对上传目录”的路径（幂等，可重复执行）。
// 必须在孤儿文件清扫之前调用。
func NormalizeAttachmentPaths(cfg *config.Config) {
	if cfg == nil || cfg.Upload.Dir == "" {
		return
	}
	base := uploadBase(cfg.Upload.Dir)
	rows, err := database.DB.Query("SELECT id, file_path FROM attachments")
	if err != nil {
		log.Printf("[附件路径规范化] 查询失败: %v", err)
		return
	}
	type fix struct {
		id  int64
		rel string
	}
	var fixes []fix
	for rows.Next() {
		var id int64
		var p sql.NullString
		if err := rows.Scan(&id, &p); err != nil {
			continue
		}
		stored := strings.TrimSpace(p.String)
		if stored == "" || strings.HasPrefix(stored, "/api/uploads/") {
			continue
		}
		abs := resolveFilePath(cfg.Upload.Dir, stored)
		if abs == "" || !fileExists(abs) {
			continue
		}
		if rel := relToBase(base, abs); rel != "" && rel != stored {
			fixes = append(fixes, fix{id, rel})
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[查询] 遍历结果失败: %v", err)
	}
	rows.Close()
	if len(fixes) == 0 {
		return
	}
	tx, err := database.DB.Begin()
	if err != nil {
		log.Printf("[附件路径规范化] 开启事务失败: %v", err)
		return
	}
	defer tx.Rollback()
	for _, f := range fixes {
		if _, err := tx.Exec("UPDATE attachments SET file_path=? WHERE id=?", f.rel, f.id); err != nil {
			log.Printf("[附件路径规范化] 更新附件 %d 失败: %v", f.id, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("[附件路径规范化] 提交失败: %v", err)
		return
	}
	log.Printf("[附件路径规范化] 已将 %d 条附件路径统一为相对上传目录格式", len(fixes))
}

// checkOwnedUploadRef 校验业务表中的文件引用，杜绝借业务字段公开或删除他人附件：
//   - 只接受 /api/uploads/{id}；
//   - 附件上传时声明的 owner_type 必须与业务类型一致；
//   - 必须为本人上传（管理员除外），且未被其他业务记录占用；
//   - keep 为当前记录已有的引用（编辑时原样保留，不再重复校验）。
//
// 返回附件 ID（空引用或保留原值时为 0）与中文错误提示。
func checkOwnedUploadRef(r *http.Request, ref, ownerType string, ownerID int64, keep ...string) (int64, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, ""
	}
	for _, k := range keep {
		if k != "" && ref == strings.TrimSpace(k) {
			return 0, ""
		}
	}
	if !strings.HasPrefix(ref, "/api/uploads/") {
		return 0, "文件引用格式非法，请重新上传"
	}
	attID, err := strconv.ParseInt(strings.TrimPrefix(ref, "/api/uploads/"), 10, 64)
	if err != nil || attID <= 0 {
		return 0, "文件引用格式非法，请重新上传"
	}
	var uploaderID, curOwner int64
	var curType string
	if err := database.DB.QueryRow("SELECT uploader_id, owner_type, owner_id FROM attachments WHERE id=?", attID).
		Scan(&uploaderID, &curType, &curOwner); err != nil {
		return 0, "引用的文件不存在，请重新上传"
	}
	if curType != ownerType {
		return 0, "引用的文件不属于本业务，请重新上传"
	}
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	if roleCode != "admin" && uploaderID != userID {
		return 0, "无权引用他人上传的文件"
	}
	if curOwner != 0 && curOwner != ownerID {
		return 0, "该文件已被其他事项占用，请重新上传"
	}
	return attID, ""
}

// bindUploadRefs 将附件绑定到业务记录（owner_id），使其不能再被其他记录引用
func bindUploadRefs(ownerID int64, attIDs ...int64) {
	for _, id := range attIDs {
		if id > 0 {
			database.DB.Exec("UPDATE attachments SET owner_id=? WHERE id=?", ownerID, id)
		}
	}
}

// validDeadline 截止时间格式校验（空值视为合法，由调用方决定是否必填）
func validDeadline(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, l := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if _, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return true
		}
	}
	return false
}

// newPublicToken 生成 128 位随机公开链接令牌（32 位十六进制）
func newPublicToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("生成随机令牌失败: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// publicID 按公开链接令牌解析业务记录 ID；旧版数字 ID 链接一律视为失效
func publicID(w http.ResponseWriter, r *http.Request, table string) (int64, bool) {
	token := strings.TrimSpace(r.PathValue("token"))
	if len(token) != 32 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "链接无效或已失效，请向发起单位索取最新链接"})
		return 0, false
	}
	var id int64
	var q string
	switch table {
	case "meetings":
		q = "SELECT id FROM meetings WHERE public_token=?"
	case "solicits":
		q = "SELECT id FROM solicits WHERE public_token=?"
	case "dispatches":
		q = "SELECT id FROM dispatches WHERE public_token=?"
	default:
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "链接无效"})
		return 0, false
	}
	if err := database.DB.QueryRow(q, token).Scan(&id); err != nil || id == 0 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "链接无效或已失效，请向发起单位索取最新链接"})
		return 0, false
	}
	return id, true
}
