package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init 初始化数据库连接和表结构
func Init(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %v", err)
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %v", err)
	}

	DB.SetMaxOpenConns(1) // SQLite 单写者，避免锁竞争
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(time.Hour)

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %v", err)
	}

	// 开启 WAL 模式提升并发读性能
	DB.Exec("PRAGMA journal_mode=WAL")
	DB.Exec("PRAGMA busy_timeout=5000")

	if err := createTables(); err != nil {
		return err
	}

	if err := migrate(); err != nil {
		return err
	}

	if err := seed(); err != nil {
		return err
	}

	return nil
}

// migrate 版本号迁移：按版本顺序执行未执行的迁移
// 数据库维护 schema_versions 表记录当前版本，每次启动从当前版本向后执行
func migrate() error {
	// 确保版本表存在
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("创建版本表失败: %v", err)
	}

	// 读取当前版本
	current := 0
	DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&current)

	// 按顺序执行高于当前版本的迁移
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := m.fn(); err != nil {
			return fmt.Errorf("迁移 v%d（%s）失败: %v", m.version, m.desc, err)
		}
		if _, err := DB.Exec("INSERT INTO schema_versions (version) VALUES (?)", m.version); err != nil {
			return fmt.Errorf("记录迁移版本 v%d 失败: %v", m.version, err)
		}
	}
	return nil
}

// migration 迁移条目
type migration struct {
	version int
	desc    string
	fn      func() error
}

// 所有迁移，按版本号升序排列；新增迁移只需追加新版本号条目
var migrations = []migration{
	{1, "初始化表结构与历史兼容迁移", migrateV1},
	{2, "用车报备增加开车人字段", migrateV2},
	{3, "公共资料分类管理表", migrateV3},
	{4, "新增分管领导角色", migrateV4},
	{5, "新增工作日历/常委大事记/各科室大事记/每周工作总结，废除周月年报", migrateV5},
	{6, "新增加班记录表（加班统计与补休）", migrateV6},
	{7, "清理废弃列：大事记/常委大事记去掉冗余字段", migrateV7},
}

