package handlers

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/middleware"
	"ynxwxcb-platform/internal/models"
)

// 库房物资：门口张贴二维码，干部免登录扫码领用/借用/归还；办公室后台管理物资台账、流水与统计。
// 库存口径：stock 为库房内现存数量，借出未还的物品不计入库存。

const (
	whMaxLines   = 30  // 一张领用单最多物资种数
	whMaxLineQty = 999 // 单样物资一次最多领用数量
)

// whKindLabel 流水类型中文名
var whKindLabel = map[string]string{
	"in":     "入库",
	"out":    "领用",
	"borrow": "借用",
	"return": "归还",
	"adjust": "盘点调整",
	"void":   "作废冲回",
}

// whUTC 本地时间转为库内 CURRENT_TIMESTAMP 同格式的 UTC 字符串，用于时间区间比较
func whUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// whMonthRange 解析 YYYY-MM，返回该月（北京时间）对应的 UTC 起止字符串
func whMonthRange(month string) (string, string, bool) {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return "", "", false
	}
	return whUTC(t), whUTC(t.AddDate(0, 1, 0)), true
}

// whDateRange 解析起止日期 YYYY-MM-DD（含结束当天），返回 UTC 起止字符串
func whDateRange(start, end string) (string, string, bool) {
	s, err1 := time.ParseInLocation("2006-01-02", start, time.Local)
	e, err2 := time.ParseInLocation("2006-01-02", end, time.Local)
	if err1 != nil || err2 != nil || e.Before(s) {
		return "", "", false
	}
	return whUTC(s), whUTC(e.AddDate(0, 0, 1)), true
}

// whOperatorName 当前登录用户姓名（后台操作的经办人）
func whOperatorName(r *http.Request) string {
	name, _ := r.Context().Value(middleware.ContextRealName).(string)
	if name == "" {
		name = "办公室"
	}
	return name
}

// ---------------- 扫码设置 ----------------

type whSettings struct {
	Token  string
	Paused bool
	Notice string
}

func loadWhSettings() (whSettings, error) {
	var s whSettings
	var paused int
	err := database.DB.QueryRow("SELECT public_token, paused, IFNULL(notice,'') FROM warehouse_settings WHERE id=1").
		Scan(&s.Token, &paused, &s.Notice)
	if err == sql.ErrNoRows {
		s.Token = newPublicToken()
		if _, err := database.DB.Exec("INSERT OR IGNORE INTO warehouse_settings (id, public_token) VALUES (1, ?)", s.Token); err != nil {
			return s, err
		}
		return loadWhSettings()
	}
	s.Paused = paused == 1
	return s, err
}

// whPublicAuth 校验门口二维码令牌（常量时间比较）；令牌不符一律 404
func whPublicAuth(w http.ResponseWriter, r *http.Request) (whSettings, bool) {
	token := strings.TrimSpace(r.PathValue("token"))
	s, err := loadWhSettings()
	if err != nil || len(token) != 32 || subtle.ConstantTimeCompare([]byte(token), []byte(s.Token)) != 1 {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "二维码已失效，请联系办公室获取最新二维码"})
		return s, false
	}
	return s, true
}

// ---------------- 人员名单 ----------------

type whPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type whDeptGroup struct {
	Department string     `json:"department"`
	People     []whPerson `json:"people"`
}

