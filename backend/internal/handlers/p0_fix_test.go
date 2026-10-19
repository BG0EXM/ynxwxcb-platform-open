package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// 删除公共资料不得连带删除同 ID 收文的附件
func TestDeleteStudyMaterial_DoesNotTouchIncomingAttachments(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()

	database.DB.Exec("INSERT INTO study_materials (id, title, content, category, publisher_id) VALUES (5, '学习资料', '', '其他', 1)")
	docFile := filepath.Join(cfg.Upload.Dir, "2026/10/doc5.pdf")
	studyFile := filepath.Join(cfg.Upload.Dir, "2026/10/study5.pdf")
	writeOldFile(t, docFile)
	writeOldFile(t, studyFile)
	database.DB.Exec("INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES ('document', 5, 'doc5.pdf', '2026/10/doc5.pdf', 1, 1)")
	database.DB.Exec("INSERT INTO attachments (owner_type, owner_id, file_name, file_path, file_size, uploader_id) VALUES ('study', 5, 'study5.pdf', '2026/10/study5.pdf', 1, 1)")

	req := httptest.NewRequest("DELETE", "/api/study-materials/5", nil)
	req.SetPathValue("id", "5")
	req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
	rec := httptest.NewRecorder()
	DeleteStudyMaterial(cfg)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除失败: %d %s", rec.Code, rec.Body.String())
	}
	if !fileExists(docFile) {
		t.Fatal("收文附件文件被误删")
	}
	var n int
	database.DB.QueryRow("SELECT COUNT(*) FROM attachments WHERE owner_type='document' AND owner_id=5").Scan(&n)
	if n != 1 {
		t.Fatal("收文附件记录被误删")
	}
	if fileExists(studyFile) {
		t.Fatal("公共资料自身附件未被清理")
	}
}

// 征求意见/材料下发不得引用他人附件、其他业务附件或任意路径
func TestSolicitDispatch_RejectForeignFileRefs(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='staff') WHERE id=2")

	adminDoc := insertTestAttachment(t, "document", 1, "secret.pdf")
	adminSolicit := insertTestAttachment(t, "solicit", 1, "admin.pdf")
	own := insertTestAttachment(t, "solicit", 2, "own.pdf")

	post := func(ref string) int {
		body := `{"title":"t","deadline":"2026-12-31 18:00","units":"县委办","pdf_path":"` + ref + `","pdf_name":"a.pdf"}`
		req := httptest.NewRequest("POST", "/api/solicits", bytes.NewBufferString(body))
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		CreateSolicit(cfg)(rec, req)
		return rec.Code
	}
	for _, bad := range []string{adminDoc, adminSolicit, "/uploads/2026/10/secret.pdf", "2026/10/secret.pdf"} {
		if code := post(bad); code == http.StatusOK {
			t.Fatalf("非法文件引用 %s 应被拒绝", bad)
		}
	}
	if code := post(own); code != http.StatusOK {
		t.Fatalf("引用本人上传附件应成功，实际 %d", code)
	}
	// 已被占用的附件不能再被其他事项引用
	if code := post(own); code == http.StatusOK {
		t.Fatal("已绑定的附件不得被第二个事项重复引用")
	}

	// 材料下发同理
	body := `{"title":"t","units":"县委办","pdf_path":"` + adminDoc + `","pdf_name":"a.pdf"}`
	req := httptest.NewRequest("POST", "/api/dispatches", bytes.NewBufferString(body))
	req = reqWithContext(req, 2, "staff1", "staff", "李干事")
	rec := httptest.NewRecorder()
	CreateDispatch(cfg)(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("材料下发引用收文附件应被拒绝")
	}

	// 截止时间格式非法应被拒绝
	body = `{"title":"t","deadline":"2026/12/31","units":"县委办","pdf_path":"` + insertTestAttachment(t, "solicit", 2, "x.pdf") + `","pdf_name":"a.pdf"}`
	req = httptest.NewRequest("POST", "/api/solicits", bytes.NewBufferString(body))
	req = reqWithContext(req, 2, "staff1", "staff", "李干事")
	rec = httptest.NewRecorder()
	CreateSolicit(cfg)(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("非法截止时间格式应被拒绝")
	}
}

