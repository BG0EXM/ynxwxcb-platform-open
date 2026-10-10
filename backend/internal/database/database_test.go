package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitFreshDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ynxwxcb_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "fresh.db")
	if err := Init(dbPath); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	defer DB.Close()

	// 检查角色数量
	var roleCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM roles").Scan(&roleCount); err != nil {
		t.Fatalf("查询 roles 失败: %v", err)
	}
	t.Logf("roles 数量: %d", roleCount)

	expectedRoles := []string{"admin", "leader", "staff", "reporter"}
	for _, code := range expectedRoles {
		var cnt int
		DB.QueryRow("SELECT COUNT(*) FROM roles WHERE code = ?", code).Scan(&cnt)
		if cnt != 1 {
			t.Errorf("角色 %s 数量期望为 1，实际为 %d", code, cnt)
		}
	}

	// 检查管理员 role_id
	var adminRoleID int64
	if err := DB.QueryRow("SELECT role_id FROM users WHERE username = 'admin'").Scan(&adminRoleID); err != nil {
		t.Fatalf("查询 admin user 失败: %v", err)
	}
	t.Logf("admin role_id: %d", adminRoleID)
	if adminRoleID == 0 {
		t.Errorf("admin 的 role_id 异常为 0")
	}

	// 检查 role_permissions 记录数
	var permCount int
	if err := DB.QueryRow("SELECT COUNT(*) FROM role_permissions").Scan(&permCount); err != nil {
		t.Fatalf("查询 role_permissions 失败: %v", err)
	}
	t.Logf("role_permissions 数量: %d", permCount)
	if permCount == 0 {
		t.Errorf("role_permissions 异常为空")
	}

	// 检查 PRAGMA 参数是否生效
	var journalMode string
	if err := DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
		t.Errorf("PRAGMA journal_mode 期望为 wal，实际为 %s, err: %v", journalMode, err)
	}
	var busyTimeout int
	if err := DB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil || busyTimeout != 5000 {
		t.Errorf("PRAGMA busy_timeout 期望为 5000，实际为 %d, err: %v", busyTimeout, err)
	}

	// 检查新增高频索引
	expectedIndexes := []string{
		"idx_users_dept",
		"idx_users_role",
		"idx_circ_user",
		"idx_study_cat",
		"idx_incoming_dept",
		"idx_att_composite",
	}
	for _, idxName := range expectedIndexes {
		var cnt int
		if err := DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idxName).Scan(&cnt); err != nil || cnt == 0 {
			t.Errorf("缺少高频索引: %s (cnt=%d, err=%v)", idxName, cnt, err)
		}
	}
}