// loadWhPeople 在册人员按科室分组（只含姓名与 ID，不含手机号、账号）；
// 系统管理员账号（admin 账号名、名称含“管理员”或未分配科室的管理员）不是实际领用人，不列入
func loadWhPeople() ([]whDeptGroup, error) {
	rows, err := database.DB.Query(`SELECT u.id, u.real_name, IFNULL(d.name, '')
		FROM users u
		LEFT JOIN departments d ON u.department_id = d.id
		LEFT JOIN roles ro ON u.role_id = ro.id
		WHERE u.status = 1 AND u.username <> 'admin'
			AND NOT (IFNULL(ro.code,'') = 'admin' AND (u.real_name LIKE '%管理员%' OR IFNULL(d.name,'') = ''))
		ORDER BY CASE WHEN d.id IS NULL THEN 1 ELSE 0 END, IFNULL(d.sort, 0), d.id, u.id
		LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []whDeptGroup{}
	index := map[string]int{}
	for rows.Next() {
		var p whPerson
		var dept string
		if err := rows.Scan(&p.ID, &p.Name, &dept); err != nil {
			return nil, err
		}
		if dept == "" {
			dept = "其他"
		}
		i, ok := index[dept]
		if !ok {
			groups = append(groups, whDeptGroup{Department: dept, People: []whPerson{}})
			i = len(groups) - 1
			index[dept] = i
		}
		groups[i].People = append(groups[i].People, p)
	}
	return groups, rows.Err()
}

// whPersonReq 领用人：名单内人员传 user_id；名单外“其他人员”传姓名、单位、电话
type whPersonReq struct {
	UserID   int64  `json:"user_id"`
	External bool   `json:"external"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Phone    string `json:"phone"`
}

type whResolvedPerson struct {
	UserID     int64
	Name       string
	Department string
	Phone      string
	External   bool
}

// resolve 校验领用人：名单内人员的姓名与科室一律以数据库为准
func (p whPersonReq) resolve() (whResolvedPerson, string) {
	if !p.External {
		if p.UserID <= 0 {
			return whResolvedPerson{}, "请选择领用人"
		}
		var name, dept string
		err := database.DB.QueryRow(`SELECT u.real_name, IFNULL(d.name, '') FROM users u
			LEFT JOIN departments d ON u.department_id = d.id WHERE u.id = ? AND u.status = 1`, p.UserID).Scan(&name, &dept)
		if err != nil {
			return whResolvedPerson{}, "所选人员不存在或已停用，请重新选择"
		}
		if dept == "" {
			dept = "其他"
		}
		return whResolvedPerson{UserID: p.UserID, Name: name, Department: dept}, ""
	}
	name := strings.TrimSpace(p.Name)
	unit := strings.TrimSpace(p.Unit)
	phone := strings.TrimSpace(p.Phone)
	if name == "" || unit == "" || phone == "" {
		return whResolvedPerson{}, "请填写姓名、单位和联系电话"
	}
	if len([]rune(name)) > 20 || len([]rune(unit)) > 50 {
		return whResolvedPerson{}, "填写内容过长"
	}
	if !phonePattern.MatchString(phone) {
		return whResolvedPerson{}, "联系电话格式不正确"
	}
	return whResolvedPerson{Name: name, Department: unit, Phone: phone, External: true}, ""
}

// ---------------- 领用（出库） ----------------

type whLine struct {
	ItemID int64 `json:"item_id"`
	Qty    int   `json:"qty"`
}

type whCheckoutReq struct {
	whPersonReq
	Purpose  string   `json:"purpose"`
	SubmitID string   `json:"submit_id"`
	Lines    []whLine `json:"lines"`
}

type whCheckoutError struct {
	status int
	msg    string
}

// whCheckout 在一个事务内逐项校验库存并扣减，生成领用单与流水；submit_id 相同的重复提交直接返回原单
func whCheckout(req whCheckoutReq, person whResolvedPerson, source string, operatorID int64, operator, ip string) (int64, *whCheckoutError) {
	purpose := strings.TrimSpace(req.Purpose)
	if len([]rune(purpose)) > 100 {
		return 0, &whCheckoutError{http.StatusBadRequest, "用途说明请控制在 100 字以内"}
	}
	submitID := strings.TrimSpace(req.SubmitID)
	if len(submitID) > 64 {
		submitID = submitID[:64]
	}
	// 合并同一物资的多行
	qtyByItem := map[int64]int{}
	order := []int64{}
	for _, l := range req.Lines {
		if l.ItemID <= 0 || l.Qty <= 0 {
			continue
		}
		if _, ok := qtyByItem[l.ItemID]; !ok {
			order = append(order, l.ItemID)
		}
		qtyByItem[l.ItemID] += l.Qty
	}
	if len(order) == 0 {
		return 0, &whCheckoutError{http.StatusBadRequest, "请至少选择一样物资"}
	}
	if len(order) > whMaxLines {
		return 0, &whCheckoutError{http.StatusBadRequest, fmt.Sprintf("一次最多领用 %d 种物资", whMaxLines)}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return 0, &whCheckoutError{http.StatusInternalServerError, "系统繁忙，请稍后重试"}
	}
	defer tx.Rollback()

	if submitID != "" {
		var existing int64
		if err := tx.QueryRow("SELECT id FROM warehouse_slips WHERE submit_id = ?", submitID).Scan(&existing); err == nil {
			return existing, nil
		}
	}

	prefix := "LY" + time.Now().Format("20060102")
	var n int
	if err := tx.QueryRow("SELECT COUNT(*) FROM warehouse_slips WHERE slip_no GLOB ?", prefix+"*").Scan(&n); err != nil {
		return 0, &whCheckoutError{http.StatusInternalServerError, "系统繁忙，请稍后重试"}
	}
	slipNo := fmt.Sprintf("%s%03d", prefix, n+1)
	external := 0
	if person.External {
		external = 1
	}
	res, err := tx.Exec(`INSERT INTO warehouse_slips (slip_no, user_id, person_name, department, phone, is_external, purpose, source, operator_id, ip, submit_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		slipNo, person.UserID, person.Name, person.Department, person.Phone, external, purpose, source, operatorID, ip, submitID)
	if err != nil {
		log.Printf("[库房物资] 生成领用单失败: %v", err)
		return 0, &whCheckoutError{http.StatusInternalServerError, "登记失败，请稍后重试"}
	}
	slipID, _ := res.LastInsertId()

	for _, itemID := range order {
		qty := qtyByItem[itemID]
		var name, kind string
		var stock, status int
		if err := tx.QueryRow("SELECT name, kind, stock, status FROM warehouse_items WHERE id = ?", itemID).Scan(&name, &kind, &stock, &status); err != nil {
			return 0, &whCheckoutError{http.StatusBadRequest, "所选物资不存在，请刷新页面后重试"}
		}
		if status != 1 {
			return 0, &whCheckoutError{http.StatusBadRequest, "「" + name + "」已停用，请刷新页面后重试"}
		}
		if qty > whMaxLineQty {
			return 0, &whCheckoutError{http.StatusBadRequest, fmt.Sprintf("「%s」单次最多领用 %d", name, whMaxLineQty)}
		}
		if stock < qty {
			return 0, &whCheckoutError{http.StatusConflict, fmt.Sprintf("「%s」库存不足（现有 %d），请联系办公室", name, stock)}
		}
		if _, err := tx.Exec("UPDATE warehouse_items SET stock = stock - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", qty, itemID); err != nil {
			return 0, &whCheckoutError{http.StatusInternalServerError, "登记失败，请稍后重试"}
		}
		recKind := "out"
		if kind == "returnable" {
			recKind = "borrow"
		}
		if _, err := tx.Exec(`INSERT INTO warehouse_records (item_id, slip_id, kind, qty, stock_after, user_id, person_name, department, operator, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			itemID, slipID, recKind, -qty, stock-qty, person.UserID, person.Name, person.Department, operator); err != nil {
			return 0, &whCheckoutError{http.StatusInternalServerError, "登记失败，请稍后重试"}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, &whCheckoutError{http.StatusInternalServerError, "登记失败，请稍后重试"}
	}
	return slipID, nil
}

// whSlipView 领用单详情（公开端与后台共用，不含 IP 与电话）
func whSlipView(slipID int64) (map[string]interface{}, string) {
	var slipNo, name, dept, purpose string
	var voided int
	var createdAt sql.NullTime
	if err := database.DB.QueryRow(`SELECT slip_no, person_name, department, purpose, voided, created_at FROM warehouse_slips WHERE id = ?`, slipID).
		Scan(&slipNo, &name, &dept, &purpose, &voided, &createdAt); err != nil {
		return nil, ""
	}
	rows, err := database.DB.Query(`SELECT i.name, i.spec, i.unit, r.kind, -r.qty
		FROM warehouse_records r JOIN warehouse_items i ON r.item_id = i.id
		WHERE r.slip_id = ? AND r.kind IN ('out','borrow') ORDER BY r.id`, slipID)
	if err != nil {
		return nil, ""
	}
	defer rows.Close()
	lines := []map[string]interface{}{}
	summary := []string{}
	for rows.Next() {
		var iname, spec, unit, kind string
		var qty int
		if err := rows.Scan(&iname, &spec, &unit, &kind, &qty); err != nil {
			return nil, ""
		}
		lines = append(lines, map[string]interface{}{"name": iname, "spec": spec, "unit": unit, "kind": kind, "qty": qty})
		summary = append(summary, fmt.Sprintf("%s×%d%s", iname, qty, unit))
	}
	v := map[string]interface{}{
		"id": slipID, "slip_no": slipNo, "person_name": name, "department": dept,
		"purpose": purpose, "voided": voided == 1, "lines": lines,
	}
	if createdAt.Valid {
		v["created_at"] = createdAt.Time
	}
	return v, strings.Join(summary, "、")
}

// ---------------- 归还 ----------------

// whReturn 归还借用记录（tx 内执行）：库存加回、写归还流水、累加已归还数量
func whReturn(tx *sql.Tx, borrowID int64, qty int, operator string) (string, error) {
	var itemID, userID int64
	var borrowed, returned, voided int
	var person, dept, itemName, unit string
	err := tx.QueryRow(`SELECT r.item_id, -r.qty, r.returned_qty, r.voided, r.user_id, r.person_name, r.department, i.name, i.unit
		FROM warehouse_records r JOIN warehouse_items i ON r.item_id = i.id
		WHERE r.id = ? AND r.kind = 'borrow'`, borrowID).
		Scan(&itemID, &borrowed, &returned, &voided, &userID, &person, &dept, &itemName, &unit)
	if err != nil {
		return "", fmt.Errorf("借用记录不存在")
	}
	outstanding := borrowed - returned
	if voided == 1 || outstanding <= 0 {
		return "", fmt.Errorf("「%s」已归还，无需重复登记", itemName)
	}
	if qty <= 0 || qty > outstanding {
		qty = outstanding
	}
	if _, err := tx.Exec("UPDATE warehouse_items SET stock = stock + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", qty, itemID); err != nil {
		return "", err
	}
	var stock int
	if err := tx.QueryRow("SELECT stock FROM warehouse_items WHERE id = ?", itemID).Scan(&stock); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO warehouse_records (item_id, kind, qty, stock_after, borrow_id, user_id, person_name, department, operator, created_at)
		VALUES (?, 'return', ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		itemID, qty, stock, borrowID, userID, person, dept, operator); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`UPDATE warehouse_records SET returned_qty = returned_qty + ?,
		returned_at = CASE WHEN returned_qty + ? >= -qty THEN CURRENT_TIMESTAMP ELSE returned_at END WHERE id = ?`, qty, qty, borrowID); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s×%d%s", itemName, qty, unit), nil
}

// whOutstandingBorrows 借出未还列表；person 为空查全部
func whOutstandingBorrows(where string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := database.DB.Query(`SELECT r.id, r.item_id, i.name, i.spec, i.unit, -r.qty - r.returned_qty, -r.qty,
			r.user_id, r.person_name, r.department, IFNULL(s.slip_no, ''), IFNULL(s.is_external, 0), r.created_at
		FROM warehouse_records r
		JOIN warehouse_items i ON r.item_id = i.id
		LEFT JOIN warehouse_slips s ON r.slip_id = s.id
		WHERE r.kind = 'borrow' AND r.voided = 0 AND -r.qty > r.returned_qty `+where+`
		ORDER BY r.id LIMIT 2000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var id, itemID, userID int64
		var name, spec, unit, person, dept, slipNo string
		var outstanding, borrowed, external int
		var createdAt sql.NullTime
		if err := rows.Scan(&id, &itemID, &name, &spec, &unit, &outstanding, &borrowed, &userID, &person, &dept, &slipNo, &external, &createdAt); err != nil {
			return nil, err
		}
		v := map[string]interface{}{
			"id": id, "item_id": itemID, "item_name": name, "spec": spec, "unit": unit,
			"outstanding": outstanding, "borrowed": borrowed, "user_id": userID,
			"person_name": person, "department": dept, "slip_no": slipNo, "external": external == 1,
		}
		if createdAt.Valid {
			v["created_at"] = createdAt.Time
			v["days"] = int(time.Since(createdAt.Time).Hours() / 24)
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

// whPersonBorrowFilter 按领用人限定借用记录：名单内按 user_id，其他人员按姓名+电话
func whPersonBorrowFilter(p whResolvedPerson) (string, []interface{}) {
	if !p.External {
		return " AND r.user_id = ?", []interface{}{p.UserID}
	}
	return " AND r.user_id = 0 AND s.is_external = 1 AND s.person_name = ? AND s.phone = ?", []interface{}{p.Name, p.Phone}
}

// ---------------- 公开端（门口二维码，免登录） ----------------

// PublicWarehouse 加载扫码页：人员名单与可领物资
func PublicWarehouse(w http.ResponseWriter, r *http.Request) {
	s, ok := whPublicAuth(w, r)
	if !ok {
		return
	}
	if s.Paused {
		middleware.JSON(w, http.StatusOK, map[string]interface{}{"paused": true, "notice": s.Notice})
		return
	}
	people, err := loadWhPeople()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "加载失败，请稍后重试"})
		return
	}
	rows, err := database.DB.Query(`SELECT id, name, category, spec, unit, kind, stock FROM warehouse_items
		WHERE status = 1 ORDER BY sort, id LIMIT 2000`)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "加载失败，请稍后重试"})
		return
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var name, category, spec, unit, kind string
		var stock int
		if err := rows.Scan(&id, &name, &category, &spec, &unit, &kind, &stock); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "加载失败，请稍后重试"})
			return
		}
		items = append(items, map[string]interface{}{
			"id": id, "name": name, "category": category, "spec": spec, "unit": unit, "kind": kind, "stock": stock,
		})
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "加载失败，请稍后重试"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"paused": false, "notice": s.Notice, "departments": people, "items": items,
	})
}

// PublicWarehouseCheckout 扫码提交领用/借用
func PublicWarehouseCheckout(w http.ResponseWriter, r *http.Request) {
	s, ok := whPublicAuth(w, r)
	if !ok {
		return
	}
	if s.Paused {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "扫码领用已暂停，请联系办公室"})
		return
	}
	var req whCheckoutReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	person, msg := req.whPersonReq.resolve()
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	slipID, cerr := whCheckout(req, person, "public", 0, "本人扫码", clientIP(r))
	if cerr != nil {
		middleware.JSON(w, cerr.status, map[string]string{"error": cerr.msg})
		return
	}
	slip, summary := whSlipView(slipID)
	if slip == nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "登记成功，但读取领用单失败"})
		return
	}
	logWithUserDept(person.UserID, person.Name, person.Department, "库房物资", "扫码领用", fmt.Sprintf("领用单%s：%s", slip["slip_no"], summary), clientIP(r))
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "登记成功", "slip": slip})
}

// PublicWarehouseBorrows 查询本人借出未还的物品（用 POST 传身份，避免电话出现在网址中）
func PublicWarehouseBorrows(w http.ResponseWriter, r *http.Request) {
	s, ok := whPublicAuth(w, r)
	if !ok {
		return
	}
	if s.Paused {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "扫码领用已暂停，请联系办公室"})
		return
	}
	var req whPersonReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	person, msg := req.resolve()
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	where, args := whPersonBorrowFilter(person)
	list, err := whOutstandingBorrows(where, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败，请稍后重试"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list})
}

// PublicWarehouseReturn 本人扫码登记归还
func PublicWarehouseReturn(w http.ResponseWriter, r *http.Request) {
	s, ok := whPublicAuth(w, r)
	if !ok {
		return
	}
	if s.Paused {
		middleware.JSON(w, http.StatusForbidden, map[string]string{"error": "扫码领用已暂停，请联系办公室"})
		return
	}
	var req struct {
		whPersonReq
		BorrowIDs []int64 `json:"borrow_ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	person, msg := req.whPersonReq.resolve()
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if len(req.BorrowIDs) == 0 || len(req.BorrowIDs) > 100 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择要归还的物品"})
		return
	}
	// 只能归还本人名下的借用记录
	where, args := whPersonBorrowFilter(person)
	mine, err := whOutstandingBorrows(where, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败，请稍后重试"})
		return
	}
	owned := map[int64]bool{}
	for _, b := range mine {
		owned[b["id"].(int64)] = true
	}
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "系统繁忙，请稍后重试"})
		return
	}
	defer tx.Rollback()
	done := []string{}
	for _, id := range req.BorrowIDs {
		if !owned[id] {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "所选物品不在您的借用记录中，请刷新后重试"})
			return
		}
		desc, err := whReturn(tx, id, 0, "本人扫码")
		if err != nil {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		done = append(done, desc)
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "登记失败，请稍后重试"})
		return
	}
	logWithUserDept(person.UserID, person.Name, person.Department, "库房物资", "扫码归还", "归还："+strings.Join(done, "、"), clientIP(r))
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "归还登记成功", "returned": done})
}

