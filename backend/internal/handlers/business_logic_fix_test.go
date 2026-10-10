package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"ynxwxcb-platform/internal/database"
	"ynxwxcb-platform/internal/models"
)

// 针对 7 项业务逻辑修复的闭环自动化测试
func TestBusinessLogicFixes(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// -------------------------------------------------------------
	// 测试项 1: 跨年请假算法失真修复（废除粗暴 MIN 逻辑，自然年比例精确分配）
	// -------------------------------------------------------------
	t.Run("跨年年假扣减比例精确分配", func(t *testing.T) {
		// 配置用户 2 在 2025 年 5 天年假，2026 年 5 天年假
		database.DB.Exec("INSERT INTO annual_leave_configs (user_id, year, days) VALUES (2, '2025', 5.0)")
		database.DB.Exec("INSERT INTO annual_leave_configs (user_id, year, days) VALUES (2, '2026', 5.0)")

		// 用户 2 申请跨年年假：2025-12-30 至 2026-01-03 (共5天跨年区间: 2025年 2天, 2026年 3天), 申请天数 2.0 天
		reqBody := models.LeaveRecord{
			UserID:    2,
			LeaveType: "annual",
			StartDate: "2025-12-30",
			EndDate:   "2026-01-03",
			Days:      2.0,
			Reason:    "跨年探亲休假",
		}
		data, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/leave", bytes.NewReader(data))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateLeaveRecord(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("跨年年假登记失败: %d %s", rec.Code, rec.Body.String())
		}

		// 检查 2025 年剩余天数与扣减天数: 总span=5天, 2025占2天(2/5=0.4), 扣 2.0 * 0.4 = 0.8 天
		remain2025, _ := getAnnualLeaveRemainDays(2, "2025", 0)
		if remain2025 < 4.19 || remain2025 > 4.21 {
			t.Fatalf("2025年剩余天数期望约为 4.2 天，实际为: %.2f (旧算法若MIN会扣2天导致只剩3天)", remain2025)
		}

		// 检查 2026 年剩余天数与扣减天数: 2026占3天(3/5=0.6), 扣 2.0 * 0.6 = 1.2 天
		remain2026, _ := getAnnualLeaveRemainDays(2, "2026", 0)
		if remain2026 < 3.79 || remain2026 > 3.81 {
			t.Fatalf("2026年剩余天数期望约为 3.8 天，实际为: %.2f", remain2026)
		}

		// 验证总扣减天数 strictly 等于申请的 2.0 天 (0.8 + 1.2 = 2.0)，彻底消除翻倍 Bug！
		totalDeducted := (5.0 - remain2025) + (5.0 - remain2026)
		if totalDeducted < 1.99 || totalDeducted > 2.01 {
			t.Fatalf("跨年请假总扣减天数应为 2.0 天，实际为: %.2f", totalDeducted)
		}
	})

	// -------------------------------------------------------------
	// 测试项 2: 调休补休申请额度超支校验（事务内聚合重算，不足立即回滚）
	// -------------------------------------------------------------
	t.Run("调休补休申请额度不足拦截", func(t *testing.T) {
		// 录入用户 2 加班 8 小时（折合 1.0 天补休）
		database.DB.Exec("INSERT INTO overtime_records (user_id, overtime_date, hours, reason) VALUES (2, '2026-05-01', 8.0, '假期值班')")

		// 申请 2.0 天补休（额度仅 1.0 天，应拦截）
		reqBody := models.LeaveRecord{
			UserID:    2,
			LeaveType: "comp",
			StartDate: "2026-05-10",
			EndDate:   "2026-05-11",
			Days:      2.0,
			Reason:    "调休两天",
		}
		data, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/leave", bytes.NewReader(data))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		CreateLeaveRecord(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("额度不足时期望返回 400 Bad Request，实际为: %d", rec.Code)
		}
	})

	// -------------------------------------------------------------
	// 测试项 3: 公车报备状态校验与时段冲突拦截
	// -------------------------------------------------------------
	t.Run("公车停用状态与时段冲突拦截", func(t *testing.T) {
		// 插入一辆可用车辆(id=10, status=1)和一辆维修中车辆(id=11, status=2)
		database.DB.Exec("INSERT INTO vehicles (id, plate_no, brand, seats, driver, status) VALUES (10, '新F88888', '大众帕萨特', 5, '张师傅', 1)")
		database.DB.Exec("INSERT INTO vehicles (id, plate_no, brand, seats, driver, status) VALUES (11, '新F99999', '别克GL8', 7, '王师傅', 2)")

		// 3.1 尝试报备维修中车辆 -> 应拦截
		applyFail := models.VehicleApply{
			VehicleID:   11,
			UserName:    "李干事",
			Purpose:     "下乡调研",
			Destination: "吉里于孜镇",
			UseDate:     "2026-06-01",
			UseTime:     "09:00-12:00",
		}
		d1, _ := json.Marshal(applyFail)
		req1 := httptest.NewRequest("POST", "/api/vehicle-applies", bytes.NewReader(d1))
		req1 = reqWithContext(req1, 2, "staff", "staff1", "李干事")
		rec1 := httptest.NewRecorder()
		CreateVehicleApply(rec1, req1)
		if rec1.Code != http.StatusBadRequest {
			t.Fatalf("对维修中车辆报备期望返回 400，实际为: %d", rec1.Code)
		}

		// 3.2 报备可用车辆 (id=10, 09:00-12:00) -> 应成功
		applyOK := models.VehicleApply{
			VehicleID:   10,
			UserName:    "李干事",
			Purpose:     "下乡调研",
			Destination: "吉里于孜镇",
			UseDate:     "2026-06-01",
			UseTime:     "09:00-12:00",
		}
		d2, _ := json.Marshal(applyOK)
		req2 := httptest.NewRequest("POST", "/api/vehicle-applies", bytes.NewReader(d2))
		req2 = reqWithContext(req2, 2, "staff", "staff1", "李干事")
		rec2 := httptest.NewRecorder()
		CreateVehicleApply(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("正常报备期望返回 200，实际为: %d %s", rec2.Code, rec2.Body.String())
		}

		// 3.3 另一人尝试报备同一车辆重叠时段 (id=10, 11:00-15:00) -> 应拦截
		applyConflict := models.VehicleApply{
			VehicleID:   10,
			UserName:    "王干事",
			Purpose:     "县城公务",
			Destination: "融媒体中心",
			UseDate:     "2026-06-01",
			UseTime:     "11:00-15:00",
		}
		d3, _ := json.Marshal(applyConflict)
		req3 := httptest.NewRequest("POST", "/api/vehicle-applies", bytes.NewReader(d3))
		req3 = reqWithContext(req3, 1, "admin", "admin", "系统管理员")
		rec3 := httptest.NewRecorder()
		CreateVehicleApply(rec3, req3)
		if rec3.Code != http.StatusBadRequest {
			t.Fatalf("重叠时段报备期望拦截返回 400，实际为: %d", rec3.Code)
		}

		// 3.4 报备同一车辆不重叠时段 (id=10, 14:00-18:00) -> 应成功
		applyNoConflict := models.VehicleApply{
			VehicleID:   10,
			UserName:    "赵干事",
			Purpose:     "县城公务",
			Destination: "县委大院",
			UseDate:     "2026-06-01",
			UseTime:     "14:00-18:00",
		}
		d4, _ := json.Marshal(applyNoConflict)
		req4 := httptest.NewRequest("POST", "/api/vehicle-applies", bytes.NewReader(d4))
		req4 = reqWithContext(req4, 1, "admin", "admin", "系统管理员")
		rec4 := httptest.NewRecorder()
		CreateVehicleApply(rec4, req4)
		if rec4.Code != http.StatusOK {
			t.Fatalf("错开时段报备期望返回 200，实际为: %d %s", rec4.Code, rec4.Body.String())
		}
	})

	// -------------------------------------------------------------
	// 测试项 4: 补录/撤销请假回溯更新考勤点到记录（防止误记旷工，撤销恢复正常）
	// -------------------------------------------------------------
	t.Run("补录请假纠正旷工与撤销请假恢复正常", func(t *testing.T) {
		date := "2026-07-01"
		// 模拟 7月1日 晨会点到，将用户 2 误记为旷工 (status=4)
		database.DB.Exec("INSERT INTO attendances (user_id, attend_date, status, remark, marked_by) VALUES (2, ?, 4, '晨会未到', 1)", date)

		// 干部补录请假（事假 1 天）
		leaveReq := models.LeaveRecord{
			UserID:    2,
			LeaveType: "personal",
			StartDate: date,
			EndDate:   date,
			Days:      1.0,
			Reason:    "家中有事补假",
		}
		d1, _ := json.Marshal(leaveReq)
		req1 := httptest.NewRequest("POST", "/api/leave", bytes.NewReader(d1))
		req1 = reqWithContext(req1, 1, "admin", "admin", "系统管理员")
		rec1 := httptest.NewRecorder()
		CreateLeaveRecord(rec1, req1)
		if rec1.Code != http.StatusOK {
			t.Fatalf("补录请假失败: %d", rec1.Code)
		}

		// 检查 attendances 表中的记录是否被自动回溯更新为请假 status=2
		var currentStatus int
		var leaveType string
		database.DB.QueryRow("SELECT status, leave_type FROM attendances WHERE user_id=2 AND attend_date=?", date).Scan(&currentStatus, &leaveType)
		if currentStatus != 2 || leaveType != "personal" {
			t.Fatalf("补录请假后考勤状态期望纠正为 status=2 (请假), leave_type='personal'，实际为 status=%d, leave_type='%s'", currentStatus, leaveType)
		}

		// 查询刚才插入的请假记录 ID 并删除撤销该请假
		var leaveID int64
		database.DB.QueryRow("SELECT id FROM leave_records WHERE user_id=2 AND start_date=? ORDER BY id DESC LIMIT 1", date).Scan(&leaveID)

		reqDel := httptest.NewRequest("DELETE", fmt.Sprintf("/api/leave/%d", leaveID), nil)
		reqDel.SetPathValue("id", strconv.FormatInt(leaveID, 10))
		reqDel = reqWithContext(reqDel, 1, "admin", "admin", "系统管理员")
		recDel := httptest.NewRecorder()
		DeleteLeaveRecord(recDel, reqDel)
		if recDel.Code != http.StatusOK {
			t.Fatalf("删除请假记录失败: %d", recDel.Code)
		}

		// 检查 attendances 表是否已自动回溯恢复为正常出勤 status=1
		database.DB.QueryRow("SELECT status, leave_type FROM attendances WHERE user_id=2 AND attend_date=?", date).Scan(&currentStatus, &leaveType)
		if currentStatus != 1 {
			t.Fatalf("撤销请假后考勤记录期望恢复为 status=1 (正常出勤)，实际为 status=%d", currentStatus)
		}
	})

	// -------------------------------------------------------------
	// 测试项 5: 公文材料下发截止时限漏验修复
	// -------------------------------------------------------------
	t.Run("材料下发已过截止时限拦截查收", func(t *testing.T) {
		// 插入已过期的下发任务（截止时间为过去的时间）
		pastDeadline := time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04")
		res, err := database.DB.Exec(
			`INSERT INTO dispatches (title, doc_no, units, deadline, pdf_path, pdf_name, created_by)
			 VALUES ('全县宣传思想工作通知', '伊宣发〔2026〕1号', '办公室\n新闻宣传科\n吉里于孜镇', ?, '/uploads/notice.pdf', 'notice.pdf', 1)`, pastDeadline)
		if err != nil {
			t.Fatalf("插入下发任务失败: %v", err)
		}
		dispatchID, _ := res.LastInsertId()

		// 尝试公开确认查收
		receiptBody := map[string]string{
			"unit":           "吉里于孜镇",
			"receiver_name":  "张宣传",
			"receiver_phone": "13899990000",
		}
		data, _ := json.Marshal(receiptBody)
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/public/dispatches/%d/receipt", dispatchID), bytes.NewReader(data))
		req.SetPathValue("id", strconv.FormatInt(dispatchID, 10))
		rec := httptest.NewRecorder()
		PublicConfirmDispatchReceipt(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("超过截止时限查收期望拦截返回 400，实际返回: %d", rec.Code)
		}
	})

	// -------------------------------------------------------------
	// 测试项 6: 删除历史加班工时导致调休负数系统坏账修复
	// -------------------------------------------------------------
	t.Run("删除加班记录导致调休负数拦截", func(t *testing.T) {
		// 录入用户 2 加班 16 小时（折合 2 天）
		res, _ := database.DB.Exec("INSERT INTO overtime_records (user_id, overtime_date, hours, reason) VALUES (2, '2026-08-01', 16.0, '重大专项保障')")
		otID, _ := res.LastInsertId()

		// 用户 2 申请调休 2.0 天（调休余额消耗完毕，剩余 0 天）
		database.DB.Exec("INSERT INTO leave_records (user_id, leave_type, start_date, end_date, days, status) VALUES (2, 'comp', '2026-08-10', '2026-08-11', 2.0, 1)")

		// 管理员尝试删除该加班记录 -> 删除后调休余额将变成 -2 天，必须禁止删除并报错！
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/overtime/%d", otID), nil)
		req.SetPathValue("id", strconv.FormatInt(otID, 10))
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		DeleteOvertimeRecord(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("删除会导致负数余额的加班记录期望拦截返回 400，实际为: %d", rec.Code)
		}
	})

	// -------------------------------------------------------------
	// 测试项 7: 会务列表 confirmed_units 统计与从表空格清洗匹配
	// -------------------------------------------------------------
	t.Run("会务列表已报名单位数统计与空格清洗", func(t *testing.T) {
		// 创建一个会议，要求 3 个单位参加：单位 A、单位 B、单位 C
		res, _ := database.DB.Exec(
			`INSERT INTO meetings (title, meeting_date, units, unit_limit, created_by)
			 VALUES ('全县宣传思想推进会', '2026-09-01', '吉里于孜镇\n榆树沟镇\n阿热吾斯塘镇', 3, 1)`)
		meetingID, _ := res.LastInsertId()

		// 模拟单位 A（吉里于孜镇）报了 2 人（注意：录入时可能带空格或换行回车）
		database.DB.Exec("INSERT INTO meeting_registrations (meeting_id, unit, attendee_name, attendee_title, phone, not_attend) VALUES (?, '吉里于孜镇 ', '张三', '宣传委员', '13800000001', 0)", meetingID)
		database.DB.Exec("INSERT INTO meeting_registrations (meeting_id, unit, attendee_name, attendee_title, phone, not_attend) VALUES (?, '吉里于孜镇', '李四', '通讯员', '13800000002', 0)", meetingID)

		// 模拟单位 B（榆树沟镇）登记了不参加
		database.DB.Exec("INSERT INTO meeting_registrations (meeting_id, unit, attendee_name, attendee_title, phone, not_attend, reason) VALUES (?, '榆树沟镇\r', '', '', '', 1, '因公出差')", meetingID)

		// 查询会务列表，验证返回的 confirmed_units 是否精准等于 2（吉里于孜镇 + 榆树沟镇已报名反馈，阿热吾斯塘镇未报名）
		req := httptest.NewRequest("GET", "/api/meetings?page=1&page_size=10", nil)
		req = reqWithContext(req, 1, "admin", "admin", "系统管理员")
		rec := httptest.NewRecorder()
		ListMeetings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("ListMeetings 失败: %d", rec.Code)
		}
		var listResp struct {
			List []models.Meeting `json:"list"`
		}
		json.Unmarshal(rec.Body.Bytes(), &listResp)
		var targetMeeting *models.Meeting
		for _, m := range listResp.List {
			if m.ID == meetingID {
				targetMeeting = &m
				break
			}
		}
		if targetMeeting == nil {
			t.Fatalf("未在列表中找到会议 ID=%d", meetingID)
		}
		if targetMeeting.ConfirmedUnits != 2 {
			t.Fatalf("期望 confirmed_units = 2（吉里于孜镇和榆树沟镇），实际为: %d", targetMeeting.ConfirmedUnits)
		}

		// 验证 Excel 签到册导出不会因从表空格丢失人名
		reqExport := httptest.NewRequest("GET", fmt.Sprintf("/api/meetings/%d/export-registration", meetingID), nil)
		reqExport.SetPathValue("id", strconv.FormatInt(meetingID, 10))
		reqExport = reqWithContext(reqExport, 1, "admin", "admin", "系统管理员")
		recExport := httptest.NewRecorder()
		ExportMeetingRegistration(recExport, reqExport)
		if recExport.Code != http.StatusOK {
			t.Fatalf("导出签到册失败: %d", recExport.Code)
		}
		if recExport.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Fatalf("签到册导出 Content-Type 异常: %s", recExport.Header().Get("Content-Type"))
		}
	})
}
