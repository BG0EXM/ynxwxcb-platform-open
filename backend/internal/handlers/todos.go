package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
)

// 工作台“今日待办”：把各模块需要当天处理的事集中汇总。
// 只读现有数据；每类按权限点放行，没有权限的类别整组不返回。

const (
	todoSoonDays      = 3  // 快截止/快召开/快退文：3 天内
	todoBorrowDays    = 30 // 借出超期：30 天
	todoItemsPerGroup = 5  // 每类最多返回条数（total 为全量）
	todoScanLimit     = 2000
)

// 紧急程度：danger 已逾期或今天到期；warning 3 天内；info 其他
var todoLevelRank = map[string]int{"danger": 0, "warning": 1, "info": 2}

type todoItem struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Due   string `json:"due"` // 时限说明，如“今天 18:00 截止”“已逾期 2 天”
	Level string `json:"level"`
	Link  string `json:"link"`
	sortT time.Time
}

type todoGroup struct {
	Key   string     `json:"key"`
	Title string     `json:"title"`
	Icon  string     `json:"icon"`
	Path  string     `json:"path"` // “查看全部”跳转地址
	Total int        `json:"total"`
	Items []todoItem `json:"items"`
}

// finish 排序（紧急在前，同级按时限先后）并截取前几条
func (g *todoGroup) finish() {
	sort.SliceStable(g.Items, func(i, j int) bool {
		a, b := g.Items[i], g.Items[j]
		if todoLevelRank[a.Level] != todoLevelRank[b.Level] {
			return todoLevelRank[a.Level] < todoLevelRank[b.Level]
		}
		return a.sortT.Before(b.sortT)
	})
	g.Total = len(g.Items)
	if len(g.Items) > todoItemsPerGroup {
		g.Items = g.Items[:todoItemsPerGroup]
	}
}

