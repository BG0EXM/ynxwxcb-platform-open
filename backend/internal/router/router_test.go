package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
)

func TestUploads_RouterProtection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ynxwxcb_router_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	uploadDir := filepath.Join(tmpDir, "uploads")
	os.MkdirAll(uploadDir, 0755)

	cfg := config.Default()
	cfg.Database.Path = dbPath
	cfg.JWT.Secret = "test-jwt-secret-string-at-least-32-chars-long!"
	cfg.Upload.Dir = uploadDir

	auth.Init(cfg.JWT.Secret)
	if err := database.Init(dbPath); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.DB.Close()

	hash, _ := database.HashPassword("123456")
	database.DB.Exec(`INSERT OR REPLACE INTO users (id, username, password_hash, real_name, department_id, role_id, status, token_version)
		VALUES (1, 'admin', ?, '系统管理员', 1, 1, 1, 0)`, hash)

	filePath := filepath.Join(cfg.Upload.Dir, "secret.pdf")
	os.WriteFile(filePath, []byte("confidential content"), 0644)

	r := NewRouter(cfg)

	// 测试 A: 未带 Token 直接 GET /uploads/secret.pdf -> 必须 401
	{
		req := httptest.NewRequest("GET", "/uploads/secret.pdf", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("未认证请求访问 /uploads/ 目录必须返回 401，实际: %d", rec.Code)
		}
	}

	// 测试 B: 带合法 Token 访问 -> 允许 200
	{
		token, _ := auth.GenerateToken(1, "admin", "系统管理员", "admin", 0)
		req := httptest.NewRequest("GET", "/uploads/secret.pdf", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("带合法 Token 访问应当返回 200，实际: %d", rec.Code)
		}
	}

	// 测试 C: 带 query token 访问 -> 允许 200
	{
		token, _ := auth.GenerateToken(1, "admin", "系统管理员", "admin", 0)
		req := httptest.NewRequest("GET", "/uploads/secret.pdf?token="+token, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("带 query token 访问应当返回 200，实际: %d", rec.Code)
		}
	}
}
