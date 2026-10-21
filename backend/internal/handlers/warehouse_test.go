package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ynxwxcb-platform/internal/database"
)

func whCall(t *testing.T, h http.HandlerFunc, method, path, body string, pathVals map[string]string, admin bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	for k, v := range pathVals {
		req.SetPathValue(k, v)
	}
	if admin {
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func whStock(t *testing.T, id int64) int {
	t.Helper()
	var s int
	if err := database.DB.QueryRow("SELECT stock FROM warehouse_items WHERE id=?", id).Scan(&s); err != nil {
		t.Fatalf("读取库存失败: %v", err)
	}
	return s
}

func whNewItem(t *testing.T, name, kind string, stock int) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"category":"办公耗材","unit":"个","kind":%q,"stock":%d,"warn_qty":2}`, name, kind, stock)
	rec := whCall(t, CreateWarehouseItem, "POST", "/api/warehouse/items", body, nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("新增物资失败: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	return res.ID
}

func TestWarehouseWorkflow(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	s, err := loadWhSettings()
	if err != nil || len(s.Token) != 32 {
		t.Fatalf("扫码令牌未初始化: %v %q", err, s.Token)
	}
	tok := map[string]string{"token": s.Token}

	pen := whNewItem(t, "中性笔", "consumable", 10)
	camera := whNewItem(t, "相机", "returnable", 1)

	// 错误令牌一律 404
	if rec := whCall(t, PublicWarehouse, "GET", "/", "", map[string]string{"token": "0123456789abcdef0123456789abcdef"}, false); rec.Code != http.StatusNotFound {
		t.Fatalf("错误令牌应 404，实际 %d", rec.Code)
	}

	// 公开页只返回姓名，不含手机号与账号；系统管理员账号不列入
	rec := whCall(t, PublicWarehouse, "GET", "/", "", tok, false)
	if rec.Code != http.StatusOK || bytes.Contains(rec.Body.Bytes(), []byte("staff1")) {
		t.Fatalf("公开页加载异常: %d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("系统管理员")) {
		t.Fatalf("公开页名单不应出现系统管理员账号: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("李干事")) {
		t.Fatalf("公开页名单缺少在册人员: %s", rec.Body.String())
	}

	// 名单内人员领用 3 支笔 + 借相机；同一 submit_id 重复提交只生成一张单
	body := fmt.Sprintf(`{"user_id":2,"purpose":"会议用","submit_id":"abc","lines":[{"item_id":%d,"qty":3},{"item_id":%d,"qty":1}]}`, pen, camera)
	for i := 0; i < 2; i++ {
		if rec := whCall(t, PublicWarehouseCheckout, "POST", "/", body, tok, false); rec.Code != http.StatusOK {
			t.Fatalf("扫码领用失败: %d %s", rec.Code, rec.Body.String())
		}
	}
	var slips int
	database.DB.QueryRow("SELECT COUNT(*) FROM warehouse_slips").Scan(&slips)
	if slips != 1 || whStock(t, pen) != 7 || whStock(t, camera) != 0 {
		t.Fatalf("领用后数据不对: 单数 %d 笔 %d 相机 %d", slips, whStock(t, pen), whStock(t, camera))
	}
	var dept string
	database.DB.QueryRow("SELECT department FROM warehouse_slips LIMIT 1").Scan(&dept)
	if dept != "新闻宣传科" {
		t.Fatalf("科室应以数据库为准，实际 %q", dept)
	}

	// 相机已借空，再借应提示库存不足
	body = fmt.Sprintf(`{"user_id":2,"lines":[{"item_id":%d,"qty":1}]}`, camera)
	if rec := whCall(t, PublicWarehouseCheckout, "POST", "/", body, tok, false); rec.Code != http.StatusConflict {
		t.Fatalf("库存不足应 409，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 其他人员：电话格式校验
	body = fmt.Sprintf(`{"external":true,"name":"王借调","unit":"县融媒体中心","phone":"123","lines":[{"item_id":%d,"qty":1}]}`, pen)
	if rec := whCall(t, PublicWarehouseCheckout, "POST", "/", body, tok, false); rec.Code != http.StatusBadRequest {
		t.Fatalf("错误电话应被拦截，实际 %d", rec.Code)
	}

	// 并发抢最后几支笔：库存 7，20 人各领 1 支 → 恰好 7 人成功，库存不为负
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := fmt.Sprintf(`{"external":true,"name":"并发%d","unit":"测试单位","phone":"13800000000","lines":[{"item_id":%d,"qty":1}]}`, i, pen)
			req := httptest.NewRequest("POST", "/", bytes.NewBufferString(b))
			req.SetPathValue("token", s.Token)
			rec := httptest.NewRecorder()
			PublicWarehouseCheckout(rec, req)
			if rec.Code == http.StatusOK {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if okCount != 7 || whStock(t, pen) != 0 {
		t.Fatalf("并发领用异常: 成功 %d 库存 %d", okCount, whStock(t, pen))
	}

	// 归还：别人不能归还李干事的借用；本人可以
	var borrowID int64
	database.DB.QueryRow("SELECT id FROM warehouse_records WHERE kind='borrow'").Scan(&borrowID)
	other := fmt.Sprintf(`{"external":true,"name":"并发1","phone":"13800000000","unit":"测试单位","borrow_ids":[%d]}`, borrowID)
	if rec := whCall(t, PublicWarehouseReturn, "POST", "/", other, tok, false); rec.Code != http.StatusBadRequest {
		t.Fatalf("他人归还应被拦截，实际 %d", rec.Code)
	}
	rec = whCall(t, PublicWarehouseBorrows, "POST", "/", `{"user_id":2}`, tok, false)
	if !bytes.Contains(rec.Body.Bytes(), []byte("相机")) {
		t.Fatalf("本人借用列表应含相机: %s", rec.Body.String())
	}
	mine := fmt.Sprintf(`{"user_id":2,"borrow_ids":[%d]}`, borrowID)
	if rec := whCall(t, PublicWarehouseReturn, "POST", "/", mine, tok, false); rec.Code != http.StatusOK {
		t.Fatalf("本人归还失败: %d %s", rec.Code, rec.Body.String())
	}
	if whStock(t, camera) != 1 {
		t.Fatalf("归还后相机库存应为 1，实际 %d", whStock(t, camera))
	}
	if rec := whCall(t, PublicWarehouseReturn, "POST", "/", mine, tok, false); rec.Code != http.StatusBadRequest {
		t.Fatalf("重复归还应被拦截，实际 %d", rec.Code)
	}

	// 作废李干事的领用单：笔加回 3，相机已归还不再加
	var slipID int64
	database.DB.QueryRow("SELECT id FROM warehouse_slips WHERE user_id=2").Scan(&slipID)
	if rec := whCall(t, VoidWarehouseSlip, "POST", "/", `{"reason":"填错"}`, map[string]string{"id": fmt.Sprint(slipID)}, true); rec.Code != http.StatusOK {
		t.Fatalf("作废失败: %d %s", rec.Code, rec.Body.String())
	}
	if whStock(t, pen) != 3 || whStock(t, camera) != 1 {
		t.Fatalf("作废后库存不对: 笔 %d 相机 %d", whStock(t, pen), whStock(t, camera))
	}
	if rec := whCall(t, VoidWarehouseSlip, "POST", "/", `{"reason":"再作废"}`, map[string]string{"id": fmt.Sprint(slipID)}, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("重复作废应被拦截，实际 %d", rec.Code)
	}

	// 入库与盘点
	if rec := whCall(t, StockInWarehouseItem, "POST", "/", `{"qty":5}`, map[string]string{"id": fmt.Sprint(pen)}, true); rec.Code != http.StatusOK || whStock(t, pen) != 8 {
		t.Fatalf("入库异常: %d 库存 %d", rec.Code, whStock(t, pen))
	}
	if rec := whCall(t, AdjustWarehouseItem, "POST", "/", `{"actual":6,"remark":"月底盘点"}`, map[string]string{"id": fmt.Sprint(pen)}, true); rec.Code != http.StatusOK || whStock(t, pen) != 6 {
		t.Fatalf("盘点异常: %d 库存 %d", rec.Code, whStock(t, pen))
	}

	// 有流水的物资不能删除
	if rec := whCall(t, DeleteWarehouseItem, "DELETE", "/", "", map[string]string{"id": fmt.Sprint(pen)}, true); rec.Code != http.StatusBadRequest {
		t.Fatalf("有流水的物资应禁止删除，实际 %d", rec.Code)
	}

	// 统计：作废的不计入，7 名其他人员各领 1 支
	today := time.Now().Format("2006-01-02")
	rec = whCall(t, WarehouseStats, "GET", "/api/warehouse/stats?start="+today+"&end="+today, "", nil, true)
	var st struct {
		ByItem []whStatRow `json:"by_item"`
	}
	json.Unmarshal(rec.Body.Bytes(), &st)
	found := false
	for _, r := range st.ByItem {
		if r.ItemName == "中性笔" {
			found = true
			if r.OutQty != 7 {
				t.Fatalf("笔的领用统计应为 7，实际 %d", r.OutQty)
			}
		}
	}
	if !found {
		t.Fatalf("统计缺少中性笔: %s", rec.Body.String())
	}

	// 列表与导出
	if rec := whCall(t, ListWarehouseRecords, "GET", "/api/warehouse/records?month="+time.Now().Format("2006-01"), "", nil, true); rec.Code != http.StatusOK {
		t.Fatalf("流水列表失败: %d %s", rec.Code, rec.Body.String())
	}
	if rec := whCall(t, ExportWarehouseRecords, "GET", "/api/export/warehouse/records", "", nil, true); rec.Code != http.StatusOK || rec.Body.Len() < 100 {
		t.Fatalf("导出流水失败: %d", rec.Code)
	}

	// 暂停后扫码提交被拒；重置令牌后旧码 404
	whCall(t, UpdateWarehouseSettings, "PUT", "/", `{"paused":true}`, nil, true)
	body = fmt.Sprintf(`{"user_id":2,"lines":[{"item_id":%d,"qty":1}]}`, pen)
	if rec := whCall(t, PublicWarehouseCheckout, "POST", "/", body, tok, false); rec.Code != http.StatusForbidden {
		t.Fatalf("暂停后应 403，实际 %d", rec.Code)
	}
	whCall(t, ResetWarehouseToken, "POST", "/", "", nil, true)
	if rec := whCall(t, PublicWarehouse, "GET", "/", "", tok, false); rec.Code != http.StatusNotFound {
		t.Fatalf("重置后旧令牌应 404，实际 %d", rec.Code)
	}
}
