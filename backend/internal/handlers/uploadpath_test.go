package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ynxwxcb-platform/internal/database"
)

func writeOldFile(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(p, past, past)
}

// 相对 upload.dir 下历史遗留的“相对工作目录”路径、旧绝对路径、新相对路径都必须被识别为活跃文件，绝不能被清扫
func TestCleanupOrphanFiles_LegacyAndRelocatedPaths(t *testing.T) {
	cfg, cleanup := setupDBConcurrencyTestEnv(t)
	defer cleanup()

	// 以临时目录为工作目录，模拟 upload.dir = "uploads"（相对路径）
	root := filepath.Dir(cfg.Upload.Dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	cfg.Upload.Dir = "uploads"
	base := filepath.Join(root, "uploads")

	legacyRel := filepath.Join(base, "2026/10/1_legacy.pdf") // 存为 uploads/2026/10/1_legacy.pdf（相对工作目录）
	movedAbs := filepath.Join(base, "2026/09/2_moved.pdf")   // 存为旧服务器绝对路径
	newRel := filepath.Join(base, "2026/10/3_new.pdf")       // 存为 2026/10/3_new.pdf（新格式）
	orphan := filepath.Join(base, "2026/10/4_orphan.pdf")    // 无引用
	for _, p := range []string{legacyRel, movedAbs, newRel, orphan} {
		writeOldFile(t, p)
	}
	ins := func(fp string) {
		if _, err := database.DB.Exec("INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES ('study', 1, 'f', ?, 1, 1)", fp); err != nil {
			t.Fatal(err)
		}
	}
	ins("uploads/2026/10/1_legacy.pdf")
	ins("/opt/old-server/data/uploads/2026/09/2_moved.pdf")
	ins("2026/10/3_new.pdf")

	for _, p := range []string{"uploads/2026/10/1_legacy.pdf", "/opt/old-server/data/uploads/2026/09/2_moved.pdf", "2026/10/3_new.pdf"} {
		if got := resolveFilePath(cfg.Upload.Dir, p); got == "" || !fileExists(got) {
			t.Fatalf("路径 %s 未能解析到真实文件，得到 %q", p, got)
		}
	}

	NormalizeAttachmentPaths(cfg)
	rows, _ := database.DB.Query("SELECT file_path FROM attachments ORDER BY id")
	var got []string
	for rows.Next() {
		var p string
		rows.Scan(&p)
		got = append(got, p)
	}
	rows.Close()
	want := []string{"2026/10/1_legacy.pdf", "2026/09/2_moved.pdf", "2026/10/3_new.pdf"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("规范化后路径期望 %v，实际 %v", want, got)
		}
	}

	CleanupOrphanFiles(cfg)
	for _, p := range []string{legacyRel, movedAbs, newRel} {
		if !fileExists(p) {
			t.Errorf("被引用的附件被误清理: %s", p)
		}
	}
	if fileExists(orphan) {
		t.Errorf("孤儿文件未被清理: %s", orphan)
	}
	if !fileExists(filepath.Join(base, trashDirName, time.Now().Format("20060102"), "2026/10/4_orphan.pdf")) {
		t.Errorf("孤儿文件应移入隔离区而非直接删除")
	}
}

// 大量引用解析失败时必须整轮熔断
func TestCleanupOrphanFiles_DanglingBreaker(t *testing.T) {
	cfg, cleanup := setupDBConcurrencyTestEnv(t)
	defer cleanup()
	for i := 0; i < 12; i++ {
		database.DB.Exec("INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES ('study', 1, 'f', ?, 1, 1)", "missing/"+string(rune('a'+i))+".pdf")
	}
	orphan := filepath.Join(cfg.Upload.Dir, "2026/10/x.pdf")
	writeOldFile(t, orphan)
	CleanupOrphanFiles(cfg)
	if !fileExists(orphan) {
		t.Fatal("引用大面积失效时应熔断，不得清理任何文件")
	}
}
