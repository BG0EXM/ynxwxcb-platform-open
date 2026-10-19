package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"ynxwxcb-platform/internal/database"
)

func postLeave(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/leave-records", bytes.NewBufferString(body))
	req = reqWithContext(req, 2, "staff1", "staff", "李干事")
	rec := httptest.NewRecorder()
	CreateLeaveRecord(rec, req)
	return rec
}

// 请假天数服务端校验：天数不得超过日历跨度，按小时请假仅限单日并由小时折算
func TestLeave_DaysValidation(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	if rec := postLeave(t, `{"leave_type":"sick","start_date":"2026-10-01","end_date":"2026-10-02","days":5}`); rec.Code == http.StatusOK {
		t.Fatal("天数超过跨度应被拒绝")
	}
	if rec := postLeave(t, `{"leave_type":"sick","start_date":"2026-10-01","end_date":"2026-10-02","leave_hours":4}`); rec.Code == http.StatusOK {
		t.Fatal("跨日按小时请假应被拒绝")
	}
	if rec := postLeave(t, `{"leave_type":"sick","start_date":"2026-01-01","end_date":"2027-06-01","days":100}`); rec.Code == http.StatusOK {
		t.Fatal("超长区间应被拒绝")
	}
	rec := postLeave(t, `{"leave_type":"sick","start_date":"2026-10-09","end_date":"2026-10-09","days":0,"leave_hours":4}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("单日按小时请假应成功: %d %s", rec.Code, rec.Body.String())
	}
	var days float64
	database.DB.QueryRow("SELECT days FROM leave_records WHERE start_date='2026-10-09' AND user_id=2").Scan(&days)
	if days != 0.5 {
		t.Fatalf("4 小时应折算为 0.5 天，实际 %v", days)
	}
}

// 编辑年休假为跨年时，必须按比例核算新年度余额
func TestLeave_UpdateCrossYearAnnualChecksNextYear(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	database.DB.Exec("INSERT INTO annual_leave_configs (user_id, year, days) VALUES (2, '2026', 10)")
	database.DB.Exec("INSERT INTO annual_leave_configs (user_id, year, days) VALUES (2, '2027', 0.5)")
	rec := postLeave(t, `{"leave_type":"annual","start_date":"2026-12-28","end_date":"2026-12-29","days":2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("年内年休假应成功: %d %s", rec.Code, rec.Body.String())
	}
	var id int64
	database.DB.QueryRow("SELECT id FROM leave_records WHERE leave_type='annual' AND user_id=2").Scan(&id)
	body := `{"id":` + strconv.FormatInt(id, 10) + `,"leave_type":"annual","start_date":"2026-12-28","end_date":"2027-01-06","days":10}`
	req := httptest.NewRequest("PUT", "/api/leave-records", bytes.NewBufferString(body))
	// 以管理员身份编辑，避免“过去月份仅管理员可改”的规则使本测试依赖当前日期
	req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
	rec = httptest.NewRecorder()
	UpdateLeaveRecord(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("跨年编辑后 2027 年度额度不足，应被拒绝")
	}
}
