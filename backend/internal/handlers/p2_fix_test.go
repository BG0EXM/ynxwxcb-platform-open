package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"ynxwxcb-platform/internal/database"
)

// 登录锁定按“用户名+IP”计数：攻击者 IP 被锁，不影响其他 IP 登录同一账号
func TestLoginLock_PerUsernameAndIP(t *testing.T) {
	cfg, cleanup := setupTestEnv(t)
	defer cleanup()
	login := func(ip, pwd string) int {
		req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString(`{"username":"staff1","password":"`+pwd+`"}`))
		req.RemoteAddr = ip + ":5555"
		rec := httptest.NewRecorder()
		Login(cfg)(rec, req)
		return rec.Code
	}
	for i := 0; i < 5; i++ {
		login("10.9.9.9", "wrong-pass")
	}
	if c := login("10.9.9.9", "123456"); c != http.StatusTooManyRequests {
		t.Fatalf("攻击者 IP 应被锁定，实际 %d", c)
	}
	if c := login("10.1.1.1", "123456"); c != http.StatusOK {
		t.Fatalf("其他 IP 登录同一账号不应受影响，实际 %d", c)
	}
}

// 用户管理保护：不能禁用自己、未传状态不改状态、非管理员不能提权、保留最后一个管理员
func TestUpdateUser_Protections(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='admin') WHERE id=1")
	database.DB.Exec("UPDATE users SET role_id=(SELECT id FROM roles WHERE code='staff') WHERE id=2")
	var adminRole, staffRole int64
	database.DB.QueryRow("SELECT id FROM roles WHERE code='admin'").Scan(&adminRole)
	database.DB.QueryRow("SELECT id FROM roles WHERE code='staff'").Scan(&staffRole)
	put := func(opID int64, opRole, body string) int {
		req := httptest.NewRequest("PUT", "/api/users", bytes.NewBufferString(body))
		req = reqWithContext(req, opID, "op", opRole, "op")
		rec := httptest.NewRecorder()
		UpdateUser(rec, req)
		return rec.Code
	}
	itoa := func(n int64) string { return string(rune('0' + n)) }
	if c := put(1, "admin", `{"id":1,"real_name":"系统管理员","role_id":`+itoa(adminRole)+`,"status":0}`); c == http.StatusOK {
		t.Fatal("管理员不能禁用自己")
	}
	if c := put(1, "admin", `{"id":2,"real_name":"李干事","role_id":`+itoa(staffRole)+`}`); c != http.StatusOK {
		t.Fatalf("未传状态的修改应成功，实际 %d", c)
	}
	var st int
	database.DB.QueryRow("SELECT status FROM users WHERE id=2").Scan(&st)
	if st != 1 {
		t.Fatal("未传状态时不得把账号改为禁用")
	}
	if c := put(2, "staff", `{"id":2,"real_name":"李干事","role_id":`+itoa(adminRole)+`,"status":1}`); c == http.StatusOK {
		t.Fatal("非管理员不得给自己授予管理员角色")
	}
	database.DB.Exec("INSERT INTO users (id, username, password_hash, real_name, department_id, role_id, status) VALUES (9, 'op9', 'x', '运维', 1, ?, 1)", adminRole)
	if c := put(9, "admin", `{"id":1,"real_name":"系统管理员","role_id":`+itoa(staffRole)+`,"status":1}`); c != http.StatusOK {
		t.Fatalf("仍有其他管理员时可降级，实际 %d", c)
	}
	if c := put(1, "admin", `{"id":9,"real_name":"运维","role_id":`+itoa(adminRole)+`,"status":0}`); c == http.StatusOK {
		t.Fatal("不得禁用最后一个启用的管理员")
	}
}

// 删除请假只回退由该请假联动产生的考勤，管理员手工点到的其他请假类型保持不变
func TestDeleteLeave_KeepsManualMark(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("INSERT INTO leave_records (id, user_id, leave_type, start_date, end_date, days, status) VALUES (31, 2, 'personal', '2026-07-02', '2026-07-03', 2, 1)")
	database.DB.Exec("INSERT INTO attendances (user_id, attend_date, status, leave_type, remark, marked_by) VALUES (2, '2026-07-02', 2, 'personal', '', 1)")
	database.DB.Exec("INSERT INTO attendances (user_id, attend_date, status, leave_type, remark, marked_by) VALUES (2, '2026-07-03', 2, 'sick', '医院证明', 1)")
	req := httptest.NewRequest("DELETE", "/api/leave-records/31", nil)
	req.SetPathValue("id", "31")
	req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
	rec := httptest.NewRecorder()
	DeleteLeaveRecord(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除失败: %d", rec.Code)
	}
	var s1, s2 int
	database.DB.QueryRow("SELECT status FROM attendances WHERE user_id=2 AND attend_date='2026-07-02'").Scan(&s1)
	database.DB.QueryRow("SELECT status FROM attendances WHERE user_id=2 AND attend_date='2026-07-03'").Scan(&s2)
	if s1 != 1 {
		t.Fatalf("联动产生的事假考勤应恢复出勤，实际 %d", s1)
	}
	if s2 != 2 {
		t.Fatalf("手工点到的病假不应被改动，实际 %d", s2)
	}
}