// ---------------- 后台：物资台账 ----------------

// whSummary 页头统计（全量口径）
func whSummary() map[string]int {
	var items, low, monthSlips, borrowed int
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_items WHERE status = 1").Scan(&items)
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_items WHERE status = 1 AND warn_qty > 0 AND stock <= warn_qty").Scan(&low)
	if start, end, ok := whMonthRange(time.Now().Format("2006-01")); ok {
		database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_slips WHERE voided = 0 AND created_at >= ? AND created_at < ?", start, end).Scan(&monthSlips)
	}
	database.DB.QueryRow("SELECT COALESCE(SUM(-qty - returned_qty), 0) FROM warehouse_records WHERE kind = 'borrow' AND voided = 0").Scan(&borrowed)
	return map[string]int{"items": items, "low": low, "month_slips": monthSlips, "borrowed": borrowed}
}

// ListWarehouseItems 物资台账（不分页，上限 2000）
func ListWarehouseItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := " WHERE 1=1"
	args := []interface{}{}
	if kw := strings.TrimSpace(q.Get("keyword")); kw != "" {
		where += " AND (i.name LIKE ? OR i.spec LIKE ? OR i.location LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if c := strings.TrimSpace(q.Get("category")); c != "" {
		where += " AND i.category = ?"
		args = append(args, c)
	}
	switch q.Get("status") {
	case "1":
		where += " AND i.status = 1"
	case "0":
		where += " AND i.status = 0"
	}
	if q.Get("low") == "1" {
		where += " AND i.status = 1 AND i.warn_qty > 0 AND i.stock <= i.warn_qty"
	}
	rows, err := database.DB.Query(`SELECT i.id, i.name, i.category, i.spec, i.unit, i.kind, i.stock, i.warn_qty, i.location, i.status, i.sort,
			i.created_at, i.updated_at,
			(SELECT COALESCE(SUM(-r.qty - r.returned_qty), 0) FROM warehouse_records r WHERE r.item_id = i.id AND r.kind = 'borrow' AND r.voided = 0)
		FROM warehouse_items i`+where+` ORDER BY i.status DESC, i.sort, i.id LIMIT 2000`, args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	list := []models.WarehouseItem{}
	for rows.Next() {
		var it models.WarehouseItem
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&it.ID, &it.Name, &it.Category, &it.Spec, &it.Unit, &it.Kind, &it.Stock, &it.WarnQty, &it.Location, &it.Status, &it.Sort,
			&createdAt, &updatedAt, &it.Borrowed); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
		if createdAt.Valid {
			it.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			it.UpdatedAt = updatedAt.Time
		}
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{
		"list": list, "summary": whSummary(), "categories": whCategories(),
	})
}

