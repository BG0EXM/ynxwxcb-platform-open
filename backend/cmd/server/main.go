package main

import (
	// 内嵌时区数据库：即使服务器未安装 tzdata，TZ=Asia/Shanghai 也能正确生效
	"database/sql"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/handlers"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/router"
	"ynxwxcb-platform/internal/secrecy"
)

func main() {
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 若配置文件不存在则创建默认配置
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		if err := cfg.Save(*configPath); err != nil {
			log.Fatalf("创建默认配置失败: %v", err)
		}
		log.Printf("已生成默认配置文件 %s，请修改 JWT 密钥和管理员密码", *configPath)
	}

	// 安全校验：拒绝默认/占位/过短的 JWT 密钥，避免被伪造令牌
	upper := strings.ToUpper(cfg.JWT.Secret)
	if len(cfg.JWT.Secret) < 32 || strings.Contains(upper, "CHANGE") || strings.Contains(upper, "PLACEHOLDER") || strings.Contains(upper, "CHANGEME") {
		log.Fatalf("JWT 密钥不安全：请在 config.json 的 jwt.secret 填入至少 32 位的随机字符串（可用 `openssl rand -base64 48` 生成）")
	}

	// 初始化 JWT
	auth.Init(cfg.JWT.Secret)
	handlers.SetAISecret(cfg.JWT.Secret) // AI 接口密钥加密根密钥

	// 初始化数据库
	if err := database.Init(cfg.Database.Path); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 加载角色-权限缓存
	middleware.ReloadPermissions()
	// 客户端 IP 是否信任 X-Forwarded-For（前置 Caddy/WAF 时配置为 true）
	middleware.SetTrustProxy(cfg.Server.TrustProxy)

	// 应用配置中的管理员账号覆盖默认管理员
	applyAdmin(cfg)

	// 启动自检保密安全与 OCR 审查引擎
	secrecy.InitOCR()

	// 上传目录统一转为绝对路径（相对路径按启动工作目录解析），并确保目录存在
	if absDir, err := filepath.Abs(cfg.Upload.Dir); err == nil {
		cfg.Upload.Dir = absDir
	}
	if err := os.MkdirAll(cfg.Upload.Dir, 0755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}
	log.Printf("上传目录: %s", cfg.Upload.Dir)
	// 附件路径统一规范化为“相对上传目录”格式（必须在孤儿文件清扫之前执行）
	handlers.NormalizeAttachmentPaths(cfg)

	// 操作日志：启动清理一次，之后每天清理超过 1 年的记录
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[后台任务异常] 初始化清理操作日志发生 panic: %v", r)
			}
		}()
		handlers.CleanupOldLogs()
	}()
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[后台任务异常] 定时清理操作日志发生 panic: %v", r)
					}
				}()
				handlers.CleanupOldLogs()
			}()
		}
	}()

	// 磁盘孤儿残留文件大扫除：启动扫描清理历史无引用残留文件，之后每天例行清理一次
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[后台任务异常] 初始化清理孤儿文件发生 panic: %v", r)
			}
		}()
		handlers.CleanupOrphanFiles(cfg)
	}()
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[后台任务异常] 定时清理孤儿文件发生 panic: %v", r)
					}
				}()
				handlers.CleanupOrphanFiles(cfg)
			}()
		}
	}()

	// 构建路由，并套上安全响应头 + 请求体大小限制（上传接口自行限制，故跳过）
	r := router.NewRouter(cfg)
	handler := middleware.SecurityHeaders(middleware.WAF(middleware.LimitBody(10<<20, middleware.IsUploadRequest)(r)))

	addr := net.JoinHostPort(cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      180 * time.Second, // 导出大文件留足时间
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("伊宁县委宣传部部务工作平台已启动，监听 %s", addr)
	log.Printf("数据库: %s", cfg.Database.Path)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// applyAdmin 用配置的管理员信息创建管理员账号：
//   - 账号不存在时按配置创建（角色按 code='admin' 查询，不写死 ID）；
//   - 账号已存在时默认不覆盖界面上修改过的密码，仅当 force_reset=true 时重置并使旧令牌失效。
func applyAdmin(cfg *config.Config) {
	if cfg.Admin.Password == "" {
		return
	}
	username := cfg.Admin.Username
	if username == "" {
		username = "admin"
	}
	if len(cfg.Admin.Password) > 72 {
		log.Printf("配置的管理员密码超过 72 字节，已忽略")
		return
	}
	var adminRoleID int64
	if err := database.DB.QueryRow("SELECT id FROM roles WHERE code='admin'").Scan(&adminRoleID); err != nil {
		log.Printf("未找到管理员角色，跳过配置管理员账号: %v", err)
		return
	}
	var userID int64
	err := database.DB.QueryRow("SELECT id FROM users WHERE username=?", username).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("查询管理员账号失败: %v", err)
		return
	}
	if err == nil && !cfg.Admin.ForceReset {
		return
	}
	hash, err := database.HashPassword(cfg.Admin.Password)
	if err != nil {
		log.Printf("加密管理员密码失败: %v", err)
		return
	}
	if userID == 0 {
		var deptID int64
		database.DB.QueryRow("SELECT id FROM departments ORDER BY id LIMIT 1").Scan(&deptID)
		if _, err := database.DB.Exec(
			`INSERT INTO users (username, password_hash, real_name, phone, department_id, role_id, status)
			 VALUES (?, ?, '系统管理员', '', ?, ?, 1)`, username, hash, deptID, adminRoleID); err != nil {
			log.Printf("创建配置管理员账号失败: %v", err)
		}
		return
	}
	if _, err := database.DB.Exec(
		"UPDATE users SET password_hash=?, role_id=?, status=1, token_version=token_version+1 WHERE id=?",
		hash, adminRoleID, userID); err != nil {
		log.Printf("重置管理员密码失败: %v", err)
		return
	}
	middleware.InvalidateTokenVersion(userID)
	log.Printf("已按 config.json（force_reset=true）重置管理员 %s 的密码，请尽快将 force_reset 改回 false", username)
}
