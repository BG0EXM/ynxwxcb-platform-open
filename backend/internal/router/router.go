package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ynxwxcb-platform/internal/config"
	"ynxwxcb-platform/internal/handlers"
	"ynxwxcb-platform/internal/middleware"
)

func NewRouter(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	// 仅需登录（无需具体权限点）
	authOnly := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(h)
	}
	// 仅系统管理员（AI 接口密钥、额度等不可下放）
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(middleware.RequireRole("admin")(h))
	}
	// 需要某权限点
	perm := func(code string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(middleware.RequirePerm(code)(h))
	}

	// ---- 公开路由 ----
	mux.HandleFunc("GET /api/health", handlers.Health)
	// 登录按 IP 限流（防爆破）
	mux.Handle("POST /api/auth/login", middleware.RateLimit(20, time.Minute)(handlers.Login(cfg)))
	// 会务公开报名（匿名，无需登录）——按 IP 限流（防刷）
	mux.Handle("GET /api/public/meetings/{token}", middleware.RateLimit(120, time.Minute)(http.HandlerFunc(handlers.PublicMeeting)))
	mux.Handle("POST /api/public/meetings/{token}/register", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicRegisterMeeting)))
	mux.Handle("POST /api/public/meetings/{token}/remove", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicRemoveAttendee)))
	mux.Handle("POST /api/public/meetings/{token}/cancel", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicCancelMeetingRegister)))
	// 征求意见公开填报与文件下载（匿名，无需登录）——按 IP 限流（防刷）
	mux.Handle("GET /api/public/solicits/{token}", middleware.RateLimit(120, time.Minute)(http.HandlerFunc(handlers.PublicSolicit)))
	mux.Handle("GET /api/public/solicits/{token}/file", middleware.RateLimit(120, time.Minute)(handlers.PublicServeSolicitFile(cfg)))
	mux.Handle("POST /api/public/solicits/{token}/upload", middleware.RateLimit(30, time.Minute)(handlers.PublicSolicitUpload(cfg)))
	mux.Handle("POST /api/public/solicits/{token}/feedback", middleware.RateLimit(30, time.Minute)(handlers.PublicSubmitFeedback(cfg)))

	// 材料下发公开查收与文件下载（匿名，无需登录）——按 IP 限流（防刷）
	mux.Handle("GET /api/public/dispatches/{token}", middleware.RateLimit(120, time.Minute)(http.HandlerFunc(handlers.PublicDispatch)))
	mux.Handle("GET /api/public/dispatches/{token}/file", middleware.RateLimit(120, time.Minute)(handlers.PublicServeDispatchFile(cfg)))
	mux.Handle("POST /api/public/dispatches/{token}/receipt", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicConfirmDispatchReceipt)))
	// 库房物资门口二维码扫码领用/归还（匿名，无需登录）——按 IP 限流（防刷）
	mux.Handle("GET /api/public/warehouse/{token}", middleware.RateLimit(120, time.Minute)(http.HandlerFunc(handlers.PublicWarehouse)))
	mux.Handle("POST /api/public/warehouse/{token}/checkout", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicWarehouseCheckout)))
	mux.Handle("POST /api/public/warehouse/{token}/borrows", middleware.RateLimit(60, time.Minute)(http.HandlerFunc(handlers.PublicWarehouseBorrows)))
	mux.Handle("POST /api/public/warehouse/{token}/return", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicWarehouseReturn)))


	// ---- 认证 ----

	mux.Handle("GET /api/auth/profile", authOnly(handlers.GetProfile))
	mux.Handle("POST /api/auth/change-password", authOnly(handlers.ChangePassword))
	mux.Handle("POST /api/auth/logout", authOnly(handlers.Logout))

	// ---- 用户与系统管理 ----
	mux.Handle("GET /api/users", perm("user.manage", handlers.ListUsers))
	mux.Handle("POST /api/users", perm("user.manage", handlers.CreateUser))
	mux.Handle("PUT /api/users", perm("user.manage", handlers.UpdateUser))
	mux.Handle("DELETE /api/users/{id}", perm("user.manage", handlers.DeleteUser))
	mux.Handle("POST /api/users/reset-password", perm("user.manage", handlers.ResetPassword))
	mux.Handle("GET /api/users/{id}/attendance-profile", perm("attendance.view", handlers.GetUserAttendanceProfile))
	mux.Handle("GET /api/operation-logs", perm("oplog.view", handlers.ListOperationLogs))
	mux.Handle("POST /api/operation-logs/clear", perm("user.manage", handlers.ClearOperationLogs))
	mux.Handle("GET /api/permissions", perm("user.manage", handlers.GetPermissionMatrix))
	mux.Handle("PUT /api/permissions", perm("user.manage", handlers.SavePermissionMatrix))
	// 下拉数据：登录即可
	mux.Handle("GET /api/roles", authOnly(handlers.ListRoles))
	mux.Handle("GET /api/departments", authOnly(handlers.ListDepartments))
	mux.Handle("GET /api/assignees", authOnly(handlers.GetAssignees))
	// 全局快捷速查与指令
	mux.Handle("GET /api/global-search", authOnly(handlers.GlobalSearch))
	mux.Handle("POST /api/departments", perm("department.manage", handlers.CreateDepartment))
	mux.Handle("PUT /api/departments", perm("department.manage", handlers.UpdateDepartment))
	mux.Handle("DELETE /api/departments/{id}", perm("department.manage", handlers.DeleteDepartment))

	// ---- 公共资料 ----
	mux.Handle("GET /api/study-materials", perm("study.view", handlers.ListStudyMaterials))
	mux.Handle("POST /api/study-materials", perm("study.publish", handlers.CreateStudyMaterial))
	mux.Handle("GET /api/study-materials/{id}", perm("study.view", handlers.GetStudyMaterial))
	mux.Handle("DELETE /api/study-materials/{id}", perm("study.delete", handlers.DeleteStudyMaterial(cfg)))
	mux.Handle("GET /api/study-categories", perm("study.view", handlers.ListStudyCategories))
	mux.Handle("POST /api/study-categories", perm("study.category", handlers.CreateStudyCategory))
	mux.Handle("PUT /api/study-categories", perm("study.category", handlers.UpdateStudyCategory))
	mux.Handle("DELETE /api/study-categories/{id}", perm("study.category", handlers.DeleteStudyCategory))

	// ---- 通讯录 ----
	mux.Handle("GET /api/contacts", perm("contact.view", handlers.ListContacts))
	mux.Handle("POST /api/contacts", perm("contact.manage", handlers.CreateContact))
	mux.Handle("PUT /api/contacts", perm("contact.manage", handlers.UpdateContact))
	mux.Handle("DELETE /api/contacts/{id}", perm("contact.manage", handlers.DeleteContact))

	// ---- 排班 ----
	mux.Handle("GET /api/duty-schedules", perm("duty.view", handlers.ListDutySchedules))
	mux.Handle("POST /api/duty-schedules", perm("duty.manage", handlers.SaveDutySchedule))
	mux.Handle("DELETE /api/duty-schedules/{id}", perm("duty.manage", handlers.DeleteDutySchedule))

	// ---- 工作日历 ----
	mux.Handle("GET /api/calendar-tasks", perm("calendar.view", handlers.ListCalendarTasks))
	mux.Handle("POST /api/calendar-tasks", perm("calendar.manage", handlers.CreateCalendarTask))
	mux.Handle("PUT /api/calendar-tasks", perm("calendar.manage", handlers.UpdateCalendarTask))
	mux.Handle("DELETE /api/calendar-tasks/{id}", perm("calendar.manage", handlers.DeleteCalendarTask))

	// ---- 常委管理 ----
	mux.Handle("GET /api/standing-events", perm("standing.manage", handlers.ListStandingCommitteeEvents))
	mux.Handle("POST /api/standing-events", perm("standing.manage", handlers.CreateStandingCommitteeEvent))
	mux.Handle("PUT /api/standing-events", perm("standing.manage", handlers.UpdateStandingCommitteeEvent))
	mux.Handle("DELETE /api/standing-events/{id}", perm("standing.manage", handlers.DeleteStandingCommitteeEvent))

	// ---- 大事记 ----
	mux.Handle("GET /api/major-events", perm("event.view", handlers.ListMajorEvents))
	mux.Handle("POST /api/major-events", perm("event.manage", handlers.CreateMajorEvent))
	mux.Handle("PUT /api/major-events", perm("event.manage", handlers.UpdateMajorEvent))
	mux.Handle("DELETE /api/major-events/{id}", perm("event.manage", handlers.DeleteMajorEvent))

	// ---- 每周工作总结 ----
	mux.Handle("GET /api/weekly-summaries", perm("weekly.view", handlers.ListWeeklySummaries))
	mux.Handle("POST /api/weekly-summaries", perm("weekly.manage", handlers.CreateWeeklySummary))
	mux.Handle("PUT /api/weekly-summaries", perm("weekly.manage", handlers.UpdateWeeklySummary))
	mux.Handle("DELETE /api/weekly-summaries/{id}", perm("weekly.manage", handlers.DeleteWeeklySummary))

	// ---- 加班统计与补休 ----
	mux.Handle("GET /api/overtime-records", perm("overtime.view", handlers.ListOvertimeRecords))
	mux.Handle("POST /api/overtime-records", perm("overtime.manage", handlers.CreateOvertimeRecord))
	mux.Handle("DELETE /api/overtime-records/{id}", perm("overtime.manage", handlers.DeleteOvertimeRecord))
	mux.Handle("GET /api/overtime-stats", perm("overtime.view", handlers.OvertimeStats))

	// ---- 年休假管理 ----
	mux.Handle("GET /api/annual-leave-configs", perm("annualleave.view", handlers.ListAnnualLeaveConfigs))
	mux.Handle("POST /api/annual-leave-configs", perm("annualleave.manage", handlers.SaveAnnualLeaveConfig))

	// ---- 会务管理 ----
	mux.Handle("GET /api/meetings", perm("meeting.manage", handlers.ListMeetings))
	mux.Handle("POST /api/meetings", perm("meeting.manage", handlers.CreateMeeting))
	mux.Handle("PUT /api/meetings", perm("meeting.manage", handlers.UpdateMeeting))
	mux.Handle("DELETE /api/meetings/{id}", perm("meeting.manage", handlers.DeleteMeeting))
	mux.Handle("GET /api/meetings/{id}", perm("meeting.manage", handlers.GetMeeting))
	mux.Handle("GET /api/meetings/{id}/registrations", perm("meeting.manage", handlers.MeetingRegistrations))
	mux.Handle("POST /api/meetings/{id}/registrations/delete", perm("meeting.manage", handlers.AdminDeleteMeetingRegistration))
	mux.Handle("POST /api/meetings/{id}/registrations/change-absent", perm("meeting.manage", handlers.AdminChangeUnitToAbsent))
	mux.Handle("POST /api/meetings/{id}/registrations/reset-unit", perm("meeting.manage", handlers.AdminResetUnitRegistration))
	mux.Handle("GET /api/export/meetings/{id}/registration", perm("meeting.manage", handlers.ExportMeetingRegistration))
	mux.Handle("GET /api/export/meeting-registrations", perm("meeting.manage", handlers.ExportMeetingRegistration))

	// ---- 征求意见管理 ----
	mux.Handle("GET /api/solicits", perm("solicit.manage", handlers.ListSolicits))
	mux.Handle("POST /api/solicits", perm("solicit.manage", handlers.CreateSolicit(cfg)))
	mux.Handle("PUT /api/solicits", perm("solicit.manage", handlers.UpdateSolicit(cfg)))
	mux.Handle("DELETE /api/solicits/{id}", perm("solicit.manage", handlers.DeleteSolicit(cfg)))
	mux.Handle("GET /api/solicits/{id}", perm("solicit.manage", handlers.GetSolicit))
	mux.Handle("POST /api/solicits/{id}/feedbacks/reset-unit", perm("solicit.manage", handlers.ResetUnitFeedback(cfg)))
	mux.Handle("GET /api/export/solicits/{id}/feedbacks", perm("solicit.manage", handlers.ExportSolicitFeedbacks))
	mux.Handle("GET /api/solicits/{id}/download-replies", perm("solicit.manage", handlers.DownloadSolicitRepliesZip(cfg)))

	// ---- 材料下发管理 ----
	mux.Handle("GET /api/dispatches", perm("dispatch.manage", handlers.ListDispatches))
	mux.Handle("POST /api/dispatches", perm("dispatch.manage", handlers.CreateDispatch(cfg)))
	mux.Handle("PUT /api/dispatches", perm("dispatch.manage", handlers.UpdateDispatch(cfg)))
	mux.Handle("DELETE /api/dispatches/{id}", perm("dispatch.manage", handlers.DeleteDispatch(cfg)))
	mux.Handle("GET /api/dispatches/{id}", perm("dispatch.manage", handlers.GetDispatch))
	mux.Handle("POST /api/dispatches/{id}/reset-unit", perm("dispatch.manage", handlers.ResetUnitDispatchReceipt))
	mux.Handle("GET /api/export/dispatches/{id}/receipts", perm("dispatch.manage", handlers.ExportDispatchReceipts))

	// ---- 库房物资 ----
	mux.Handle("GET /api/warehouse/items", perm("warehouse.manage", handlers.ListWarehouseItems))
	mux.Handle("POST /api/warehouse/items", perm("warehouse.manage", handlers.CreateWarehouseItem))
	mux.Handle("PUT /api/warehouse/items", perm("warehouse.manage", handlers.UpdateWarehouseItem))
	mux.Handle("DELETE /api/warehouse/items/{id}", perm("warehouse.manage", handlers.DeleteWarehouseItem))
	mux.Handle("POST /api/warehouse/items/{id}/stock-in", perm("warehouse.manage", handlers.StockInWarehouseItem))
	mux.Handle("POST /api/warehouse/items/{id}/adjust", perm("warehouse.manage", handlers.AdjustWarehouseItem))
	mux.Handle("GET /api/warehouse/records", perm("warehouse.manage", handlers.ListWarehouseRecords))
	mux.Handle("POST /api/warehouse/slips/{id}/void", perm("warehouse.manage", handlers.VoidWarehouseSlip))
	mux.Handle("POST /api/warehouse/checkout", perm("warehouse.manage", handlers.AdminWarehouseCheckout))
	mux.Handle("GET /api/warehouse/people", perm("warehouse.manage", handlers.ListWarehousePeople))
	mux.Handle("GET /api/warehouse/borrows", perm("warehouse.manage", handlers.ListWarehouseBorrows))
	mux.Handle("POST /api/warehouse/borrows/{id}/return", perm("warehouse.manage", handlers.AdminReturnWarehouseBorrow))
	mux.Handle("GET /api/warehouse/stats", perm("warehouse.manage", handlers.WarehouseStats))
	mux.Handle("GET /api/warehouse/settings", perm("warehouse.manage", handlers.GetWarehouseSettings))
	mux.Handle("PUT /api/warehouse/settings", perm("warehouse.manage", handlers.UpdateWarehouseSettings))
	mux.Handle("POST /api/warehouse/settings/reset-token", perm("warehouse.manage", handlers.ResetWarehouseToken))
	mux.Handle("GET /api/export/warehouse/records", perm("warehouse.manage", handlers.ExportWarehouseRecords))
	mux.Handle("GET /api/export/warehouse/items", perm("warehouse.manage", handlers.ExportWarehouseItems))
	mux.Handle("GET /api/export/warehouse/stats", perm("warehouse.manage", handlers.ExportWarehouseStats))


	// ---- AI 写作助手 ----
	mux.Handle("GET /api/ai/status", perm("ai.use", handlers.AIStatus))
	mux.Handle("GET /api/ai/prompts", perm("ai.use", handlers.AIListPrompts))
	mux.Handle("GET /api/ai/usage", perm("ai.use", handlers.AIMyUsage))
	mux.Handle("POST /api/ai/chat", middleware.RateLimit(60, time.Minute)(perm("ai.use", handlers.AIChat)))
	mux.Handle("GET /api/ai/admin/settings", adminOnly(handlers.AIAdminGetSettings))
	mux.Handle("PUT /api/ai/admin/settings", adminOnly(handlers.AIAdminSaveSettings))
	mux.Handle("POST /api/ai/admin/test", adminOnly(handlers.AIAdminTest))
	mux.Handle("GET /api/ai/admin/accounts", adminOnly(handlers.AIAdminAccounts))
	mux.Handle("POST /api/ai/admin/topup", adminOnly(handlers.AIAdminTopup))
	mux.Handle("GET /api/ai/admin/ledger", adminOnly(handlers.AIAdminLedger))
	mux.Handle("GET /api/ai/admin/usage", adminOnly(handlers.AIAdminUsage))
	mux.Handle("GET /api/ai/admin/prompts", adminOnly(handlers.AIAdminListPrompts))
	mux.Handle("POST /api/ai/admin/prompts", adminOnly(handlers.AIAdminCreatePrompt))
	mux.Handle("PUT /api/ai/admin/prompts", adminOnly(handlers.AIAdminUpdatePrompt))
	mux.Handle("DELETE /api/ai/admin/prompts/{id}", adminOnly(handlers.AIAdminDeletePrompt))

	// ---- 考勤 ----
	mux.Handle("POST /api/attendance/mark", perm("attendance.mark", handlers.MarkAttendance))
	mux.Handle("GET /api/attendance/list", perm("attendance.view", handlers.ListAttendances))
	mux.Handle("GET /api/attendance/stats", perm("attendance.stats", handlers.AttendanceStats))
	mux.Handle("GET /api/attendance/dates", perm("attendance.view", handlers.AttendanceDates))
	mux.Handle("GET /api/attendance/mark-users", perm("attendance.mark", handlers.MarkUsers))
	mux.Handle("GET /api/attendance/monthly", perm("attendance.stats", handlers.AttendanceMonthly))
	mux.Handle("GET /api/attendance/yearly", perm("attendance.stats", handlers.AttendanceYearly))

	// ---- 请假管理 ----
	mux.Handle("GET /api/leave-records/{id}", perm("leave.view", handlers.GetLeaveRecord))
	mux.Handle("POST /api/leave-records", perm("leave.manage", handlers.CreateLeaveRecord))
	mux.Handle("PUT /api/leave-records", perm("leave.manage", handlers.UpdateLeaveRecord))
	mux.Handle("GET /api/leave-records", perm("leave.view", handlers.ListLeaveRecords))
	mux.Handle("DELETE /api/leave-records/{id}", perm("leave.delete", handlers.DeleteLeaveRecord))
	mux.Handle("GET /api/leave-stats", perm("leave.view", handlers.LeaveStats))
	// 兼容别名（适配前端 /leaves 请求路径）
	mux.Handle("GET /api/leaves/{id}", perm("leave.view", handlers.GetLeaveRecord))
	mux.Handle("POST /api/leaves", perm("leave.manage", handlers.CreateLeaveRecord))
	mux.Handle("PUT /api/leaves", perm("leave.manage", handlers.UpdateLeaveRecord))
	mux.Handle("GET /api/leaves", perm("leave.view", handlers.ListLeaveRecords))
	mux.Handle("DELETE /api/leaves/{id}", perm("leave.delete", handlers.DeleteLeaveRecord))
	mux.Handle("GET /api/leaves/stats", perm("leave.view", handlers.LeaveStats))
	mux.Handle("GET /api/export/leaves", perm("leave.export", handlers.ExportLeaveRecords))

	// ---- 公车管理 ----
	mux.Handle("GET /api/vehicles", perm("vehicle.view", handlers.ListVehicles))
	mux.Handle("POST /api/vehicles", perm("vehicle.manage", handlers.CreateVehicle))
	mux.Handle("PUT /api/vehicles", perm("vehicle.manage", handlers.UpdateVehicle))
	mux.Handle("DELETE /api/vehicles/{id}", perm("vehicle.manage", handlers.DeleteVehicle))
	mux.Handle("POST /api/vehicle-applies", perm("vehicle.apply", handlers.CreateVehicleApply))
	mux.Handle("PUT /api/vehicle-applies", perm("vehicle.apply", handlers.UpdateVehicleApply))
	mux.Handle("GET /api/vehicle-applies", perm("vehicle.view", handlers.ListVehicleApplies))
	mux.Handle("GET /api/vehicle-applies/{id}", perm("vehicle.view", handlers.GetVehicleApply))
	mux.Handle("DELETE /api/vehicle-applies/{id}", perm("vehicle.apply", handlers.DeleteVehicleApply))
	mux.Handle("GET /api/vehicle-stats", perm("vehicle.view", handlers.VehicleStats))

	// ---- 文件上传（登录即可，供各模块附件）----
	mux.Handle("POST /api/uploads", authOnly(handlers.UploadFile(cfg)))
	mux.Handle("GET /api/uploads/{id}", authOnly(handlers.DownloadAttachment(cfg)))
	mux.Handle("PUT /api/uploads/link", authOnly(handlers.LinkAttachment))

	// ---- 收文登记 ----
	mux.Handle("GET /api/incoming-docs", perm("incoming.view", handlers.ListIncomingDocs))
	mux.Handle("POST /api/incoming-docs", perm("incoming.manage", handlers.CreateIncomingDoc))
	mux.Handle("POST /api/incoming-docs/{id}/status", perm("incoming.manage", handlers.UpdateIncomingDocStatus))
	mux.Handle("GET /api/incoming-docs/{id}", perm("incoming.view", handlers.GetIncomingDoc))
	mux.Handle("PUT /api/incoming-docs", perm("incoming.manage", handlers.UpdateIncomingDoc))
	mux.Handle("DELETE /api/incoming-docs/{id}", perm("incoming.manage", handlers.DeleteIncomingDoc(cfg)))
	mux.Handle("GET /api/incoming-doc-stats", perm("incoming.view", handlers.IncomingDocStats))
	// 传阅记录
	mux.Handle("POST /api/circulations", perm("incoming.circulation", handlers.AddCirculation))
	mux.Handle("PUT /api/circulations", perm("incoming.circulation", handlers.UpdateCirculation))
	mux.Handle("DELETE /api/circulations/{id}", perm("incoming.circulation", handlers.DeleteCirculation))
	// 兼容别名（适配前端 /incoming-circulations 请求路径）
	mux.Handle("POST /api/incoming-circulations", perm("incoming.circulation", handlers.AddCirculation))
	mux.Handle("PUT /api/incoming-circulations", perm("incoming.circulation", handlers.UpdateCirculation))
	mux.Handle("DELETE /api/incoming-circulations/{id}", perm("incoming.circulation", handlers.DeleteCirculation))

	// ---- 首页统计 ----
	mux.Handle("GET /api/dashboard-stats", perm("dashboard.view", handlers.DashboardStats))
	// 今日待办：登录即可，各类待办在接口内按权限点逐类放行（顶栏铃铛在任何页面都要用）
	mux.Handle("GET /api/dashboard/todos", authOnly(handlers.DashboardTodos))

	// ---- 数据导出（Excel）----
	mux.Handle("GET /api/export/vehicle-applies", perm("vehicle.export", handlers.ExportVehicleApplies))
	mux.Handle("GET /api/export/leave-records", perm("leave.export", handlers.ExportLeaveRecords))
	mux.Handle("GET /api/export/attendances", perm("attendance.export", handlers.ExportAttendances))
	mux.Handle("GET /api/export/duty-schedules", perm("duty.export", handlers.ExportDutySchedules))
	mux.Handle("GET /api/export/incoming-docs", perm("incoming.export", handlers.ExportIncomingDocs))
	mux.Handle("GET /api/export/incoming", perm("incoming.export", handlers.ExportIncomingDocs))
	mux.Handle("GET /api/export/calendar-tasks", perm("calendar.export", handlers.ExportCalendarTasks))
	mux.Handle("GET /api/export/overtime-records", perm("overtime.export", handlers.ExportOvertimeRecords))
	mux.Handle("GET /api/export/annual-leave-configs", perm("annualleave.export", handlers.ExportAnnualLeaveConfigs))

	// ---- 数据导出（Word）----
	mux.Handle("GET /api/export/standing-events", perm("standing.manage", handlers.ExportStandingCommitteeEvents))
	mux.Handle("GET /api/export/major-events", perm("event.export", handlers.ExportMajorEvents))
	mux.Handle("GET /api/export/weekly-summaries", perm("weekly.export", handlers.ExportWeeklySummaries))

	// 上传文件受控直读服务（仅已登录认证用户可访问，收敛未鉴权下载漏洞）
	mux.Handle("GET /uploads/", authOnly(handlers.ServeUploadFile(cfg)))

	// 静态文件服务（前端构建产物）+ SPA 回退
	staticDir := "static"
	mux.Handle("GET /", spaHandler{staticDir: staticDir})

	return mux
}

// spaHandler 支持 history 路由的静态文件服务
type spaHandler struct {
	staticDir string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 未注册的接口地址一律返回 JSON 404，不能回退成页面：
	// 否则升级时新前端先于新后端上线，浏览器会把页面 HTML 当作接口结果缓存下来，长时间显示空数据
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
		middleware.JSON(w, http.StatusNotFound, map[string]string{"error": "接口不存在，请刷新页面后重试"})
		return
	}
	// 路径穿越防护
	path := filepath.Clean(r.URL.Path)
	rel := strings.TrimPrefix(path, string(os.PathSeparator))
	fullPath := filepath.Join(h.staticDir, rel)
	absStatic, _ := filepath.Abs(h.staticDir)
	absFull, _ := filepath.Abs(fullPath)
	if absFull != absStatic && !strings.HasPrefix(absFull, absStatic+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	// 如果请求的文件存在则直接提供，否则回退到 index.html
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, fullPath)
		return
	}
	// 入口页不缓存，升级后用户刷新即可拿到新版前端
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(h.staticDir, "index.html"))
}
