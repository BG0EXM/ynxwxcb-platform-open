package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"ynxwxcb-platform/internal/auth"
	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

func setupTestEnv(t *testing.T) (*config.Config, func()) {
	tmpDir, err := os.MkdirTemp("", "ynxwxcb_e2e_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	uploadDir := filepath.Join(tmpDir, "uploads")
	os.MkdirAll(uploadDir, 0755)

	cfg := config.Default()
	cfg.Server.Port = "8080"
	cfg.Server.TrustProxy = false
	cfg.Database.Path = dbPath
	cfg.JWT.Secret = "test-jwt-secret-string-at-least-32-chars-long!"
	cfg.Upload.Dir = uploadDir
	cfg.Upload.MaxMB = 20
	cfg.Admin.Username = "admin"
	cfg.Admin.Password = "testadminpassword"

	auth.Init(cfg.JWT.Secret)
	if err := database.Init(dbPath); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	// 补足可能缺失的基础数据
	database.DB.Exec("INSERT OR IGNORE INTO roles (id, name, code, description) VALUES (1, '系统管理员', 'admin', '全权限')")
	database.DB.Exec("INSERT OR IGNORE INTO roles (id, name, code, description) VALUES (2, '分管领导', 'leader', '审阅')")
	database.DB.Exec("INSERT OR IGNORE INTO roles (id, name, code, description) VALUES (3, '科室工作人员', 'staff', '业务')")
	database.DB.Exec("INSERT OR IGNORE INTO roles (id, name, code, description) VALUES (4, '乡镇/通讯员', 'reporter', '通讯')")
	database.DB.Exec("INSERT OR IGNORE INTO departments (id, name) VALUES (1, '办公室')")
	database.DB.Exec("INSERT OR IGNORE INTO departments (id, name) VALUES (2, '新闻宣传科')")

	hash, _ := database.HashPassword("123456")
	database.DB.Exec(`INSERT OR REPLACE INTO users (id, username, password_hash, real_name, department_id, role_id, status, token_version)
		VALUES (1, 'admin', ?, '系统管理员', 1, 1, 1, 0)`, hash)
	database.DB.Exec(`INSERT OR REPLACE INTO users (id, username, password_hash, real_name, department_id, role_id, status, token_version)
		VALUES (2, 'staff1', ?, '李干事', 2, 3, 1, 0)`, hash)

	middleware.ReloadPermissions()

	cleanup := func() {
		database.DB.Close()
		os.RemoveAll(tmpDir)
	}
	return cfg, cleanup
}

func reqWithContext(req *http.Request, userID int64, username, roleCode, realName string) *http.Request {
	ctx := req.Context()
	ctx = context.WithValue(ctx, middleware.ContextUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextUsername, username)
	ctx = context.WithValue(ctx, middleware.ContextRoleCode, roleCode)
	ctx = context.WithValue(ctx, middleware.ContextRealName, realName)
	return req.WithContext(ctx)
}

// 1. 测试考勤与请假、补休、年休假联动
func TestAttendanceAndLeaveE2E(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// 步骤 A: 录入加班记录 8 小时 (折合 1 天补休)
	{
		body := `{"user_id":2,"overtime_date":"2026-10-01","hours":8,"reason":"国庆值班"}`
		req := httptest.NewRequest("POST", "/api/overtime-records", bytes.NewBufferString(body))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateOvertimeRecord(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("新增加班记录失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 B: 验证 getCompRemainDays 为 1.0 天
	remain := getCompRemainDays(2)
	if remain != 1.0 {
		t.Fatalf("期望剩余补休 1.0 天，实际 %.2f 天", remain)
	}

	// 步骤 C: 申请 2 天补休（应当被拒绝，因为只剩 1 天）
	{
		body := `{"user_id":2,"leave_type":"comp","start_date":"2026-10-08","end_date":"2026-10-09","days":2,"reason":"补休"}`
		req := httptest.NewRequest("POST", "/api/leave-records", bytes.NewBufferString(body))
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		CreateLeaveRecord(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("申请超过余额的补休应当被拦截，但成功了")
		}
	}

	// 步骤 D: 申请 1 天补休（应当成功）
	{
		body := `{"user_id":2,"leave_type":"comp","start_date":"2026-10-08","end_date":"2026-10-08","days":1,"reason":"补休"}`
		req := httptest.NewRequest("POST", "/api/leave-records", bytes.NewBufferString(body))
		req = reqWithContext(req, 2, "staff1", "staff", "李干事")
		rec := httptest.NewRecorder()
		CreateLeaveRecord(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("申请 1 天补休失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 E: 点到列表 MarkUsers，当天有补休记录，应自动标记为正常出勤并带 auto_comp=1
	{
		req := httptest.NewRequest("GET", "/api/attendance/mark-users?date=2026-10-08", nil)
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		MarkUsers(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("MarkUsers 失败: %d, %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			List []struct {
				ID       int64 `json:"id"`
				Status   int   `json:"status"`
				AutoComp int   `json:"auto_comp"`
			} `json:"list"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		found := false
		for _, u := range resp.List {
			if u.ID == 2 {
				found = true
				if u.Status != 1 || u.AutoComp != 1 {
					t.Fatalf("补休人员点到状态期望 status=1 (出勤), auto_comp=1，实际 status=%d, auto_comp=%d", u.Status, u.AutoComp)
				}
			}
		}
		if !found {
			t.Fatalf("未找到人员 ID=2")
		}
	}
}

// 2. 测试征求意见模块完整闭环
func TestSolicitWorkflowE2E(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()

	// 步骤 A: 创建征求意见任务
	var solicitID int64
	{
		body := `{
			"title": "关于开展2026年文化宣传周活动的意见征求",
			"doc_no": "伊党宣字〔2026〕15号",
			"deadline": "2026-12-31 18:00",
			"units": "吉里于孜镇\n愉群翁回族乡\n县教育局",
			"content": "请各单位认真研读草案并反馈。",
			"pdf_path": "/uploads/solicits/draft.pdf",
			"pdf_name": "文化宣传周活动草案.pdf"
		}`
		req := httptest.NewRequest("POST", "/api/solicits", bytes.NewBufferString(body))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateSolicit(cfg)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("创建征求意见失败: %d, %s", rec.Code, rec.Body.String())
		}
		var res map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &res)
		if idVal, ok := res["id"].(float64); ok {
			solicitID = int64(idVal)
		} else {
			// 从数据库查
			database.DB.QueryRow("SELECT id FROM solicits ORDER BY id DESC LIMIT 1").Scan(&solicitID)
		}
	}

	if solicitID == 0 {
		t.Fatalf("solicitID 不合法: %d", solicitID)
	}

	// 步骤 B: 公开免密端加载详情
	{
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/public/solicits/%d", solicitID), nil)
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		PublicSolicit(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("公开获取征求意见失败: %d, %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Units   []string `json:"units"`
			Expired bool     `json:"expired"`
		}
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if len(resp.Units) != 3 || resp.Expired {
			t.Fatalf("公开端单位数或截止状态异常: units=%d, expired=%v", len(resp.Units), resp.Expired)
		}
	}

	// 步骤 C: 公开端上传盖章回函
	var uploadedPath string
	{
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		part, _ := w.CreateFormFile("file", "吉里于孜镇盖章回函.pdf")
		part.Write([]byte("%PDF-1.4 正常无涉密公文内容，原则同意活动方案。"))
		w.Close()

		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/solicits/%d/upload", solicitID), &b)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		PublicSolicitUpload(cfg)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("公开上传回函失败: %d, %s", rec.Code, rec.Body.String())
		}
		var res map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &res)
		uploadedPath = res["file_path"].(string)
	}

	// 步骤 D: 公开提交反馈（无意见）
	{
		body := fmt.Sprintf(`{
			"unit": "吉里于孜镇",
			"has_opinion": 0,
			"opinion_detail": "",
			"reply_doc_path": "%s",
			"reply_doc_name": "吉里于孜镇盖章回函.pdf",
			"contact_name": "王主任",
			"contact_phone": "13912345678"
		}`, uploadedPath)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/solicits/%d/feedback", solicitID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("提交反馈失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 E: 验证防篡改锁定：再次重复提交应返回 403 Forbidden
	{
		body := fmt.Sprintf(`{
			"unit": "吉里于孜镇",
			"has_opinion": 1,
			"opinion_detail": "试图篡改",
			"reply_doc_path": "%s",
			"reply_doc_name": "篡改.pdf",
			"contact_name": "黑客",
			"contact_phone": "13900000000"
		}`, uploadedPath)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/solicits/%d/feedback", solicitID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("重复提交未触发防篡改锁定，状态码: %d", rec.Code)
		}
	}

	// 步骤 F: 管理员在后台重置该单位
	{
		body := `{"unit": "吉里于孜镇"}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/solicits/%d/feedbacks/reset-unit", solicitID), bytes.NewBufferString(body))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		ResetUnitFeedback(cfg)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("管理员重置反馈失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 G: 重置后再次提交应允许成功
	{
		body := fmt.Sprintf(`{
			"unit": "吉里于孜镇",
			"has_opinion": 0,
			"opinion_detail": "",
			"reply_doc_path": "%s",
			"reply_doc_name": "吉里于孜镇盖章回函.pdf",
			"contact_name": "王主任",
			"contact_phone": "13912345678"
		}`, uploadedPath)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/solicits/%d/feedback", solicitID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(solicitID, 10))
		rec := httptest.NewRecorder()
		PublicSubmitFeedback(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("重置后重新提交失败: %d, %s", rec.Code, rec.Body.String())
		}
	}
}

// 3. 测试材料下发模块完整闭环
func TestDispatchWorkflowE2E(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()

	// 步骤 A: 创建材料下发任务
	var dispatchID int64
	{
		body := `{
			"title": "关于印发《2026年全县精神文明建设要点》的通知",
			"doc_no": "伊文明办发〔2026〕5号",
			"units": "吉里于孜镇\n愉群翁回族乡\n县委办",
			"content": "请各单位查收并贯彻落实。",
			"pdf_path": "/uploads/dispatches/notice.pdf",
			"pdf_name": "精神文明建设要点.pdf"
		}`
		req := httptest.NewRequest("POST", "/api/dispatches", bytes.NewBufferString(body))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateDispatch(cfg)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("创建材料下发失败: %d, %s", rec.Code, rec.Body.String())
		}
		database.DB.QueryRow("SELECT id FROM dispatches ORDER BY id DESC LIMIT 1").Scan(&dispatchID)
	}

	// 步骤 B: 公开确认查收
	{
		body := `{
			"unit": "吉里于孜镇",
			"receiver_name": "刘宣传员",
			"receiver_phone": "13888888888"
		}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/dispatches/%d/receipt", dispatchID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(dispatchID, 10))
		rec := httptest.NewRecorder()
		PublicConfirmDispatchReceipt(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("确认查收失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 C: 再次确认查收（测试 read_count 递增）
	{
		body := `{
			"unit": "吉里于孜镇",
			"receiver_name": "刘宣传员",
			"receiver_phone": "13888888888"
		}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/dispatches/%d/receipt", dispatchID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(dispatchID, 10))
		rec := httptest.NewRecorder()
		PublicConfirmDispatchReceipt(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("再次查收失败: %d, %s", rec.Code, rec.Body.String())
		}
		var count int
		database.DB.QueryRow("SELECT read_count FROM dispatch_receipts WHERE dispatch_id=? AND unit='吉里于孜镇'", dispatchID).Scan(&count)
		if count != 2 {
			t.Fatalf("查收次数递增期望为 2，实际为 %d", count)
		}
	}
}

// 4. 测试会务管理报名与限额控制
func TestMeetingWorkflowE2E(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// 步骤 A: 创建会议（限制每单位 1 人）
	var meetingID int64
	{
		body := `{
			"title": "全县宣传思想文化工作推进会",
			"meeting_date": "2026-11-20",
			"meeting_time": "10:30",
			"location": "县委四楼一号会议室",
			"content": "请各单位分管领导准时参会。",
			"units": "吉里于孜镇\n愉群翁回族乡",
			"unit_limit": 1
		}`
		req := httptest.NewRequest("POST", "/api/meetings", bytes.NewBufferString(body))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateMeeting(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("创建会议失败: %d, %s", rec.Code, rec.Body.String())
		}
		database.DB.QueryRow("SELECT id FROM meetings ORDER BY id DESC LIMIT 1").Scan(&meetingID)
	}

	// 步骤 B: 第一次公开报名（参加）
	{
		body := `{
			"unit": "吉里于孜镇",
			"attendee_name": "张宣传委员",
			"attendee_title": "党委宣传委员",
			"phone": "13800000001",
			"not_attend": 0
		}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/meetings/%d/register", meetingID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(meetingID, 10))
		rec := httptest.NewRecorder()
		PublicRegisterMeeting(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("会议报名失败: %d, %s", rec.Code, rec.Body.String())
		}
	}

	// 步骤 C: 验证限额：该单位再报第 2 人（单人限额下应覆盖/更新原有单人，或报错）
	// 注意代码逻辑：unitLimit == 1 时会更新/替换单人
	{
		body := `{
			"unit": "吉里于孜镇",
			"attendee_name": "李副镇长",
			"attendee_title": "副镇长",
			"phone": "13800000002",
			"not_attend": 0
		}`
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/meetings/%d/register", meetingID), bytes.NewBufferString(body))
		req.SetPathValue("id", strconv.FormatInt(meetingID, 10))
		rec := httptest.NewRecorder()
		PublicRegisterMeeting(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("单人限额更新报名失败: %d, %s", rec.Code, rec.Body.String())
		}
		// 校验报名表里吉里于孜镇依然只有 1 人
		var cnt int
		database.DB.QueryRow("SELECT COUNT(*) FROM meeting_registrations WHERE meeting_id=? AND unit='吉里于孜镇' AND not_attend=0", meetingID).Scan(&cnt)
		if cnt != 1 {
			t.Fatalf("单人限额下期望报名人数为 1，实际为 %d", cnt)
		}
	}
}

// 5. 测试涉密文件上传阻断（Secrecy 防线）
func TestSecrecyUploadBlocking(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()

	// 模拟上传带有国家秘密密级标识“机密”的文件
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, _ := w.CreateFormFile("file", "关于全县网络安全排查通报(机密).pdf")
	part.Write([]byte("%PDF-1.4 本文件属于国家机密级材料，严禁外传。"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/uploads", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
	rec := httptest.NewRecorder()
	UploadFile(cfg)(rec, req)

	// 必须被 403 阻断
	if rec.Code != http.StatusForbidden {
		t.Fatalf("涉密文件上传期望返回 403 Forbidden，实际返回: %d, body: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res["security_violation"] != "SECRECY_LEAK_PREVENTED" {
		t.Fatalf("涉密拦截标记异常: %v", res)
	}
}

// 6. 测试用户修改密码后令牌版本自增与旧令牌失效
func TestTokenVersionRevocation(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// 生成初始 token (version=0)
	token1, err := auth.GenerateToken(2, "staff1", "李干事", "staff", 0)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}

	// 校验 token1 正确性
	claims, err := auth.ParseToken(token1)
	if err != nil || claims.TokenVersion != 0 {
		t.Fatalf("解析 token1 失败: %v", err)
	}

	// 用户修改密码（使 token_version 自增）
	body := `{"old_password":"123456","new_password":"newpassword123"}`
	req := httptest.NewRequest("POST", "/api/auth/change-password", bytes.NewBufferString(body))
	req = reqWithContext(req, 2, "staff1", "staff", "李干事")
	rec := httptest.NewRecorder()
	ChangePassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("修改密码失败: %d, %s", rec.Code, rec.Body.String())
	}

	// 验证数据库中 token_version 已自增为 1
	var tv int
	database.DB.QueryRow("SELECT token_version FROM users WHERE id=2").Scan(&tv)
	if tv != 1 {
		t.Fatalf("token_version 期望为 1，实际为 %d", tv)
	}

	// 验证使用旧 token1 (version=0) 请求被 middleware.Auth 拒绝 (401)
	authMiddleware := middleware.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqOld := httptest.NewRequest("GET", "/api/auth/profile", nil)
	reqOld.Header.Set("Authorization", "Bearer "+token1)
	recOld := httptest.NewRecorder()
	authMiddleware.ServeHTTP(recOld, reqOld)
	if recOld.Code != http.StatusUnauthorized {
		t.Fatalf("使用已失效旧 token 期望返回 401，实际返回: %d", recOld.Code)
	}

	// 生成新 token (version=1) 请求应成功通过
	token2, _ := auth.GenerateToken(2, "staff1", "李干事", "staff", 1)
	reqNew := httptest.NewRequest("GET", "/api/auth/profile", nil)
	reqNew.Header.Set("Authorization", "Bearer "+token2)
	recNew := httptest.NewRecorder()
	authMiddleware.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("使用新 token 期望返回 200，实际返回: %d", recNew.Code)
	}
}