// 旧版数字 ID 公开链接必须失效，令牌链接可用；乡镇通讯员默认不再拥有征求意见/材料下发管理权限
func TestPublicLinks_TokenOnly(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("INSERT INTO meetings (id, title, meeting_date, meeting_time, location, units, public_token) VALUES (7, '会', '2099-01-01', '10:00', '会议室', '县委办', 'abcdefabcdefabcdefabcdefabcdef12')")

	call := func(tok string) int {
		req := httptest.NewRequest("GET", "/api/public/meetings/"+tok, nil)
		req.SetPathValue("token", tok)
		rec := httptest.NewRecorder()
		PublicMeeting(rec, req)
		return rec.Code
	}
	if code := call("7"); code != http.StatusNotFound {
		t.Fatalf("数字 ID 链接应失效，实际 %d", code)
	}
	if code := call("abcdefabcdefabcdefabcdefabcdef00"); code != http.StatusNotFound {
		t.Fatalf("错误令牌应 404，实际 %d", code)
	}
	if code := call("abcdefabcdefabcdefabcdefabcdef12"); code != http.StatusOK {
		t.Fatalf("正确令牌应 200，实际 %d", code)
	}

	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions rp JOIN roles r ON rp.role_id=r.id
		WHERE r.code='reporter' AND rp.permission_code IN ('solicit.manage','dispatch.manage')`).Scan(&n)
	if n != 0 {
		t.Fatalf("v20 迁移后 reporter 不应再拥有征求意见/材料下发管理权限，实际 %d 项", n)
	}
	database.DB.QueryRow(`SELECT COUNT(*) FROM role_permissions rp JOIN roles r ON rp.role_id=r.id
		WHERE r.code='staff' AND rp.permission_code='solicit.manage'`).Scan(&n)
	if n != 1 {
		t.Fatal("科室用户应保留征求意见管理权限")
	}
}

// 被删除用户的旧令牌不得在新用户身上复活；角色以数据库为准
func TestAuth_DeletedUserTokenCannotRevive(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='admin') WHERE id=1")
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='staff') WHERE id=2")
	hash, _ := database.HashPassword("x")
	res, _ := database.DB.Exec("INSERT INTO users (username, password_hash, real_name, department_id, role_id, status, token_version) VALUES ('boss', ?, '老管理员', 1, (SELECT id FROM roles WHERE code='admin'), 1, 0)", hash)
	oldID, _ := res.LastInsertId()
	oldToken, err := auth.GenerateToken(oldID, "boss", "老管理员", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	var seen string
	h := middleware.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = r.Context().Value(middleware.ContextRoleCode).(string)
	}))
	call := func(tok string) int {
		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if call(oldToken) != http.StatusOK || seen != "admin" {
		t.Fatal("正常令牌应通过")
	}

	// 删除用户并新建一个普通用户
	del := httptest.NewRequest("DELETE", "/api/users/x", nil)
	del.SetPathValue("id", strconv.FormatInt(oldID, 10))
	del = reqWithContext(del, 1, "admin", "admin", "系统管理员")
	rec := httptest.NewRecorder()
	DeleteUser(rec, del)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除用户失败: %d %s", rec.Code, rec.Body.String())
	}
	cr := httptest.NewRequest("POST", "/api/users", bytes.NewBufferString(`{"username":"newbie","real_name":"新人","department_id":1,"role_id":2}`))
	cr = reqWithContext(cr, 1, "admin", "admin", "系统管理员")
	rec = httptest.NewRecorder()
	CreateUser(rec, cr)
	if rec.Code != http.StatusOK {
		t.Fatalf("新建用户失败: %d", rec.Code)
	}
	var newID int64
	database.DB.QueryRow("SELECT id FROM users WHERE username='newbie'").Scan(&newID)
	if newID == oldID {
		t.Fatal("用户 ID 不得复用")
	}
	// 即使强行把新用户放到旧 ID 上，旧令牌也因用户名/版本不符而失效
	database.DB.Exec("UPDATE users SET id=? WHERE id=?", oldID, newID)
	middleware.InvalidateTokenVersion(oldID)
	if code := call(oldToken); code != http.StatusUnauthorized {
		t.Fatalf("被删除用户的旧令牌应失效，实际 %d", code)
	}

	// 角色以库为准：令牌声称 admin，但库内为 staff
	middleware.InvalidateTokenVersion(2)
	staffTok, _ := auth.GenerateToken(2, "staff1", "李干事", "admin", 0)
	if call(staffTok) != http.StatusOK || seen != "staff" {
		t.Fatalf("角色应取自数据库，实际 %q", seen)
	}
}

// 默认密码用户必须先改密：服务端拦截其他接口，仅放行个人信息/改密/退出
func TestAuth_MustChangePasswordEnforced(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='staff'), must_change_password=1 WHERE id=2")
	middleware.InvalidateTokenVersion(2)
	tok, _ := auth.GenerateToken(2, "staff1", "李干事", "staff", 0)
	h := middleware.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	call := func(path string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := call("/api/incoming-docs"); c != http.StatusForbidden {
		t.Fatalf("默认密码用户访问业务接口应 403，实际 %d", c)
	}
	if c := call("/api/auth/profile"); c != http.StatusOK {
		t.Fatalf("个人信息接口应放行，实际 %d", c)
	}
}