// todoStartOfDay 本地时间当天零点
func todoStartOfDay(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// todoDayDiff 目标时刻所在日期距今天相差几天（今天为 0，明天为 1，昨天为 -1）
func todoDayDiff(now, t time.Time) int {
	return int(todoStartOfDay(t).Sub(todoStartOfDay(now)).Hours() / 24)
}

// todoDueText 截止类时限说明
func todoDueText(now, t time.Time, verb string) string {
	hm := t.In(time.Local).Format("15:04")
	switch d := todoDayDiff(now, t); {
	case d < 0:
		return fmt.Sprintf("已逾期 %d 天", -d)
	case d == 0:
		return "今天 " + hm + " " + verb
	case d == 1:
		return "明天 " + hm + " " + verb
	case d == 2:
		return "后天 " + hm + " " + verb
	default:
		return t.In(time.Local).Format("1月2日 15:04") + " " + verb
	}
}

// todoParseDeadline 解析截止时间（只填日期的视为当天 23:59:59）
func todoParseDeadline(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			if l == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Second)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// todoUnits 把换行分隔的单位范围拆成去重列表
func todoUnits(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, u := range strings.Split(raw, "\n") {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// todoRespondedUnits 某条记录已响应（反馈/查收/报名）的单位集合
func todoRespondedUnits(query string, id int64) map[string]bool {
	done := map[string]bool{}
	rows, err := database.DB.Query(query, id)
	if err != nil {
		return done
	}
	defer rows.Close()
	for rows.Next() {
		var u string
		if rows.Scan(&u) == nil {
			done[strings.TrimSpace(u)] = true
		}
	}
	return done
}

// todoMissing 范围内尚未响应的单位数
func todoMissing(units []string, done map[string]bool) int {
	n := 0
	for _, u := range units {
		if !done[u] {
			n++
		}
	}
	return n
}

// DashboardTodos 工作台今日待办 GET /api/dashboard/todos
func DashboardTodos(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	roleCode, _ := r.Context().Value(middleware.ContextRoleCode).(string)
	now := time.Now()
	can := func(code string) bool { return middleware.HasPermission(roleCode, code) }

	groups := []*todoGroup{}
	if can("incoming.view") {
		groups = append(groups, todoIncomingReview(now, roleCode, userID), todoIncomingReturn(now, roleCode, userID))
	}
	if can("solicit.manage") {
		groups = append(groups, todoDeadlineGroup(now, "solicit"))
	}
	if can("dispatch.manage") {
		groups = append(groups, todoDeadlineGroup(now, "dispatch"))
	}
	if can("meeting.manage") {
		groups = append(groups, todoMeetings(now))
	}
	if can("warehouse.manage") {
		groups = append(groups, todoWarehouseBorrows(now), todoWarehouseLow())
	}

	total := 0
	out := []*todoGroup{}
	for _, g := range groups {
		g.finish()
		if g.Total > 0 {
			total += g.Total
			out = append(out, g)
		}
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"total": total, "groups": out})
}

// 收文·待批示：管理员看全部，其他人只看自己登记的
func todoIncomingReview(now time.Time, roleCode string, userID int64) *todoGroup {
	g := &todoGroup{Key: "incoming_review", Title: "收文待批示", Icon: "FolderOpened", Path: "/incoming?status=3", Items: []todoItem{}}
	q := `SELECT id, IFNULL(receive_no,''), title, IFNULL(from_unit,''), IFNULL(urgency,''), IFNULL(received_date,'')
		FROM incoming_docs WHERE status = 3`
	args := []interface{}{}
	if roleCode != "admin" {
		q += " AND registrar_id = ?"
		args = append(args, userID)
	}
	rows, err := database.DB.Query(q+" ORDER BY id LIMIT ?", append(args, todoScanLimit)...)
	if err != nil {
		return g
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var no, title, from, urgency, received string
		if rows.Scan(&id, &no, &title, &from, &urgency, &received) != nil {
			continue
		}
		level := "info"
		switch urgency {
		case "特急":
			level = "danger"
		case "急件":
			level = "warning"
		}
		desc := strings.Trim(strings.Join([]string{no, from}, " · "), " ·")
		due := ""
		t, err := time.ParseInLocation("2006-01-02", received, time.Local)
		if err == nil {
			if d := -todoDayDiff(now, t); d > 0 {
				due = fmt.Sprintf("已等 %d 天", d)
			} else {
				due = "今天收文"
			}
		}
		if urgency == "特急" || urgency == "急件" {
			due = strings.TrimLeft(urgency+" · "+due, " ·")
		}
		g.Items = append(g.Items, todoItem{ID: id, Title: title, Desc: desc, Due: due, Level: level,
			Link: fmt.Sprintf("/incoming?id=%d", id), sortT: t})
	}
	return g
}

// 收文·退文快到期：需退回、未退、退文日期在 3 天内或已过期
func todoIncomingReturn(now time.Time, roleCode string, userID int64) *todoGroup {
	g := &todoGroup{Key: "incoming_return", Title: "收文退文快到期", Icon: "Back", Path: "/incoming?need_return=1", Items: []todoItem{}}
	limit := todoStartOfDay(now).AddDate(0, 0, todoSoonDays).Format("2006-01-02")
	q := `SELECT id, IFNULL(receive_no,''), title, return_date FROM incoming_docs
		WHERE need_return = 1 AND IFNULL(returned,0) = 0 AND IFNULL(return_date,'') <> '' AND return_date <= ?`
	args := []interface{}{limit}
	if roleCode != "admin" {
		q += " AND registrar_id = ?"
		args = append(args, userID)
	}
	rows, err := database.DB.Query(q+" ORDER BY return_date LIMIT ?", append(args, todoScanLimit)...)
	if err != nil {
		return g
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var no, title, ret string
		if rows.Scan(&id, &no, &title, &ret) != nil {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02", ret, time.Local)
		if err != nil {
			continue
		}
		d := todoDayDiff(now, t)
		level, due := "warning", fmt.Sprintf("%d 天后退文（%s）", d, t.Format("1月2日"))
		switch {
		case d < 0:
			level, due = "danger", fmt.Sprintf("退文已逾期 %d 天", -d)
		case d == 0:
			level, due = "danger", "今天须退文"
		case d == 1:
			due = "明天须退文"
		}
		g.Items = append(g.Items, todoItem{ID: id, Title: title, Desc: no, Due: due, Level: level,
			Link: fmt.Sprintf("/incoming?id=%d", id), sortT: t})
	}
	return g
}

// 征求意见 / 材料下发：3 天内截止且还有单位未反馈/未查收
func todoDeadlineGroup(now time.Time, kind string) *todoGroup {
	g := &todoGroup{Key: "solicit", Title: "征求意见快截止", Icon: "EditPen", Path: "/solicits", Items: []todoItem{}}
	listQ := "SELECT id, title, units, IFNULL(deadline,'') FROM solicits WHERE IFNULL(deadline,'') <> '' LIMIT ?"
	doneQ := "SELECT unit FROM solicit_feedbacks WHERE solicit_id = ?"
	verb, missingWord, link := "截止", "未反馈", "/solicits?id=%d"
	if kind == "dispatch" {
		g = &todoGroup{Key: "dispatch", Title: "材料下发快截止", Icon: "Promotion", Path: "/dispatches", Items: []todoItem{}}
		listQ = "SELECT id, title, units, IFNULL(deadline,'') FROM dispatches WHERE IFNULL(deadline,'') <> '' LIMIT ?"
		doneQ = "SELECT unit FROM dispatch_receipts WHERE dispatch_id = ?"
		missingWord, link = "未查收", "/dispatches?id=%d"
	}
	type rec struct {
		id       int64
		title    string
		units    string
		deadline time.Time
	}
	var recs []rec
	rows, err := database.DB.Query(listQ, todoScanLimit)
	if err != nil {
		return g
	}
	horizon := todoStartOfDay(now).AddDate(0, 0, todoSoonDays+1)
	for rows.Next() {
		var x rec
		var dl string
		if rows.Scan(&x.id, &x.title, &x.units, &dl) != nil {
			continue
		}
		t, ok := todoParseDeadline(dl)
		// 只提醒尚未截止、且在 3 天内（含今天）截止的
		if !ok || !t.After(now) || !t.Before(horizon) {
			continue
		}
		x.deadline = t
		recs = append(recs, x)
	}
	rows.Close()
	for _, x := range recs {
		units := todoUnits(x.units)
		missing := todoMissing(units, todoRespondedUnits(doneQ, x.id))
		if missing == 0 {
			continue
		}
		level := "warning"
		if todoDayDiff(now, x.deadline) == 0 {
			level = "danger"
		}
		g.Items = append(g.Items, todoItem{ID: x.id, Title: x.title,
			Desc:  fmt.Sprintf("还差 %d 家%s（共 %d 家）", missing, missingWord, len(units)),
			Due:   todoDueText(now, x.deadline, verb),
			Level: level, Link: fmt.Sprintf(link, x.id), sortT: x.deadline})
	}
	return g
}

// 会务：3 天内召开、还有单位未报名（未限定单位范围的会议照常提醒）
func todoMeetings(now time.Time) *todoGroup {
	g := &todoGroup{Key: "meeting", Title: "会议快召开", Icon: "OfficeBuilding", Path: "/meetings", Items: []todoItem{}}
	from := todoStartOfDay(now).Format("2006-01-02")
	to := todoStartOfDay(now).AddDate(0, 0, todoSoonDays).Format("2006-01-02")
	rows, err := database.DB.Query(`SELECT id, title, IFNULL(meeting_date,''), IFNULL(meeting_time,''), IFNULL(location,''), IFNULL(units,'')
		FROM meetings WHERE meeting_date >= ? AND meeting_date <= ? LIMIT ?`, from, to, todoScanLimit)
	if err != nil {
		return g
	}
	type rec struct {
		id                          int64
		title, date, tm, loc, units string
	}
	var recs []rec
	for rows.Next() {
		var x rec
		if rows.Scan(&x.id, &x.title, &x.date, &x.tm, &x.loc, &x.units) == nil {
			recs = append(recs, x)
		}
	}
	rows.Close()
	for _, x := range recs {
		start, ok := parseMeetingTime(x.date, x.tm)
		if !ok || !start.After(now) {
			continue
		}
		units := todoUnits(x.units)
		done := todoRespondedUnits("SELECT unit FROM meeting_registrations WHERE meeting_id = ? AND TRIM(IFNULL(unit,'')) <> ''", x.id)
		desc := fmt.Sprintf("已报名 %d 家", len(done))
		if len(units) > 0 {
			missing := todoMissing(units, done)
			if missing == 0 {
				continue
			}
			desc = fmt.Sprintf("已报名 %d 家，还差 %d 家（共 %d 家）", len(units)-missing, missing, len(units))
		}
		if x.loc != "" {
			desc += " · " + x.loc
		}
		level := "warning"
		if todoDayDiff(now, start) == 0 {
			level = "danger"
		}
		g.Items = append(g.Items, todoItem{ID: x.id, Title: x.title, Desc: desc, Due: todoDueText(now, start, "召开"),
			Level: level, Link: fmt.Sprintf("/meetings?id=%d", x.id), sortT: start})
	}
	return g
}

// 库房·借出超过 30 天未还（超过 60 天标红）
func todoWarehouseBorrows(now time.Time) *todoGroup {
	g := &todoGroup{Key: "warehouse_borrow", Title: "借出超期未还", Icon: "Box", Path: "/supplies?tab=borrows", Items: []todoItem{}}
	list, err := whOutstandingBorrows("")
	if err != nil {
		return g
	}
	for _, b := range list {
		t, ok := b["created_at"].(time.Time)
		if !ok {
			continue
		}
		days := -todoDayDiff(now, t)
		if days < todoBorrowDays {
			continue
		}
		level := "warning"
		if days >= todoBorrowDays*2 {
			level = "danger"
		}
		g.Items = append(g.Items, todoItem{
			ID:    b["id"].(int64),
			Title: fmt.Sprintf("%s × %d%s", b["item_name"], b["outstanding"], b["unit"]),
			Desc:  fmt.Sprintf("%s · %s", b["person_name"], b["department"]),
			Due:   fmt.Sprintf("已借 %d 天", days),
			Level: level, Link: "/supplies?tab=borrows", sortT: t,
		})
	}
	return g
}

// 库房·低库存（库存为 0 标红）
func todoWarehouseLow() *todoGroup {
	g := &todoGroup{Key: "warehouse_low", Title: "物资库存不足", Icon: "Box", Path: "/supplies?tab=items&low=1", Items: []todoItem{}}
	rows, err := database.DB.Query(`SELECT id, name, IFNULL(spec,''), unit, stock, warn_qty FROM warehouse_items
		WHERE status = 1 AND warn_qty > 0 AND stock <= warn_qty ORDER BY stock, id LIMIT ?`, todoScanLimit)
	if err != nil {
		return g
	}
	defer rows.Close()
	base := time.Unix(0, 0)
	for rows.Next() {
		var id int64
		var name, spec, unit string
		var stock, warn int
		if rows.Scan(&id, &name, &spec, &unit, &stock, &warn) != nil {
			continue
		}
		level, due := "warning", fmt.Sprintf("剩 %d%s", stock, unit)
		if stock <= 0 {
			level, due = "danger", "已领完"
		}
		title := name
		if spec != "" {
			title += "（" + spec + "）"
		}
		g.Items = append(g.Items, todoItem{ID: id, Title: title, Desc: fmt.Sprintf("预警数量 %d%s，请及时补货", warn, unit),
			Due: due, Level: level, Link: "/supplies?tab=items&low=1", sortT: base.Add(time.Duration(stock) * time.Second)})
	}
	return g
}