// whCategories 已用分类（供筛选与新增时选择）
func whCategories() []string {
	out := []string{}
	rows, err := database.DB.Query("SELECT DISTINCT category FROM warehouse_items WHERE category <> '' ORDER BY category LIMIT 200")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	return out
}

// validateWhItem 校验并规整物资字段
func validateWhItem(it *models.WarehouseItem) string {
	it.Name = strings.TrimSpace(it.Name)
	it.Category = strings.TrimSpace(it.Category)
	it.Spec = strings.TrimSpace(it.Spec)
	it.Unit = strings.TrimSpace(it.Unit)
	it.Location = strings.TrimSpace(it.Location)
	if it.Name == "" {
		return "请填写物资名称"
	}
	if it.Unit == "" {
		it.Unit = "个"
	}
	if len([]rune(it.Name)) > 50 || len([]rune(it.Category)) > 20 || len([]rune(it.Spec)) > 50 ||
		len([]rune(it.Unit)) > 10 || len([]rune(it.Location)) > 50 {
		return "填写内容过长"
	}
	if it.Kind != "consumable" && it.Kind != "returnable" {
		return "请选择物资类型"
	}
	if it.WarnQty < 0 || it.WarnQty > 100000 {
		return "预警数量不正确"
	}
	return ""
}

