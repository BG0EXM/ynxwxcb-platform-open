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
	// 需要某权限点
	perm := func(code string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(middleware.RequirePerm(code)(h))
	}

	// ---- 公开路由 ----
	mux.HandleFunc("GET /api/health", handlers.Health)
	// 登录按 IP 限流（防爆破）
	mux.Handle("POST /api/auth/login", middleware.RateLimit(20, time.Minute)(handlers.Login(cfg)))
	// 会务公开报名（匿名，无需登录）——按 IP 限流（防刷）
	mux.Handle("GET /api/public/meetings/{id}", middleware.RateLimit(120, time.Minute)(http.HandlerFunc(handlers.PublicMeeting)))
	mux.Handle("POST /api/public/meetings/{id}/register", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicRegisterMeeting)))
	mux.Handle("POST /api/public/meetings/{id}/remove", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicRemoveAttendee)))
	mux.Handle("POST /api/public/meetings/{id}/cancel", middleware.RateLimit(30, time.Minute)(http.HandlerFunc(handlers.PublicCancelMeetingRegister)))

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
	mux.Handle("GET /api/operation-logs", perm("oplog.view", handlers.ListOperationLogs))
	mux.Handle("GET /api/permissions", perm("user.manage", handlers.GetPermissionMatrix))
	mux.Handle("PUT /api/permissions", perm("user.manage", handlers.SavePermissionMatrix))
	// 下拉数据：登录即可
	mux.Handle("GET /api/roles", authOnly(handlers.ListRoles))
	mux.Handle("GET /api/departments", authOnly(handlers.ListDepartments))
	mux.Handle("GET /api/assignees", authOnly(handlers.GetAssignees))
	mux.Handle("POST /api/departments", perm("department.manage", handlers.CreateDepartment))
	mux.Handle("PUT /api/departments", perm("department.manage", handlers.UpdateDepartment))
	mux.Handle("DELETE /api/departments/{id}", perm("department.manage", handlers.DeleteDepartment))

	// ---- 公共资料 ----
	mux.Handle("GET /api/study-materials", perm("study.view", handlers.ListStudyMaterials))
	mux.Handle("POST /api/study-materials", perm("study.publish", handlers.CreateStudyMaterial))
	mux.Handle("GET /api/study-materials/{id}", perm("study.view", handlers.GetStudyMaterial))
	mux.Handle("DELETE /api/study-materials/{id}", perm("study.delete", handlers.DeleteStudyMaterial))
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
	mux.Handle("GET /api/export/meetings/{id}/registration", perm("meeting.manage", handlers.ExportMeetingRegistration))

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
	mux.Handle("GET /api/incoming-docs/{id}", perm("incoming.view", handlers.GetIncomingDoc))
	mux.Handle("PUT /api/incoming-docs", perm("incoming.manage", handlers.UpdateIncomingDoc))
	mux.Handle("DELETE /api/incoming-docs/{id}", perm("incoming.manage", handlers.DeleteIncomingDoc))
	mux.Handle("GET /api/incoming-doc-stats", perm("incoming.view", handlers.IncomingDocStats))
	// 传阅记录
	mux.Handle("POST /api/circulations", perm("incoming.circulation", handlers.AddCirculation))
	mux.Handle("PUT /api/circulations", perm("incoming.circulation", handlers.UpdateCirculation))
	mux.Handle("DELETE /api/circulations/{id}", perm("incoming.circulation", handlers.DeleteCirculation))

	// ---- 首页统计 ----
	mux.Handle("GET /api/dashboard-stats", perm("dashboard.view", handlers.DashboardStats))

	// ---- 数据导出（Excel）----
	mux.Handle("GET /api/export/vehicle-applies", perm("vehicle.export", handlers.ExportVehicleApplies))
	mux.Handle("GET /api/export/leave-records", perm("leave.export", handlers.ExportLeaveRecords))
	mux.Handle("GET /api/export/attendances", perm("attendance.export", handlers.ExportAttendances))
	mux.Handle("GET /api/export/duty-schedules", perm("duty.export", handlers.ExportDutySchedules))
	mux.Handle("GET /api/export/incoming-docs", perm("incoming.export", handlers.ExportIncomingDocs))
	mux.Handle("GET /api/export/calendar-tasks", perm("calendar.export", handlers.ExportCalendarTasks))
	mux.Handle("GET /api/export/overtime-records", perm("overtime.export", handlers.ExportOvertimeRecords))
	mux.Handle("GET /api/export/annual-leave-configs", perm("annualleave.export", handlers.ExportAnnualLeaveConfigs))

	// ---- 数据导出（Word）----
	mux.Handle("GET /api/export/standing-events", perm("standing.manage", handlers.ExportStandingCommitteeEvents))
	mux.Handle("GET /api/export/major-events", perm("event.export", handlers.ExportMajorEvents))
	mux.Handle("GET /api/export/weekly-summaries", perm("weekly.export", handlers.ExportWeeklySummaries))

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
	http.ServeFile(w, r, filepath.Join(h.staticDir, "index.html"))
}
