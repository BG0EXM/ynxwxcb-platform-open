package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/handlers"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/router"
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

	// 确保上传目录存在
	os.MkdirAll(cfg.Upload.Dir, 0755)

	// 操作日志：启动清理一次，之后每天清理超过 1 年的记录
	handlers.CleanupOldLogs()
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			handlers.CleanupOldLogs()
		}
	}()

	// 构建路由，并套上安全响应头 + 请求体大小限制（上传接口自行限制，故跳过）
	r := router.NewRouter(cfg)
	handler := middleware.SecurityHeaders(middleware.LimitBody(10<<20, "/api/uploads")(r))

	addr := ":" + cfg.Server.Port
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

// applyAdmin 用配置的管理员信息创建/更新管理员账号
// 密码非空即生效，用户名缺省时使用默认 admin，方便通过 config.json 修改管理员密码
func applyAdmin(cfg *config.Config) {
	if cfg.Admin.Password == "" {
		return
	}
	username := cfg.Admin.Username
	if username == "" {
		username = "admin"
	}
	hash, err := database.HashPassword(cfg.Admin.Password)
	if err != nil {
		log.Printf("加密管理员密码失败: %v", err)
		return
	}
	_, err = database.DB.Exec(
		`INSERT INTO users (username, password_hash, real_name, phone, department_id, role_id, status)
		 VALUES (?, ?, '系统管理员', '', 1, 1, 1)
		 ON CONFLICT(username) DO UPDATE SET password_hash = ?`,
		username, hash, hash)
	if err != nil {
		log.Printf("配置管理员账号失败: %v", err)
	}
}