// CreateWarehouseItem 新增物资（可带初始库存）
func CreateWarehouseItem(w http.ResponseWriter, r *http.Request) {
	var it models.WarehouseItem
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if msg := validateWhItem(&it); msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if it.Stock < 0 || it.Stock > 100000 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "初始库存不正确"})
		return
	}
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	defer tx.Rollback()
	var dup int
	tx.QueryRow("SELECT COUNT(*) FROM warehouse_items WHERE name = ? AND spec = ?", it.Name, it.Spec).Scan(&dup)
	if dup > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "已存在同名同规格的物资"})
		return
	}
	res, err := tx.Exec(`INSERT INTO warehouse_items (name, category, spec, unit, kind, stock, warn_qty, location, status, sort, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		it.Name, it.Category, it.Spec, it.Unit, it.Kind, it.Stock, it.WarnQty, it.Location, it.Sort)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	id, _ := res.LastInsertId()
	if it.Stock > 0 {
		if _, err := tx.Exec(`INSERT INTO warehouse_records (item_id, kind, qty, stock_after, operator, remark, created_at)
			VALUES (?, 'in', ?, ?, ?, '初始库存', CURRENT_TIMESTAMP)`, id, it.Stock, it.Stock, whOperatorName(r)); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	logOperation(r, "库房物资", "新增", fmt.Sprintf("新增物资「%s」，初始库存 %d%s", it.Name, it.Stock, it.Unit))
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "已新增", "id": id})
}

// UpdateWarehouseItem 修改物资信息（不改库存；库存只能通过入库/盘点变动）
func UpdateWarehouseItem(w http.ResponseWriter, r *http.Request) {
	var it models.WarehouseItem
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil || it.ID <= 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if msg := validateWhItem(&it); msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if it.Status != 0 {
		it.Status = 1
	}
	var oldKind string
	if err := database.DB.QueryRow("SELECT kind FROM warehouse_items WHERE id = ?", it.ID).Scan(&oldKind); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "物资不存在"})
		return
	}
	// 有借出未还时不允许把可归还物品改成耗材，避免借用记录无法归还
	if oldKind == "returnable" && it.Kind == "consumable" {
		var out int
		database.DB.QueryRow("SELECT COALESCE(SUM(-qty - returned_qty), 0) FROM warehouse_records WHERE item_id = ? AND kind = 'borrow' AND voided = 0", it.ID).Scan(&out)
		if out > 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该物品还有借出未还的记录，归还后才能改为耗材"})
			return
		}
	}
	var dup int
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_items WHERE name = ? AND spec = ? AND id <> ?", it.Name, it.Spec, it.ID).Scan(&dup)
	if dup > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "已存在同名同规格的物资"})
		return
	}
	if _, err := database.DB.Exec(`UPDATE warehouse_items SET name=?, category=?, spec=?, unit=?, kind=?, warn_qty=?, location=?, status=?, sort=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		it.Name, it.Category, it.Spec, it.Unit, it.Kind, it.WarnQty, it.Location, it.Status, it.Sort, it.ID); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	logOperation(r, "库房物资", "修改", "修改物资「"+it.Name+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已保存"})
}

// DeleteWarehouseItem 删除物资：已有出入库流水的只能停用
func DeleteWarehouseItem(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var name string
	if err := database.DB.QueryRow("SELECT name FROM warehouse_items WHERE id = ?", id).Scan(&name); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "物资不存在"})
		return
	}
	var n int
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_records WHERE item_id = ?", id).Scan(&n)
	if n > 0 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该物资已有出入库记录，不能删除，可改为停用"})
		return
	}
	if _, err := database.DB.Exec("DELETE FROM warehouse_items WHERE id = ?", id); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败"})
		return
	}
	resetAutoIncrement()
	logOperation(r, "库房物资", "删除", "删除物资「"+name+"」")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// whStockChange 入库/盘点的通用事务：mode=in 加库存，mode=adjust 按实点数调整
