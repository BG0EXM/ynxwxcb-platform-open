package database

// Permission 权限点定义
type Permission struct {
	Code   string `json:"code"`   // 权限点标识（英文，前后端统一）
	Name   string `json:"name"`   // 中文名称
	Module string `json:"module"` // 所属模块（用于矩阵分组展示）
	Sort   int    `json:"sort"`   // 排序
}

// PermissionCatalog 全部权限点（顺序即展示顺序）
var PermissionCatalog = []Permission{
	// 系统管理
	{"user.manage", "用户管理（增删改/重置密码）", "系统管理", 1},
	{"department.manage", "部门管理", "系统管理", 2},
	{"oplog.view", "查看操作日志", "系统管理", 3},
	// 收文管理
	{"incoming.view", "查看收文", "收文管理", 10},
	{"incoming.manage", "登记/编辑/删除收文", "收文管理", 11},
	{"incoming.circulation", "管理传阅登记", "收文管理", 12},
	{"incoming.export", "导出收文台账", "收文管理", 13},
	// 通讯录
	{"contact.view", "查看通讯录", "通讯录", 20},
	{"contact.manage", "增删改联系人", "通讯录", 21},
	// 值守排班
	{"duty.view", "查看值守排班", "值守排班", 30},
	{"duty.manage", "排班/删除", "值守排班", 31},
	{"duty.export", "导出值守排班", "值守排班", 32},
	// 工作日历
	{"calendar.view", "查看工作日历", "工作日历", 40},
	{"calendar.manage", "新增/修改/删除工作", "工作日历", 41},
	{"calendar.export", "导出工作日历", "工作日历", 42},
	// 常委管理
	{"standing.manage", "常委大事记（含导出）", "常委管理", 50},
	// 大事记
	{"event.view", "查看大事记", "大事记", 60},
	{"event.manage", "录入/修改/删除大事记", "大事记", 61},
	{"event.export", "导出大事记 Word", "大事记", 62},
	// 每周工作总结
	{"weekly.view", "查看每周总结", "每周工作总结", 70},
	{"weekly.manage", "录入/修改/删除", "每周工作总结", 71},
	{"weekly.export", "导出每周总结 Word", "每周工作总结", 72},
	// 加班管理
	{"overtime.view", "查看加班/补休", "加班管理", 80},
	{"overtime.manage", "录入/删除加班", "加班管理", 81},
	{"overtime.export", "导出加班", "加班管理", 82},
	// 年休假管理
	{"annualleave.view", "查看年休假", "年休假管理", 90},
	{"annualleave.manage", "配置年休假", "年休假管理", 91},
	{"annualleave.export", "导出年休假", "年休假管理", 92},
	// 会务管理
	{"meeting.manage", "会务管理（含导出签到单）", "会务管理", 100},
	// 考勤管理
	{"attendance.mark", "考勤点到", "考勤管理", 110},
	{"attendance.view", "查看考勤记录", "考勤管理", 111},
	{"attendance.stats", "考勤统计（日/月/年）", "考勤管理", 112},
	{"attendance.export", "导出考勤", "考勤管理", 113},
	// 请假管理
	{"leave.view", "查看请假", "请假管理", 120},
	{"leave.manage", "登记/修改请假", "请假管理", 121},
	{"leave.delete", "删除请假", "请假管理", 122},
	{"leave.export", "导出请假", "请假管理", 123},
	// 公车管理
	{"vehicle.view", "查看公车/报备", "公车管理", 130},
	{"vehicle.manage", "车辆增删改", "公车管理", 131},
	{"vehicle.apply", "用车报备", "公车管理", 132},
	{"vehicle.export", "导出用车", "公车管理", 133},
	// 公共资料
	{"study.view", "查看公共资料", "公共资料", 140},
	{"study.publish", "发布资料", "公共资料", 141},
	{"study.delete", "删除资料", "公共资料", 142},
	{"study.category", "资料分类管理", "公共资料", 143},
	// 工作台
	{"dashboard.view", "工作台", "工作台", 150},
}

// businessDefault 普通业务角色（leader/staff/reporter）默认拥有的权限点
// 收文 4 项默认不给（仅 admin），可在权限矩阵中再开
var businessDefault = []string{
	"contact.view", "contact.manage",
	"duty.view", "duty.export",
	"calendar.view", "calendar.manage", "calendar.export",
	"event.view", "event.manage",
	"weekly.view", "weekly.manage",
	"overtime.view",
	"annualleave.view",
	"attendance.view", "attendance.export",
	"leave.view", "leave.manage", "leave.export",
	"vehicle.view", "vehicle.apply", "vehicle.export",
	"study.view", "study.publish",
	"dashboard.view",
}

// DefaultRolePermissions 各角色默认权限（admin 由代码旁路，始终全权限）
var DefaultRolePermissions = map[string][]string{
	"admin":    allPermissionCodes(),
	"leader":   businessDefault,
	"staff":    businessDefault,
	"reporter": businessDefault,
}

// allPermissionCodes 返回全部权限点标识
func allPermissionCodes() []string {
	codes := make([]string, 0, len(PermissionCatalog))
	for _, p := range PermissionCatalog {
		codes = append(codes, p.Code)
	}
	return codes
}
