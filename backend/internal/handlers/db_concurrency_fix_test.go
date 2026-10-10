package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
)

func setupDBConcurrencyTestEnv(t *testing.T) (*config.Config, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "ynxwxcb_db_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	uploadDir := filepath.Join(tmpDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		t.Fatalf("创建上传目录失败: %v", err)
	}

	cfg := config.Default()
	cfg.Database.Path = dbPath
	cfg.Upload.Dir = uploadDir

	if err := database.Init(dbPath); err != nil {
		t.Fatalf("数据库初始化失败: %v", err)
	}

	cleanup := func() {
		if database.DB != nil {
			database.DB.Close()
		}
		os.RemoveAll(tmpDir)
	}

	return cfg, cleanup
}

// 1. 测试 CleanupOrphanFiles：白名单正确保留各业务表附件，且数据库异常时熔断保护
func TestCleanupOrphanFiles_WhitelistAndCircuitBreaker(t *testing.T) {
	cfg, cleanup := setupDBConcurrencyTestEnv(t)
	defer cleanup()

	// 准备过去 25 小时的测试文件
	pastTime := time.Now().Add(-25 * time.Hour)

	// 1. solicit pdf 与 word
	solicitPDF := filepath.Join(cfg.Upload.Dir, "solicit_doc.pdf")
	solicitWord := filepath.Join(cfg.Upload.Dir, "solicit_draft.docx")
	os.WriteFile(solicitPDF, []byte("pdf-content"), 0644)
	os.WriteFile(solicitWord, []byte("word-content"), 0644)
	os.Chtimes(solicitPDF, pastTime, pastTime)
	os.Chtimes(solicitWord, pastTime, pastTime)

	// 2. solicit_feedbacks reply 与 attachment
	feedbackReply := filepath.Join(cfg.Upload.Dir, "feedback_reply.pdf")
	feedbackAttach := filepath.Join(cfg.Upload.Dir, "feedback_attach.docx")
	os.WriteFile(feedbackReply, []byte("reply-content"), 0644)
	os.WriteFile(feedbackAttach, []byte("attach-content"), 0644)
	os.Chtimes(feedbackReply, pastTime, pastTime)
	os.Chtimes(feedbackAttach, pastTime, pastTime)

	// 3. dispatches pdf 与 attachment
	dispatchPDF := filepath.Join(cfg.Upload.Dir, "dispatch_doc.pdf")
	dispatchAttach := filepath.Join(cfg.Upload.Dir, "dispatch_attach.zip")
	os.WriteFile(dispatchPDF, []byte("dispatch-pdf"), 0644)
	os.WriteFile(dispatchAttach, []byte("dispatch-attach"), 0644)
	os.Chtimes(dispatchPDF, pastTime, pastTime)
	os.Chtimes(dispatchAttach, pastTime, pastTime)

	// 4. 真正没有引用的孤儿文件
	orphanFile := filepath.Join(cfg.Upload.Dir, "orphan_file.txt")
	os.WriteFile(orphanFile, []byte("orphan"), 0644)
	os.Chtimes(orphanFile, pastTime, pastTime)

	// 写入数据库记录（相对路径或虚拟路径）
	_, err := database.DB.Exec(`INSERT INTO solicits (id, title, deadline, units, pdf_path, pdf_name, word_path, word_name)
		VALUES (1, '征求意见测试', '2026-10-15', '各科室', '/uploads/solicit_doc.pdf', 'solicit_doc.pdf', 'solicit_draft.docx', 'solicit_draft.docx')`)
	if err != nil {
		t.Fatalf("插入 solicits 失败: %v", err)
	}

	_, err = database.DB.Exec(`INSERT INTO solicit_feedbacks (solicit_id, unit, reply_doc_path, reply_doc_name, attachment_path, attachment_name, contact_name, contact_phone)
		VALUES (1, '办公室', 'feedback_reply.pdf', 'feedback_reply.pdf', '/uploads/feedback_attach.docx', 'feedback_attach.docx', '张三', '13800000000')`)
	if err != nil {
		t.Fatalf("插入 solicit_feedbacks 失败: %v", err)
	}

	_, err = database.DB.Exec(`INSERT INTO dispatches (id, title, units, pdf_path, pdf_name, attachment_path, attachment_name)
		VALUES (1, '材料下发测试', '各科室', '/uploads/dispatch_doc.pdf', 'dispatch_doc.pdf', 'dispatch_attach.zip', 'dispatch_attach.zip')`)
	if err != nil {
		t.Fatalf("插入 dispatches 失败: %v", err)
	}

	// 正常运行 CleanupOrphanFiles
	CleanupOrphanFiles(cfg)

	// 业务引用的 6 个文件必须全部保留
	checkFiles := []string{solicitPDF, solicitWord, feedbackReply, feedbackAttach, dispatchPDF, dispatchAttach}
	for _, f := range checkFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("白名单业务文件被错误删除: %s", f)
		}
	}

	// 真正的孤儿文件应当被清理
	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Errorf("孤儿文件未被清理: %s", orphanFile)
	}

	// 5. 测试熔断安全机制：如果数据库连接关闭，严禁删除任何文件
	newOrphan := filepath.Join(cfg.Upload.Dir, "new_orphan.txt")
	os.WriteFile(newOrphan, []byte("new-orphan"), 0644)
	os.Chtimes(newOrphan, pastTime, pastTime)

	// 关闭数据库模拟 DB 查询失败
	database.DB.Close()

	// 此时调用 CleanupOrphanFiles 必须触发熔断保护退出，严禁删除 newOrphan
	CleanupOrphanFiles(cfg)

	if _, err := os.Stat(newOrphan); os.IsNotExist(err) {
		t.Errorf("熔断防护失效：数据库异常时孤儿文件被意外删除！")
	}
}

// 2. 测试 DeleteDispatch 事务包裹与级联删除
func TestDeleteDispatch_TransactionWrap(t *testing.T) {
	cfg, cleanup := setupDBConcurrencyTestEnv(t)
	defer cleanup()

	// 准备文件
	pdfPath := filepath.Join(cfg.Upload.Dir, "dispatch_del.pdf")
	os.WriteFile(pdfPath, []byte("pdf"), 0644)

	// 插入材料下发及查收记录
	database.DB.Exec(`INSERT INTO dispatches (id, title, units, pdf_path, pdf_name)
		VALUES (88, '要删除的通知', '全部门', '/uploads/dispatch_del.pdf', 'dispatch_del.pdf')`)
	database.DB.Exec(`INSERT INTO dispatch_receipts (id, dispatch_id, unit, receiver_name, receiver_phone)
		VALUES (101, 88, '宣传科', '王科长', '13900000000')`)

	handler := DeleteDispatch(cfg)

	req := httptest.NewRequest("DELETE", "/api/dispatches/88", nil)
	req.SetPathValue("id", "88")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("删除材料下发期望 200，实际: %d", rec.Code)
	}

	var dCount, rCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM dispatches WHERE id = 88").Scan(&dCount)
	database.DB.QueryRow("SELECT COUNT(*) FROM dispatch_receipts WHERE dispatch_id = 88").Scan(&rCount)
	if dCount != 0 || rCount != 0 {
		t.Errorf("DeleteDispatch 未完全在事务中清理记录: dCount=%d, rCount=%d", dCount, rCount)
	}

	if _, err := os.Stat(pdfPath); !os.IsNotExist(err) {
		t.Errorf("DeleteDispatch 未清理物理附件: %s", pdfPath)
	}
}