func whStockChange(w http.ResponseWriter, r *http.Request, mode string) {
	id := pathID(r)
	var req struct {
		Qty    int    `json:"qty"`
		Actual int    `json:"actual"`
		Remark string `json:"remark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	req.Remark = strings.TrimSpace(req.Remark)
	if len([]rune(req.Remark)) > 100 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "备注请控制在 100 字以内"})
		return
	}
	if mode == "in" && (req.Qty <= 0 || req.Qty > 100000) {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写正确的入库数量"})
		return
	}
	if mode == "adjust" {
		if req.Actual < 0 || req.Actual > 1000000 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写正确的实点数量"})
			return
		}
		if req.Remark == "" {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写盘点调整原因"})
			return
		}
	}
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	defer tx.Rollback()
	var name, unit string
	var stock int
	if err := tx.QueryRow("SELECT name, unit, stock FROM warehouse_items WHERE id = ?", id).Scan(&name, &unit, &stock); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "物资不存在"})
		return
	}
	delta := req.Qty
	if mode == "adjust" {
		delta = req.Actual - stock
		if delta == 0 {
			middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "实点数量与系统库存一致，无需调整"})
			return
		}
	}
	if _, err := tx.Exec("UPDATE warehouse_items SET stock = stock + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", delta, id); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	if _, err := tx.Exec(`INSERT INTO warehouse_records (item_id, kind, qty, stock_after, operator, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, id, mode, delta, stock+delta, whOperatorName(r), req.Remark); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	if mode == "in" {
		logOperation(r, "库房物资", "入库", fmt.Sprintf("「%s」入库 %d%s，现有 %d", name, delta, unit, stock+delta))
	} else {
		logOperation(r, "库房物资", "盘点", fmt.Sprintf("「%s」盘点 %d → %d%s（%s）", name, stock, stock+delta, unit, req.Remark))
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "已保存", "stock": stock + delta})
}

// StockInWarehouseItem 入库
func StockInWarehouseItem(w http.ResponseWriter, r *http.Request) { whStockChange(w, r, "in") }

// AdjustWarehouseItem 盘点调整
func AdjustWarehouseItem(w http.ResponseWriter, r *http.Request) { whStockChange(w, r, "adjust") }

// ---------------- 后台：流水、作废、补录、借出未还 ----------------

// whRecordFilter 流水筛选条件（列表与导出共用）
func whRecordFilter(r *http.Request) (string, []interface{}, string) {
	q := r.URL.Query()
	where := " WHERE 1=1"
	args := []interface{}{}
	if m := strings.TrimSpace(q.Get("month")); m != "" {
		start, end, ok := whMonthRange(m)
		if !ok {
			return "", nil, "月份格式不正确"
		}
		where += " AND r.created_at >= ? AND r.created_at < ?"
		args = append(args, start, end)
	}
	if k := q.Get("kind"); k != "" {
		if _, ok := whKindLabel[k]; !ok {
			return "", nil, "类型不正确"
		}
		where += " AND r.kind = ?"
		args = append(args, k)
	}
	if d := strings.TrimSpace(q.Get("department")); d != "" {
		where += " AND r.department = ?"
		args = append(args, d)
	}
	if id, _ := strconv.ParseInt(q.Get("item_id"), 10, 64); id > 0 {
		where += " AND r.item_id = ?"
		args = append(args, id)
	}
	if kw := strings.TrimSpace(q.Get("keyword")); kw != "" {
		where += " AND (r.person_name LIKE ? OR i.name LIKE ? OR s.slip_no LIKE ? OR r.remark LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	return where, args, ""
}

const whRecordSelect = `SELECT r.id, r.item_id, i.name, i.spec, i.unit, r.kind, r.qty, r.stock_after, r.slip_id, IFNULL(s.slip_no, ''),
		r.user_id, r.person_name, r.department, IFNULL(s.phone, ''), IFNULL(s.is_external, 0), IFNULL(s.purpose, ''), IFNULL(s.source, ''),
		r.operator, r.remark, r.voided, IFNULL(s.void_reason, ''), r.returned_qty, r.created_at
	FROM warehouse_records r
	JOIN warehouse_items i ON r.item_id = i.id
	LEFT JOIN warehouse_slips s ON r.slip_id = s.id`

func scanWhRecord(rows *sql.Rows) (models.WarehouseRecord, error) {
	var rec models.WarehouseRecord
	var external, voided int
	var createdAt sql.NullTime
	err := rows.Scan(&rec.ID, &rec.ItemID, &rec.ItemName, &rec.Spec, &rec.Unit, &rec.Kind, &rec.Qty, &rec.StockAfter, &rec.SlipID, &rec.SlipNo,
		&rec.UserID, &rec.PersonName, &rec.Department, &rec.Phone, &external, &rec.Purpose, &rec.Source,
		&rec.Operator, &rec.Remark, &voided, &rec.VoidReason, &rec.ReturnedQty, &createdAt)
	rec.External = external == 1
	rec.Voided = voided == 1
	if createdAt.Valid {
		rec.CreatedAt = createdAt.Time
	}
	return rec, err
}

// ListWarehouseRecords 出入库流水（分页）
func ListWarehouseRecords(w http.ResponseWriter, r *http.Request) {
	where, args, msg := whRecordFilter(r)
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	pp := parsePage(r)
	var total int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM warehouse_records r JOIN warehouse_items i ON r.item_id = i.id
		LEFT JOIN warehouse_slips s ON r.slip_id = s.id`+where, args...).Scan(&total); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	rows, err := database.DB.Query(whRecordSelect+where+" ORDER BY r.id DESC LIMIT ? OFFSET ?",
		append(args, pp.PageSize, (pp.Page-1)*pp.PageSize)...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	defer rows.Close()
	list := []models.WarehouseRecord{}
	for rows.Next() {
		rec, err := scanWhRecord(rows)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "读取数据失败"})
			return
		}
		list = append(list, rec)
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	result := paginateResult(list, total, pp.Page, pp.PageSize)
	result["summary"] = whSummary()
	middleware.JSON(w, http.StatusOK, result)
}

// VoidWarehouseSlip 作废领用单：未归还部分加回库存，流水标记作废
func VoidWarehouseSlip(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || len([]rune(req.Reason)) > 100 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请填写作废原因（100 字以内）"})
		return
	}
	operator := whOperatorName(r)
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	defer tx.Rollback()
	var slipNo, person string
	var voided int
	if err := tx.QueryRow("SELECT slip_no, person_name, voided FROM warehouse_slips WHERE id = ?", id).Scan(&slipNo, &person, &voided); err != nil {
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "领用单不存在"})
		return
	}
	if voided == 1 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "该领用单已作废"})
		return
	}
	type line struct {
		id, itemID   int64
		restore      int
		userID       int64
		person, dept string
	}
	rows, err := tx.Query(`SELECT id, item_id, -qty - returned_qty, user_id, person_name, department FROM warehouse_records
		WHERE slip_id = ? AND kind IN ('out','borrow') AND voided = 0`, id)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.id, &l.itemID, &l.restore, &l.userID, &l.person, &l.dept); err != nil {
			rows.Close()
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
			return
		}
		lines = append(lines, l)
	}
	rows.Close()
	for _, l := range lines {
		if l.restore > 0 {
			if _, err := tx.Exec("UPDATE warehouse_items SET stock = stock + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", l.restore, l.itemID); err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
				return
			}
			var stock int
			tx.QueryRow("SELECT stock FROM warehouse_items WHERE id = ?", l.itemID).Scan(&stock)
			if _, err := tx.Exec(`INSERT INTO warehouse_records (item_id, slip_id, kind, qty, stock_after, user_id, person_name, department, operator, remark, created_at)
				VALUES (?, ?, 'void', ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
				l.itemID, id, l.restore, stock, l.userID, l.person, l.dept, operator, "作废领用单"+slipNo+"："+req.Reason); err != nil {
				middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
				return
			}
		}
		if _, err := tx.Exec("UPDATE warehouse_records SET voided = 1 WHERE id = ?", l.id); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
			return
		}
	}
	if _, err := tx.Exec("UPDATE warehouse_slips SET voided = 1, void_reason = ?, voided_by = ?, voided_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.Reason, operator, id); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	logOperation(r, "库房物资", "作废", fmt.Sprintf("作废领用单%s（领用人%s）：%s", slipNo, person, req.Reason))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已作废，库存已加回"})
}

