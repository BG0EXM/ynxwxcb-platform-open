package handlers

import (
	"bytes"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/secrecy"
)

// 1. 测试 resolveFilePath 安全防线：严禁目录穿越与任意绝对路径
func TestResolveFilePath_Security(t *testing.T) {
	baseDir := "/var/data/uploads"

	badPaths := []string{
		"/etc/passwd",
		"../../../../etc/passwd",
		"/uploads/../../etc/passwd",
		"/uploads/../../../shadow",
		"C:\\Windows\\System32\\cmd.exe",
		"",
		"   ",
		"/var/data/uploads/../../etc/passwd",
		"../solicits/secret.pdf",
	}

	for _, p := range badPaths {
		resolved := resolveFilePath(baseDir, p)
		if resolved != "" {
			t.Fatalf("恶意路径应当被拦截返回空字符串，但返回了: %s (输入: %s)", resolved, p)
		}
	}

	goodPath := "/uploads/solicits/2026/10/test.pdf"
	resolved := resolveFilePath(baseDir, goodPath)
	expected := filepath.Clean("/var/data/uploads/solicits/2026/10/test.pdf")
	if resolved != expected {
		t.Fatalf("合法路径期望解析为 %s，实际为 %s", expected, resolved)
	}
}

// 2. 测试 PublicSubmitFeedback 参数严格校验
func TestPublicSubmitFeedback_PathValidation(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	database.DB.Exec(`INSERT INTO solicits (id, title, doc_no, deadline, units, content, pdf_path, pdf_name, created_by)
		VALUES (101, '测试文件', '发字1号', '2030-12-31 23:59:59', '测试单位A', '正文', '/uploads/solicits/test.pdf', 'test.pdf', 1)`)

	// 测试用例 A: reply_doc_path 包含 .. 穿越
	{
		body := `{"unit":"测试单位A","reply_doc_path":"/uploads/solicits/../secret.pdf","contact_name":"张三","contact_phone":"13800000000"}`
		req := httptest.NewRequest("POST", "/api/public/solicits/101/feedback", bytes.NewBufferString(body))
		req.SetPathValue("id", "101")
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("期望 400 拦截穿越路径，实际状态码: %d", rec.Code)
		}
	}

	// 测试用例 B: reply_doc_path 非 /uploads/solicits/ 开头
	{
		body := `{"unit":"测试单位A","reply_doc_path":"/etc/passwd","contact_name":"张三","contact_phone":"13800000000"}`
		req := httptest.NewRequest("POST", "/api/public/solicits/101/feedback", bytes.NewBufferString(body))
		req.SetPathValue("id", "101")
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("期望 400 拦截非 solicits 目录路径，实际状态码: %d", rec.Code)
		}
	}

	// 测试用例 C: attachment_path 包含 .. 穿越
	{
		body := `{"unit":"测试单位A","reply_doc_path":"/uploads/solicits/ok.pdf","attachment_path":"/uploads/solicits/../evil.docx","contact_name":"张三","contact_phone":"13800000000"}`
		req := httptest.NewRequest("POST", "/api/public/solicits/101/feedback", bytes.NewBufferString(body))
		req.SetPathValue("id", "101")
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("期望 400 拦截附件穿越路径，实际状态码: %d", rec.Code)
		}
	}

	// 测试用例 D: 合法提交
	{
		body := `{"unit":"测试单位A","reply_doc_path":"/uploads/solicits/2026/10/ok.pdf","reply_doc_name":"公函.pdf","contact_name":"张三","contact_phone":"13912345678"}`
		req := httptest.NewRequest("POST", "/api/public/solicits/101/feedback", bytes.NewBufferString(body))
		req.SetPathValue("id", "101")
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("合法参数期望 200，实际状态码: %d, body: %s", rec.Code, rec.Body.String())
		}
	}
}

// 3. 测试 PublicSolicit 敏感数据脱敏与物理文件路径禁止外泄
func TestPublicSolicit_PrivacyMasking(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	database.DB.Exec(`INSERT INTO solicits (id, title, doc_no, deadline, units, content, pdf_path, pdf_name, created_by)
		VALUES (102, '保密公文', '字102', '2030-12-31 23:59:59', '单位A', '正文', '/uploads/solicits/test.pdf', 'test.pdf', 1)`)
	database.DB.Exec(`INSERT INTO solicit_feedbacks (solicit_id, unit, has_opinion, opinion_detail, reply_doc_path, reply_doc_name, attachment_path, attachment_name, contact_name, contact_phone, created_at, updated_at)
		VALUES (102, '单位A', 0, '', '/uploads/solicits/2026/10/secret_stamp.pdf', '单位A红头回函.pdf', '/uploads/solicits/2026/10/edit.docx', '修改.docx', '李干事', '13912345678', datetime('now'), datetime('now'))`)

	req := httptest.NewRequest("GET", "/api/public/solicits/102?unit=单位A", nil)
	req.SetPathValue("id", "102")
	rec := httptest.NewRecorder()
	PublicSolicit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际: %d", rec.Code)
	}

	respStr := rec.Body.String()
	if strings.Contains(respStr, "13912345678") {
		t.Fatalf("公开端泄露了未脱敏的完整手机号: %s", respStr)
	}
	if !strings.Contains(respStr, "139****5678") {
		t.Fatalf("公开端未按预期返回脱敏手机号: %s", respStr)
	}

	if strings.Contains(respStr, "/uploads/solicits/2026/10/secret_stamp.pdf") {
		t.Fatalf("公开端泄露了回函物理路径: %s", respStr)
	}
	if strings.Contains(respStr, "/uploads/solicits/2026/10/edit.docx") {
		t.Fatalf("公开端泄露了附件物理路径: %s", respStr)
	}
	if !strings.Contains(respStr, `"has_reply_doc":true`) {
		t.Fatalf("公开端未返回 has_reply_doc: true 标识: %s", respStr)
	}
}

