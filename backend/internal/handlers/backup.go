package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// BackupItem 备份文件信息
type BackupItem struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	SizeText  string `json:"size_text"`
	CreatedAt string `json:"created_at"`
}

// formatBytes 格式化文件大小
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// getBackupDir 获取备份存放目录
func getBackupDir(cfg *config.Config) string {
	dbDir := filepath.Dir(cfg.Database.Path)
	return filepath.Join(dbDir, "backups")
}

// ListBackups 获取系统数据库备份列表（仅管理员）
func ListBackups(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backupDir := getBackupDir(cfg)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "无法访问备份目录"})
			return
		}

		entries, err := os.ReadDir(backupDir)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取备份列表失败"})
			return
		}

		items := make([]BackupItem, 0)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, BackupItem{
				Filename:  entry.Name(),
				SizeBytes: info.Size(),
				SizeText:  formatBytes(info.Size()),
				CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}

		// 按修改时间倒序排列（最新的在最前）
		sort.Slice(items, func(i, j int) bool {
			return items[i].CreatedAt > items[j].CreatedAt
		})

		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"list": items,
		})
	}
}

// CreateBackup 执行数据库在线快照备份（仅管理员）
func CreateBackup(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backupDir := getBackupDir(cfg)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "创建备份目录失败"})
			return
		}

		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("ynxwxcb_backup_%s.db", timestamp)
		backupPath := filepath.Join(backupDir, filename)

		// SQLite 在线备份最佳实践：VACUUM INTO 'target_path'
		// modernc.org/sqlite 支持标准 VACUUM INTO 语法，生成事务一致性且经过碎片整理的独立快照
		// 文件路径使用绝对路径避免工作目录差异
		absBackupPath, err := filepath.Abs(backupPath)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "解析备份路径失败"})
			return
		}

		// 安全防护：转义单引号
		safePath := strings.ReplaceAll(absBackupPath, "'", "''")
		_, err = database.DB.Exec(fmt.Sprintf("VACUUM INTO '%s'", safePath))
		if err != nil {
			// 若 VACUUM INTO 失败（如极端权限或驱动版本），回退到安全文件流拷贝
			copyErr := copyFile(cfg.Database.Path, absBackupPath)
			if copyErr != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "数据库快照创建失败: " + err.Error()})
				return
			}
		}

		info, _ := os.Stat(absBackupPath)
		var size int64
		if info != nil {
			size = info.Size()
		}

		logOperation(r, "系统管理", "备份", fmt.Sprintf("创建数据库快照「%s」（%s）", filename, formatBytes(size)))

		middleware.JSON(w, http.StatusOK, map[string]interface{}{
			"message":    "数据库备份成功",
			"filename":   filename,
			"size_bytes": size,
			"size_text":  formatBytes(size),
		})
	}
}

// DownloadBackup 下载指定的数据库备份文件（仅管理员）
func DownloadBackup(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "缺少备份文件名"})
			return
		}

		// 安全防范：路径穿越校验，仅允许合法文件名
		cleanName := filepath.Base(filename)
		if cleanName != filename || !strings.HasSuffix(cleanName, ".db") {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "非法的文件名"})
			return
		}

		backupDir := getBackupDir(cfg)
		fullPath := filepath.Join(backupDir, cleanName)

		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "备份文件不存在"})
			return
		}

		f, err := os.Open(fullPath)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "打开备份文件失败"})
			return
		}
		defer f.Close()

		logOperation(r, "系统管理", "下载", fmt.Sprintf("下载数据库备份「%s」", cleanName))

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", cleanName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		io.Copy(w, f)
	}
}

// copyFile 文件安全复制辅助函数
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