// AdminWarehouseCheckout 办公室后台代为补录领用
func AdminWarehouseCheckout(w http.ResponseWriter, r *http.Request) {
	var req whCheckoutReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	person, msg := req.whPersonReq.resolve()
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	operatorID, _ := r.Context().Value(middleware.ContextUserID).(int64)
	slipID, cerr := whCheckout(req, person, "admin", operatorID, whOperatorName(r), clientIP(r))
	if cerr != nil {
		middleware.JSON(w, cerr.status, map[string]string{"error": cerr.msg})
		return
	}
	slip, summary := whSlipView(slipID)
	logOperation(r, "库房物资", "补录领用", fmt.Sprintf("为%s（%s）补录领用：%s", person.Name, person.Department, summary))
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"message": "已登记", "slip": slip})
}

// ListWarehousePeople 补录领用时选人
func ListWarehousePeople(w http.ResponseWriter, r *http.Request) {
	people, err := loadWhPeople()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"departments": people})
}

// ListWarehouseBorrows 全部借出未还
func ListWarehouseBorrows(w http.ResponseWriter, r *http.Request) {
	list, err := whOutstandingBorrows("")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"list": list, "summary": whSummary()})
}

// AdminReturnWarehouseBorrow 办公室代为登记归还（可部分归还）
func AdminReturnWarehouseBorrow(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var req struct {
		Qty int `json:"qty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	tx, err := database.DB.Begin()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	defer tx.Rollback()
	var person string
	tx.QueryRow("SELECT person_name FROM warehouse_records WHERE id = ?", id).Scan(&person)
	desc, err := whReturn(tx, id, req.Qty, whOperatorName(r))
	if err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := tx.Commit(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	logOperation(r, "库房物资", "代为归还", fmt.Sprintf("代%s登记归还：%s", person, desc))
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已登记归还"})
}

// ---------------- 后台：统计 ----------------

type whStatRow struct {
	Department string `json:"department"`
	ItemName   string `json:"item_name"`
	Spec       string `json:"spec"`
	Unit       string `json:"unit"`
	OutQty     int    `json:"out_qty"`
	BorrowQty  int    `json:"borrow_qty"`
	Times      int    `json:"times"`
}

// whStats 统计区间内领用/借用（不含已作废）；groupBy = department | item | department_item
func whStats(start, end, groupBy string) ([]whStatRow, error) {
	var sel, group, order string
	switch groupBy {
	case "department":
		sel, group, order = "r.department, '', '', ''", "r.department", "SUM(-r.qty) DESC"
	case "item":
		sel, group, order = "'', i.name, i.spec, i.unit", "r.item_id", "SUM(-r.qty) DESC"
	default:
		sel, group, order = "r.department, i.name, i.spec, i.unit", "r.department, r.item_id", "r.department, i.sort, i.name"
	}
	rows, err := database.DB.Query(`SELECT `+sel+`,
			COALESCE(SUM(CASE WHEN r.kind = 'out' THEN -r.qty ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN r.kind = 'borrow' THEN -r.qty ELSE 0 END), 0),
			COUNT(DISTINCT r.slip_id)
		FROM warehouse_records r JOIN warehouse_items i ON r.item_id = i.id
		WHERE r.kind IN ('out','borrow') AND r.voided = 0 AND r.created_at >= ? AND r.created_at < ?
		GROUP BY `+group+` ORDER BY `+order+` LIMIT 10000`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []whStatRow{}
	for rows.Next() {
		var s whStatRow
		if err := rows.Scan(&s.Department, &s.ItemName, &s.Spec, &s.Unit, &s.OutQty, &s.BorrowQty, &s.Times); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// WarehouseStats 按科室、按物资统计
func WarehouseStats(w http.ResponseWriter, r *http.Request) {
	start, end, ok := whDateRange(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if !ok {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择正确的统计起止日期"})
		return
	}
	byDept, err1 := whStats(start, end, "department")
	byItem, err2 := whStats(start, end, "item")
	if err1 != nil || err2 != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "统计失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"by_department": byDept, "by_item": byItem})
}

// ---------------- 后台：二维码设置 ----------------

// GetWarehouseSettings 查看门口二维码设置
func GetWarehouseSettings(w http.ResponseWriter, r *http.Request) {
	s, err := loadWhSettings()
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败"})
		return
	}
	middleware.JSON(w, http.StatusOK, map[string]interface{}{"public_token": s.Token, "paused": s.Paused, "notice": s.Notice})
}

// UpdateWarehouseSettings 暂停/恢复扫码领用、修改页面提示语
func UpdateWarehouseSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paused bool   `json:"paused"`
		Notice string `json:"notice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	req.Notice = strings.TrimSpace(req.Notice)
	if len([]rune(req.Notice)) > 200 {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "提示语请控制在 200 字以内"})
		return
	}
	if _, err := loadWhSettings(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	paused := 0
	if req.Paused {
		paused = 1
	}
	if _, err := database.DB.Exec("UPDATE warehouse_settings SET paused = ?, notice = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", paused, req.Notice); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "保存失败"})
		return
	}
	state := "开放"
	if req.Paused {
		state = "暂停"
	}
	logOperation(r, "库房物资", "修改", "扫码领用设置为"+state)
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已保存"})
}

