package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

type todoResp struct {
	Total  int `json:"total"`
	Groups []struct {
		Key   string `json:"key"`
		Total int    `json:"total"`
		Items []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			Due   string `json:"due"`
			Level string `json:"level"`
			Link  string `json:"link"`
		} `json:"items"`
	} `json:"groups"`
}

func fetchTodos(t *testing.T, userID int64, role string) todoResp {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/dashboard/todos", nil)
	req = reqWithContext(req, userID, "u", role, "测试")
	rec := httptest.NewRecorder()
	DashboardTodos(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("待办接口失败: %d %s", rec.Code, rec.Body.String())
	}
	var res todoResp
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return res
}

func TestDashboardTodos(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	now := time.Now()
	day := func(n int) string { return now.AddDate(0, 0, n).Format("2006-01-02") }
	// 截止时间统一取当天 23:59，避免测试时刻临近午夜导致跨天
	at := func(n int) string { return day(n) + " 23:59" }
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(q, args...); err != nil {
			t.Fatalf("准备数据失败: %v\n%s", err, q)
		}
	}

	// 收文：管理员登记的特急待批示、干事登记的待批示、已办结的不算；退文昨天到期
	exec(`INSERT INTO incoming_docs (id, title, status, urgency, registrar_id, received_date) VALUES
		(1, '特急来文', 3, '特急', 1, ?), (2, '干事登记的来文', 3, '一般', 2, ?), (3, '已办结', 5, '一般', 1, ?)`, day(-2), day(0), day(-1))
	exec(`INSERT INTO incoming_docs (id, title, status, registrar_id, need_return, returned, return_date) VALUES
		(4, '要退的文', 4, 1, 1, 0, ?), (5, '已退的文', 4, 1, 1, 1, ?), (6, '还早的退文', 4, 1, 1, 0, ?)`, day(-1), day(-1), day(10))

	// 征求意见：明天截止差 1 家；全部反馈的不算；10 天后截止的不算；已截止的不算
	exec(`INSERT INTO solicits (id, title, deadline, units, pdf_path, pdf_name) VALUES
		(1, '明天截止', ?, '甲单位
乙单位', 'x', 'x.pdf'), (2, '都反馈了', ?, '甲单位', 'x', 'x.pdf'),
		(3, '还早', ?, '甲单位', 'x', 'x.pdf'), (4, '已截止', ?, '甲单位', 'x', 'x.pdf')`, at(1), at(1), at(10), at(-1))
	exec(`INSERT INTO solicit_feedbacks (solicit_id, unit, reply_doc_path, reply_doc_name, contact_name, contact_phone) VALUES
		(1, '甲单位', 'x', 'x', 'x', 'x'), (2, '甲单位', 'x', 'x', 'x', 'x')`)

	// 材料下发：今天截止差 2 家
	exec(`INSERT INTO dispatches (id, title, deadline, units, pdf_path, pdf_name) VALUES (1, '今天截止的材料', ?, '甲单位
乙单位', 'x', 'x.pdf')`, at(0))

	// 会议：后天召开差 1 家；报齐的不算
	exec(`INSERT INTO meetings (id, title, meeting_date, meeting_time, units) VALUES
		(1, '后天的会', ?, '23:00', '甲单位
乙单位'), (2, '报齐的会', ?, '23:00', '甲单位')`, day(2), day(2))
	exec(`INSERT INTO meeting_registrations (meeting_id, unit, attendee_name) VALUES (1, '甲单位', '张三'), (2, '甲单位', '李四')`)

	// 库房：借出 40 天的相机、刚借的话筒、低库存的纸
	exec(`INSERT INTO warehouse_items (id, name, unit, kind, stock, warn_qty) VALUES
		(1, '相机', '台', 'returnable', 0, 0), (2, '话筒', '支', 'returnable', 3, 0), (3, '复印纸', '包', 'consumable', 2, 5)`)
	exec(`INSERT INTO warehouse_slips (id, slip_no, user_id, person_name, department, created_at) VALUES (1, 'LY1', 2, '李干事', '新闻宣传科', datetime('now','-40 days'))`)
	exec(`INSERT INTO warehouse_records (item_id, slip_id, kind, qty, user_id, person_name, department, created_at) VALUES
		(1, 1, 'borrow', -1, 2, '李干事', '新闻宣传科', datetime('now','-40 days')),
		(2, 1, 'borrow', -1, 2, '李干事', '新闻宣传科', datetime('now','-2 days'))`)

	admin := fetchTodos(t, 1, "admin")
	got := map[string][]string{}
	links := map[string]string{}
	levels := map[string]string{}
	for _, g := range admin.Groups {
		for _, it := range g.Items {
			got[g.Key] = append(got[g.Key], it.Title)
			links[it.Title] = it.Link
			levels[it.Title] = it.Level
		}
	}
	expect := map[string][]string{
		"incoming_review":  {"特急来文", "干事登记的来文"},
		"incoming_return":  {"要退的文"},
		"solicit":          {"明天截止"},
		"dispatch":         {"今天截止的材料"},
		"meeting":          {"后天的会"},
		"warehouse_borrow": {"相机 × 1台"},
		"warehouse_low":    {"复印纸"},
	}
	for k, want := range expect {
		if len(got[k]) != len(want) {
			t.Fatalf("%s 期望 %v，实际 %v", k, want, got[k])
		}
		for i := range want {
			if got[k][i] != want[i] {
				t.Fatalf("%s 第 %d 条期望 %q，实际 %q", k, i, want[i], got[k][i])
			}
		}
	}
	if admin.Total != 8 {
		t.Fatalf("管理员待办总数期望 8，实际 %d", admin.Total)
	}
	if levels["特急来文"] != "danger" || levels["要退的文"] != "danger" || levels["今天截止的材料"] != "danger" || levels["明天截止"] != "warning" {
		t.Fatalf("紧急程度不对: %v", levels)
	}
	if links["明天截止"] != "/solicits?id=1" || links["特急来文"] != "/incoming?id=1" {
		t.Fatalf("跳转链接不对: %v", links)
	}

	// 科室干事：默认没有收文与库房权限，只看到征求意见、材料下发
	staff := fetchTodos(t, 2, "staff")
	for _, g := range staff.Groups {
		switch g.Key {
		case "solicit", "dispatch":
		default:
			t.Fatalf("科室干事不应看到 %s", g.Key)
		}
	}

	// 有收文权限的非管理员：只看自己登记的
	database.DB.Exec(`INSERT OR IGNORE INTO role_permissions (role_id, permission_code) SELECT id, 'incoming.view' FROM roles WHERE code='leader'`)
	middleware.ReloadPermissions()
	leader := fetchTodos(t, 2, "leader")
	seen := false
	for _, g := range leader.Groups {
		if g.Key == "incoming_review" {
			seen = true
			if g.Total != 1 || g.Items[0].Title != "干事登记的来文" {
				t.Fatalf("非管理员只应看到自己登记的收文: %+v", g.Items)
			}
		}
	}
	if !seen {
		t.Fatalf("有收文权限的非管理员应看到自己登记的待批示收文")
	}
}