// 4. 测试 Secrecy: 零宽字符与全角空格、换行符清洗，以及 PDF panic 恢复
func TestSecrecy_ZeroWidthAndRecover(t *testing.T) {
	evasiveText := "机\u3000密" // 全角空格
	res1, _ := secrecy.CheckFileSecrecy("test.txt", strings.NewReader(evasiveText))
	if !res1.Violated {
		t.Fatalf("全角空格混淆应当被识别为涉密违规，但未命中")
	}

	evasiveText2 := "机\n密" // 换行符
	res2, _ := secrecy.CheckFileSecrecy("test.txt", strings.NewReader(evasiveText2))
	if !res2.Violated {
		t.Fatalf("换行符混淆应当被识别为涉密违规，但未命中")
	}

	evasiveText3 := "绝\u200B密" // 零宽空格
	res3, _ := secrecy.CheckFileSecrecy("test.txt", strings.NewReader(evasiveText3))
	if !res3.Violated {
		t.Fatalf("零宽空格混淆应当被识别为涉密违规，但未命中")
	}

	// 测试畸形数据不会导致 panic
	corruptedPdf := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\nmalformed trash data ...")
	resPanic, _ := secrecy.CheckFileSecrecy("test_corrupted.pdf", bytes.NewReader(corruptedPdf))
	_ = resPanic
}

// 5. 测试 WAF: 跨换行符 SQL 注入拦截与 XSS 过滤
func TestWAF_MultiLineAndXSS(t *testing.T) {
	handler := middleware.WAF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// 测试跨换行符 SQL 注入
	multiLineBody := "select\n*\nfrom\nusers"
	req := httptest.NewRequest("POST", "/api/test", bytes.NewBufferString(multiLineBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("WAF 应当拦截跨换行符 SQL 注入，实际状态码: %d", rec.Code)
	}

	// 测试拦截页面中的 URL 路径 XSS 转义
	xssPath := "/api/<script>alert(1)</script>"
	reqXSS := httptest.NewRequest("GET", xssPath, nil)
	reqXSS.Header.Set("Accept", "text/html")
	recXSS := httptest.NewRecorder()
	handler.ServeHTTP(recXSS, reqXSS)
	if recXSS.Code != http.StatusForbidden {
		t.Fatalf("期望拦截 XSS 探测路由，实际状态码: %d", recXSS.Code)
	}
	bodyStr := recXSS.Body.String()
	if strings.Contains(bodyStr, "<script>alert(1)</script>") {
		t.Fatalf("WAF 拦截页面中存在反射型 XSS 漏洞未转义: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, html.EscapeString(xssPath)) {
		t.Fatalf("WAF 拦截页面未对路由进行 HTML Escape")
	}
}

// 6. 测试请假删除水平越权拦截
func TestAttendance_DeleteLeaveRecord_Permissions(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	database.DB.Exec(`INSERT INTO leave_records (id, user_id, leave_type, start_date, end_date, days, status)
		VALUES (1, 1, 'annual', '2026-10-10', '2026-10-10', 1, 1)`)
	database.DB.Exec(`INSERT INTO leave_records (id, user_id, leave_type, start_date, end_date, days, status)
		VALUES (2, 2, 'annual', '2026-10-11', '2026-10-11', 1, 1)`)

	// 测试 A: staff (user_id=2) 尝试删除 admin (user_id=1) 的记录 -> 必须 403 Forbidden
	{
		req := httptest.NewRequest("DELETE", "/api/leave-records/1", nil)
		req.SetPathValue("id", "1")
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		DeleteLeaveRecord(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("非所有者非管理员删除请假应当被 403 拦截，实际状态码: %d", rec.Code)
		}
	}

	// 测试 B: staff (user_id=2) 删除自己拥有 (user_id=2) 的记录 -> 应当允许 200
	{
		req := httptest.NewRequest("DELETE", "/api/leave-records/2", nil)
		req.SetPathValue("id", "2")
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		DeleteLeaveRecord(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("所有者删除自己记录应当允许，实际状态码: %d", rec.Code)
		}
	}
}

// 7. 测试附件下载 ownerID!=0 鉴权穿透修复
func TestUploads_DownloadAttachment_AccessControl(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()

	filePath := filepath.Join(cfg.Upload.Dir, "doc.pdf")
	os.WriteFile(filePath, []byte("sensitive document content"), 0644)
	database.DB.Exec(`INSERT INTO attachments (id, owner_type, owner_id, file_name, file_path, file_size, uploader_id)
		VALUES (99, 'document', 10, '重要收文.pdf', ?, 26, 1)`, filePath)

	downloadHandler := DownloadAttachment(cfg)

	// 测试 A: 另一个用户 staff (user_id=2)，没有 incoming.view 权限下载该收文附件 -> 应当 403 Forbidden
	{
		req := httptest.NewRequest("GET", "/api/uploads/99", nil)
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		downloadHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("无 incoming.view 权限下载收文附件应当被拦截，实际: %d", rec.Code)
		}
	}

	// 测试 B: 管理员 admin (user_id=1) 可以下载
	{
		req := httptest.NewRequest("GET", "/api/uploads/99", nil)
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		downloadHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("管理员下载应当成功，实际: %d", rec.Code)
		}
	}
}