// ResetWarehouseToken 重置门口二维码，旧码立即失效
func ResetWarehouseToken(w http.ResponseWriter, r *http.Request) {
	if _, err := loadWhSettings(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	token := newPublicToken()
	if _, err := database.DB.Exec("UPDATE warehouse_settings SET public_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", token); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "操作失败"})
		return
	}
	logOperation(r, "库房物资", "重置二维码", "重置库房门口二维码，旧二维码作废")
	middleware.JSON(w, http.StatusOK, map[string]string{"message": "已重置，请重新打印张贴", "public_token": token})
}

// ---------------- 导出 ----------------

// ExportWarehouseRecords 导出出入库流水
func ExportWarehouseRecords(w http.ResponseWriter, r *http.Request) {
	where, args, msg := whRecordFilter(r)
	if msg != "" {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	rows, err := database.DB.Query(whRecordSelect+where+" ORDER BY r.id DESC LIMIT 10000", args...)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
		return
	}
	defer rows.Close()
	data := [][]interface{}{}
	for rows.Next() {
		rec, err := scanWhRecord(rows)
		if err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
			return
		}
		state := ""
		if rec.Voided {
			state = "已作废"
		}
		person := rec.PersonName
		if rec.External {
			person += "（其他人员）"
		}
		source := map[string]string{"public": "扫码", "admin": "后台补录"}[rec.Source]
		note := rec.Purpose
		if rec.Remark != "" {
			if note != "" {
				note += "；"
			}
			note += rec.Remark
		}
		data = append(data, []interface{}{
			formatDateTime(rec.CreatedAt), rec.SlipNo, whKindLabel[rec.Kind], rec.ItemName, rec.Spec, rec.Qty, rec.Unit, rec.StockAfter,
			person, rec.Department, rec.Phone, note, source, rec.Operator, state,
		})
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
		return
	}
	headers := []string{"时间", "领用单号", "类型", "物资", "规格", "库存变动", "单位", "变动后库存", "领用人", "科室/单位", "联系电话", "用途/备注", "来源", "经办", "状态"}
	logOperation(r, "库房物资", "导出", "导出库房出入库流水")
	exportExcel(w, "出入库流水", "库房出入库流水.xlsx", headers, data)
}

// ExportWarehouseItems 导出库存台账
func ExportWarehouseItems(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`SELECT i.name, i.category, i.spec, i.unit, i.kind, i.stock, i.warn_qty, i.location, i.status,
			(SELECT COALESCE(SUM(-r.qty - r.returned_qty), 0) FROM warehouse_records r WHERE r.item_id = i.id AND r.kind = 'borrow' AND r.voided = 0)
		FROM warehouse_items i ORDER BY i.status DESC, i.sort, i.id LIMIT 10000`)
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
		return
	}
	defer rows.Close()
	data := [][]interface{}{}
	for rows.Next() {
		var name, category, spec, unit, kind, location string
		var stock, warn, status, borrowed int
		if err := rows.Scan(&name, &category, &spec, &unit, &kind, &stock, &warn, &location, &status, &borrowed); err != nil {
			middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
			return
		}
		kindLabel, state, warnFlag := "耗材", "启用", ""
		if kind == "returnable" {
			kindLabel = "可归还"
		}
		if status != 1 {
			state = "停用"
		}
		if status == 1 && warn > 0 && stock <= warn {
			warnFlag = "低于预警"
		}
		data = append(data, []interface{}{name, category, spec, unit, kindLabel, stock, borrowed, warn, warnFlag, location, state})
	}
	if err := rows.Err(); err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
		return
	}
	headers := []string{"物资名称", "分类", "规格", "单位", "类型", "库存", "借出未还", "预警数量", "预警", "存放位置", "状态"}
	logOperation(r, "库房物资", "导出", "导出库房库存台账")
	exportExcel(w, "库存台账", "库房库存台账"+time.Now().Format("20060102")+".xlsx", headers, data)
}

// ExportWarehouseStats 导出科室领用汇总（科室 × 物资）
func ExportWarehouseStats(w http.ResponseWriter, r *http.Request) {
	startS, endS := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	start, end, ok := whDateRange(startS, endS)
	if !ok {
		middleware.JSON(w, http.StatusBadRequest, map[string]string{"error": "请选择正确的统计起止日期"})
		return
	}
	list, err := whStats(start, end, "department_item")
	if err != nil {
		middleware.JSON(w, http.StatusInternalServerError, map[string]string{"error": "导出失败"})
		return
	}
	data := [][]interface{}{}
	for _, s := range list {
		data = append(data, []interface{}{s.Department, s.ItemName, s.Spec, s.Unit, s.OutQty, s.BorrowQty, s.Times})
	}
	headers := []string{"科室/单位", "物资", "规格", "单位", "领用数量", "借用数量", "领用次数"}
	logOperation(r, "库房物资", "导出", fmt.Sprintf("导出科室领用汇总（%s 至 %s）", startS, endS))
	exportExcel(w, "科室领用汇总", fmt.Sprintf("库房科室领用汇总%s至%s.xlsx", startS, endS), headers, data)
}