// migrateV2 版本2：用车报备支持科室人开车（增加 driver_name 字段）
func migrateV2() error {
	if !hasColumn("vehicle_applies", "driver_name") {
		_, err := DB.Exec("ALTER TABLE vehicle_applies ADD COLUMN driver_name TEXT")
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateV3 版本3：公共资料分类管理表 + 预置默认分类
func migrateV3() error {
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS study_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		code TEXT NOT NULL UNIQUE,
		sort INTEGER DEFAULT 0
	)`); err != nil {
		return err
	}
	var cnt int
	DB.QueryRow("SELECT COUNT(*) FROM study_categories").Scan(&cnt)
	if cnt == 0 {
		defaults := []struct{ name, code string }{
			{"理论学习", "theory"},
			{"业务知识", "business"},
			{"政策文件", "policy"},
			{"其他", "other"},
		}
		for i, c := range defaults {
			_, err := DB.Exec("INSERT INTO study_categories (name, code, sort) VALUES (?, ?, ?)", c.name, c.code, i+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateV4 版本4：新增分管领导角色
func migrateV4() error {
	var cnt int
	DB.QueryRow("SELECT COUNT(*) FROM roles WHERE code = 'leader'").Scan(&cnt)
	if cnt == 0 {
		_, err := DB.Exec("INSERT INTO roles (name, code, description) VALUES (?, ?, ?)",
			"分管领导", "leader", "分管领导，查看与审阅权限")
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateV5 版本5：新增工作日历、常委大事记、各科室大事记、每周工作总结四张表，废除周月年报 reports 表
func migrateV5() error {
	// 工作日历（各科室日历格子中添加要做的工作）
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS calendar_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		department_id INTEGER,
		title TEXT NOT NULL,
		content TEXT,
		start_date TEXT,
		end_date TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	// 常委大事记（仅记录常委个人情况，仅管理员操作）
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS standing_committee_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_date TEXT,
		member_name TEXT,
		title TEXT NOT NULL,
		content TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	// 各科室大事记（每月/每年重大事项，替代周月年报）
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS major_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		department_id INTEGER,
		event_type TEXT,
		period TEXT,
		title TEXT NOT NULL,
		content TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	// 每周工作总结（各科室录入本周重点工作）
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS weekly_summaries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		department_id INTEGER,
		week_start TEXT,
		week_end TEXT,
		content TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	DB.Exec("CREATE INDEX IF NOT EXISTS idx_calendar_date ON calendar_tasks(start_date)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_calendar_dept ON calendar_tasks(department_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_sc_events_date ON standing_committee_events(event_date)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_me_dept ON major_events(department_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_me_period ON major_events(period)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_ws_dept ON weekly_summaries(department_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_ws_week ON weekly_summaries(week_start)")

	// 废除旧周月年报表（数据无保留价值，整体重做）
	DB.Exec("DROP TABLE IF EXISTS reports")
	return nil
}

// migrateV6 版本6：新增加班记录表，用于加班统计与补休管理
func migrateV6() error {
	// 加班记录：日期 + 人员 + 加班小时数 + 事由；补休在 leave_records 中用 leave_type='comp' 登记
	if _, err := DB.Exec(`CREATE TABLE IF NOT EXISTS overtime_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		overtime_date TEXT,
		hours REAL DEFAULT 0,
		reason TEXT,
		created_by INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_ot_user ON overtime_records(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_ot_date ON overtime_records(overtime_date)")
	return nil
}

// migrateV7 版本7：清理废弃列（V1.3.7 表单去掉了大事记详情、常委姓名与详情）
func migrateV7() error {
	// 清理废弃列：major_events.content（大事记已删详情框）
	if hasColumn("major_events", "content") {
		DB.Exec(`CREATE TABLE major_events_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			department_id INTEGER,
			event_type TEXT,
			period TEXT,
			title TEXT NOT NULL,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		DB.Exec(`INSERT OR IGNORE INTO major_events_new (id, department_id, event_type, period, title, created_by, created_at, updated_at)
			SELECT id, department_id, event_type, period, title, created_by, created_at, updated_at FROM major_events`)
		DB.Exec("DROP TABLE major_events")
		DB.Exec("ALTER TABLE major_events_new RENAME TO major_events")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_me_dept ON major_events(department_id)")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_me_period ON major_events(period)")
	}

	// 清理废弃列：standing_committee_events.member_name / content（常委不填姓名、无详情）
	if hasColumn("standing_committee_events", "member_name") {
		DB.Exec(`CREATE TABLE standing_committee_events_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_date TEXT,
			title TEXT NOT NULL,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		DB.Exec(`INSERT OR IGNORE INTO standing_committee_events_new (id, event_date, title, created_by, created_at, updated_at)
			SELECT id, event_date, title, created_by, created_at, updated_at FROM standing_committee_events`)
		DB.Exec("DROP TABLE standing_committee_events")
		DB.Exec("ALTER TABLE standing_committee_events_new RENAME TO standing_committee_events")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_sc_events_date ON standing_committee_events(event_date)")
	}

	return nil
}

// migrateV1 版本1：历史兼容迁移
// 新库由 createTables() 直接建出最新结构；老库通过条件判断补齐/重建
func migrateV1() error {
	// 迁移条件1：旧结构（含 shift 列）或含 is_xianweiban 列的表，重建为最新结构
	// 迁移条件2：duty_schedules 仅有单列 UNIQUE(duty_date)，需升级为 UNIQUE(duty_date, user_id) 支持一天两人
	if hasColumn("duty_schedules", "shift") || hasColumn("duty_schedules", "is_xianweiban") || hasSingleDateUnique("duty_schedules") {
		DB.Exec(`CREATE TABLE duty_schedules_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			duty_date TEXT,
			user_id INTEGER,
			is_dawangyuan INTEGER DEFAULT 0,
			note TEXT,
			status INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(duty_date, user_id)
		)`)
		DB.Exec(`INSERT OR IGNORE INTO duty_schedules_new (id, duty_date, user_id, is_dawangyuan, note, status, created_at)
			SELECT id, duty_date, user_id, is_dawangyuan, note, status, created_at FROM duty_schedules`)
		DB.Exec("DROP TABLE duty_schedules")
		DB.Exec("ALTER TABLE duty_schedules_new RENAME TO duty_schedules")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_schedule_date ON duty_schedules(duty_date)")
	}

	// 考勤表迁移：旧结构含 check_in_time 打卡字段 → 重建为点到结构
	if hasColumn("attendances", "check_in_time") {
		DB.Exec(`CREATE TABLE attendances_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			attend_date TEXT,
			status INTEGER DEFAULT 1,
			leave_type TEXT,
			remark TEXT,
			marked_by INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, attend_date)
		)`)
		DB.Exec(`INSERT OR IGNORE INTO attendances_new (id, user_id, attend_date, status, remark, created_at, updated_at)
			SELECT id, user_id, attend_date, status, remark, created_at, updated_at FROM attendances`)
		DB.Exec("DROP TABLE attendances")
		DB.Exec("ALTER TABLE attendances_new RENAME TO attendances")
	}

	// 考勤表补充列：已有表但缺 leave_type 时补充
	if !hasColumn("attendances", "leave_type") {
		DB.Exec("ALTER TABLE attendances ADD COLUMN leave_type TEXT")
	}

	// 用车表迁移：旧结构含审批字段 → 重建为报备结构
	if hasColumn("vehicle_applies", "approver_id") {
		DB.Exec(`CREATE TABLE vehicle_applies_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vehicle_id INTEGER,
			reporter_id INTEGER,
			user_name TEXT,
			purpose TEXT,
			destination TEXT,
			use_date TEXT,
			use_time TEXT,
			passengers INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		DB.Exec(`INSERT OR IGNORE INTO vehicle_applies_new (id, vehicle_id, reporter_id, purpose, destination, created_at)
			SELECT id, vehicle_id, applicant_id, purpose, destination, created_at FROM vehicle_applies`)
		DB.Exec("DROP TABLE vehicle_applies")
		DB.Exec("ALTER TABLE vehicle_applies_new RENAME TO vehicle_applies")
	}

	// 确保新结构索引存在（放在迁移之后，避免旧表结构下建索引报错）
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_att_user ON attendances(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_att_date ON attendances(attend_date)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_apply_vehicle ON vehicle_applies(vehicle_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_apply_reporter ON vehicle_applies(reporter_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_apply_date ON vehicle_applies(use_date)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_leave_user ON leave_records(user_id)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_leave_type ON leave_records(leave_type)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_leave_date ON leave_records(start_date)")

	// 车辆表补充列：已有表缺车架号等字段时补充
	for _, col := range []string{"vin", "engine_no", "insurance_date", "inspect_date", "register_date"} {
		if !hasColumn("vehicles", col) {
			DB.Exec("ALTER TABLE vehicles ADD COLUMN " + col + " TEXT")
		}
	}

	// 收文表补充列：文件编号/退回相关字段
	if !hasColumn("incoming_docs", "doc_no") {
		DB.Exec("ALTER TABLE incoming_docs ADD COLUMN doc_no TEXT")
	}
	if !hasColumn("incoming_docs", "return_date") {
		DB.Exec("ALTER TABLE incoming_docs ADD COLUMN return_date TEXT")
	}
	if !hasColumn("incoming_docs", "returned") {
		DB.Exec("ALTER TABLE incoming_docs ADD COLUMN returned INTEGER DEFAULT 0")
	}
	if !hasColumn("incoming_docs", "need_return") {
		DB.Exec("ALTER TABLE incoming_docs ADD COLUMN need_return INTEGER DEFAULT 0")
	}

	return nil
}

// hasSingleDateUnique 检测 duty_schedules 是否仅有单列 UNIQUE(duty_date) 约束
// 通过 sqlite_master 中的建表语句判断，避免 PRAGMA 嵌套查询死锁
func hasSingleDateUnique(table string) bool {
	var sqlText string
	err := DB.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&sqlText)
	if err != nil {
		return false
	}
	// 单列约束形式：UNIQUE(duty_date)（后面紧跟 ) 或 ,）区别于 UNIQUE(duty_date, user_id)
	if strings.Contains(sqlText, "UNIQUE(duty_date)") {
		return true
	}
	return false
}

// hasColumn 检查表是否存在某列
// 使用 pragma_table_info 表值函数，单条查询避免 PRAGMA 嵌套死锁
// 表名和列名均为代码内常量，无注入风险
func hasColumn(table, col string) bool {
	var cnt int
	err := DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('" + table + "') WHERE name='" + col + "'").Scan(&cnt)
	if err != nil {
		return false
	}
	return cnt > 0
}

func createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS departments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			parent_id INTEGER DEFAULT 0,
			sort INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			code TEXT NOT NULL UNIQUE,
			description TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			real_name TEXT NOT NULL,
			phone TEXT,
			department_id INTEGER DEFAULT 0,
			role_id INTEGER DEFAULT 3,
			status INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS study_materials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT,
			category TEXT,
			publisher_id INTEGER,
			read_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS study_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			code TEXT NOT NULL UNIQUE,
			sort INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS contacts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			phone TEXT,
			department_id INTEGER DEFAULT 0,
			position TEXT,
			is_public INTEGER DEFAULT 1,
			sort INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS duty_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			duty_date TEXT,
			user_id INTEGER,
			is_dawangyuan INTEGER DEFAULT 0,
			note TEXT,
			status INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(duty_date, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			department_id INTEGER,
			title TEXT NOT NULL,
			content TEXT,
			start_date TEXT,
			end_date TEXT,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS standing_committee_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_date TEXT,
			title TEXT NOT NULL,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS major_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			department_id INTEGER,
			event_type TEXT,
			period TEXT,
			title TEXT NOT NULL,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS weekly_summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			department_id INTEGER,
			week_start TEXT,
			week_end TEXT,
			content TEXT,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS overtime_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			overtime_date TEXT,
			hours REAL DEFAULT 0,
			reason TEXT,
			created_by INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS leave_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			leave_type TEXT,
			start_date TEXT,
			end_date TEXT,
			days REAL DEFAULT 0,
			reason TEXT,
			status INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS attendances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			attend_date TEXT,
			status INTEGER DEFAULT 1,
			leave_type TEXT,
			remark TEXT,
			marked_by INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, attend_date)
		)`,
		`CREATE TABLE IF NOT EXISTS vehicles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plate_no TEXT NOT NULL,
			brand TEXT,
			seats INTEGER DEFAULT 5,
			driver TEXT,
			status INTEGER DEFAULT 1,
			vin TEXT,
			engine_no TEXT,
			insurance_date TEXT,
			inspect_date TEXT,
			register_date TEXT,
			purchase_at TEXT,
			note TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS vehicle_applies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vehicle_id INTEGER,
			reporter_id INTEGER,
			user_name TEXT,
			driver_name TEXT,
			purpose TEXT,
			destination TEXT,
			use_date TEXT,
			use_time TEXT,
			passengers INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS incoming_docs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			receive_no TEXT,
			received_date TEXT,
			from_unit TEXT,
			from_doc_no TEXT,
			doc_no TEXT,
			title TEXT NOT NULL,
			copies INTEGER DEFAULT 1,
			secret_level TEXT DEFAULT '普通',
			urgency TEXT DEFAULT '一般',
			suggest TEXT,
			leader_comment TEXT,
			processing TEXT,
			return_date TEXT,
			returned INTEGER DEFAULT 0,
			need_return INTEGER DEFAULT 0,
			registrar_id INTEGER,
			status INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS circulation_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			doc_id INTEGER,
			user_id INTEGER,
			order_no INTEGER DEFAULT 0,
			read_date TEXT,
			signature TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS attachments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_type TEXT,
			owner_id INTEGER,
			file_name TEXT,
			file_path TEXT,
			file_size INTEGER DEFAULT 0,
			uploader_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_schedule_date ON duty_schedules(duty_date)`,
		`CREATE INDEX IF NOT EXISTS idx_incoming_status ON incoming_docs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_incoming_date ON incoming_docs(received_date)`,
		`CREATE INDEX IF NOT EXISTS idx_circ_doc ON circulation_records(doc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_date ON calendar_tasks(start_date)`,
		`CREATE INDEX IF NOT EXISTS idx_calendar_dept ON calendar_tasks(department_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sc_events_date ON standing_committee_events(event_date)`,
		`CREATE INDEX IF NOT EXISTS idx_me_dept ON major_events(department_id)`,
		`CREATE INDEX IF NOT EXISTS idx_me_period ON major_events(period)`,
		`CREATE INDEX IF NOT EXISTS idx_ws_dept ON weekly_summaries(department_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ws_week ON weekly_summaries(week_start)`,
		`CREATE INDEX IF NOT EXISTS idx_ot_user ON overtime_records(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ot_date ON overtime_records(overtime_date)`,
	}

	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("建表失败: %v\nSQL: %s", err, s)
		}
	}
	return nil
}

// seed 初始化基础数据
func seed() error {
	// 角色
	roles := []struct {
		name, code, desc string
	}{
		{"系统管理员", "admin", "系统管理，用户管理，全部权限"},
		{"分管领导", "leader", "分管领导，查看与审阅权限"},
		{"科室工作人员", "staff", "业务办理，日常操作"},
		{"乡镇/通讯员", "reporter", "投稿、接收通知、上报材料"},
	}

	// 检查 roles 是否为空
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM roles").Scan(&count)
	if count == 0 {
		for _, r := range roles {
			_, err := DB.Exec("INSERT INTO roles (name, code, description) VALUES (?, ?, ?)", r.name, r.code, r.desc)
			if err != nil {
				return fmt.Errorf("初始化角色失败: %v", err)
			}
		}
	}

	// 默认部门
	var deptCount int
	DB.QueryRow("SELECT COUNT(*) FROM departments").Scan(&deptCount)
	if deptCount == 0 {
		defaultDepts := []string{"办公室", "新闻宣传科", "理论教育科", "文明创建科", "网络舆情科", "文化文艺科", "乡镇通讯员"}
		for i, name := range defaultDepts {
			_, err := DB.Exec("INSERT INTO departments (name, parent_id, sort) VALUES (?, 0, ?)", name, i+1)
			if err != nil {
				return fmt.Errorf("初始化部门失败: %v", err)
			}
		}
	}

	// 默认管理员 admin / admin123
	var userCount int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if userCount == 0 {
		hash, err := HashPassword("admin123")
		if err != nil {
			return fmt.Errorf("生成默认密码失败: %v", err)
		}
		var roleID int64
		DB.QueryRow("SELECT id FROM roles WHERE code='admin'").Scan(&roleID)
		var deptID int64
		DB.QueryRow("SELECT id FROM departments WHERE name='办公室'").Scan(&deptID)
		_, err = DB.Exec("INSERT INTO users (username, password_hash, real_name, phone, department_id, role_id, status) VALUES (?, ?, ?, ?, ?, ?, 1)",
			"admin", hash, "系统管理员", "13800000000", deptID, roleID)
		if err != nil {
			return fmt.Errorf("初始化管理员失败: %v", err)
		}
	}

	return nil
}
