# 伊宁县委宣传部部务工作平台 — 架构梳理文档

> 更新日期：2026-08-25
> 状态：现行架构快照，供技术演进参考（尚未重构）

## 一、总体架构

单体全栈应用，Go 单二进制同时提供 API 和前端静态资源，SQLite 单文件数据库，Nginx 反代 HTTPS。

```
浏览器 (Vue3 SPA + Element Plus)
   │  HTTPS
   ▼
Nginx (443, 反代 + 静态缓存)
   │  127.0.0.1:8080
   ▼
Go 单二进制 (ynxwxcb-server, ~10MB)
   ├─ Router (http.ServeMux, Go1.22 模式路由)
   ├─ Middleware: Auth(JWT) → RequireRole
   ├─ Handlers: 12 大业务域
   └─ SQLite (WAL) + 文件存储 (data/uploads/)
```

## 二、技术栈

| 层 | 技术 | 版本/说明 |
|---|---|---|
| 前端 | Vue 3 (Composition API) | 3.5.x |
| UI | Element Plus | 2.9.x + zh-cn |
| 前端路由 | Vue Router | 4.5.x history 模式 |
| 状态 | Pinia | 2.x（仅 auth） |
| HTTP | Axios + 拦截器 | token 注入 / 401 跳转 |
| 后端 | Go net/http | 1.25，标准库，未用 Web 框架 |
| 数据库 | SQLite (modernc.org/sqlite) | 纯 Go 无 CGO，WAL，MaxOpenConns=1 |
| Excel 导出 | excelize/v2 | 纯 Go 生成 xlsx |
| Word 导出 | 标准库手工构造 docx | internal/handlers/docx.go，无需第三方依赖 |
| 认证 | JWT HS256 | golang-jwt/v5，24h 无刷新 |
| 密码 | bcrypt | golang.org/x/crypto |
| 品牌资源 | 官方党徽 PNG (1024x1024) | src/assets/danghui.png + public/favicon.png |
| 部署 | Nginx + systemd + crontab | 单机单进程，备份脚本 |

## 三、后端代码结构 (backend/internal)

```
cmd/server/main.go      入口：配置→JWT→数据库→启动
├── config/             JSON 配置（server/database/jwt/upload/admin）
├── database/           建表 + 迁移 + 种子数据 + 密码哈希
├── models/             数据模型
├── auth/               JWT 生成/解析
├── middleware/         Auth / RequireRole / JSON 响应辅助
├── handlers/           业务逻辑（SQL 直写，无 service 层）
│   ├── auth.go         登录/资料/改密/用户管理/角色/部门
│   ├── incoming.go     收文登记/传阅（含呈批单/传阅卡数据源）
│   ├── attendance.go   考勤点到（含迟到）+ 请假管理 + 月度/年度统计
│   ├── vehicle.go      公车信息 + 用车报备
│   ├── misc.go         通讯录 + 值守排班
│   ├── calendar.go     工作日历（全屏大日历，各科室任务）
│   ├── standing.go     常委管理（常委大事记，仅管理员，按月分组导出 Word）
│   ├── events.go       各科室大事记（每月，按年汇总导出 Word 公文格式）
│   ├── weekly.go       每周工作总结（各科室，管理员汇总导出 Word）
│   ├── study.go        公共资料
│   ├── export.go       Excel 导出（6 模块台账）
│   ├── docx.go         Word 导出工具（标准库构造 docx）
│   └── uploads.go      文件上传下载 + 首页统计
└── router/             全部路由集中注册（单文件）
```

## 四、数据库（SQLite 19 张表）

- 系统：users / roles / departments
- 业务：attendances / leave_records
-       vehicles / vehicle_applies
-       contacts / duty_schedules / calendar_tasks
-       standing_committee_events / major_events / weekly_summaries
-       incoming_docs + circulation_records / attachments
-       study_materials / study_categories

> 业务规则要点：
> - 值守排班：当天值守至 21:00 收文，一天可安排一至两人（UNIQUE duty_date, user_id），`is_dawangyuan` 标记当天是否同时有县委大院排班
> - 收文管理：incoming_docs 记录上级来文，含文件编号/需退回/是否已退/退回日期；circulation_records 记录传阅人；可打印呈批单、传阅登记卡、80×50mm 热敏标签
> - 考勤：管理员晨会手工点到（出勤/请假/出差/未到/迟到），请假细分年假/病假/事假等，支持月度/年度统计与打印
> - 工作日历：calendar_tasks 记录各科室要做的工作（start_date/end_date 支持跨天），全屏日历按月/按年展示，Excel 导出可单科室/全部
> - 常委管理：standing_committee_events 记录常委大事记（不填姓名），仅管理员可操作，Word 导出按月分组公文格式
> - 大事记：major_events 记录各科室每月重大事项（period: YYYY-MM），按年汇总整个宣传部导出 Word（仅管理员，按月公文格式）
> - 每周工作总结：weekly_summaries 记录各科室本周重点工作（week_start/week_end），各科室自管，管理员可增删改全部并汇总导出 Word（仅管理员）

### 数据库版本迁移机制

程序启动时按版本号顺序执行未执行的迁移（`database.go` 的 `migrate()`）：

- `schema_versions` 表记录当前已应用的版本号，已执行的迁移不会重复执行
- 迁移定义在 `migrations` 切片中，每个条目 = 版本号 + 描述 + 执行函数
- **升级数据库只需新增一个迁移函数并追加到切片末尾**，启动时自动执行，旧数据保留
- 新增表走 `createTables()`（`CREATE TABLE IF NOT EXISTS`），加字段/改结构走迁移
- 升级前建议先备份 `data/`（见 `deploy/backup.sh`）

```
示例：未来新增"值班日志"模块
migrations = []migration{
    {1, "初始化表结构与历史兼容迁移", migrateV1},
    {2, "新增值班日志表", migrateV2},   // ← 新增这个
}
```

#### 迁移规范（请遵守）

1. **已发布的迁移永远不回改**。只追加新版本号，保证新旧环境行为一致
2. **加字段**：`ALTER TABLE ... ADD COLUMN`（如 v2 的 driver_name），简单、保留数据
3. **改结构/重建表**：建新表 → 拷数据 → 删旧表 → 改名（如 migrateV1 的排班表重建）
4. **幂等**：迁移用 `hasColumn()`/`IF NOT EXISTS` 判断，重复执行不报错
5. **备份先行**：升级前跑 `backup.sh`，数据库出问题可还原
6. **远期（可选）**：若迁移积累到几十个，可将 v1~vN 合并为一个"基线"（直接建最新结构），新部署只跑基线 + 之后的迁移

## 五、前端结构 (frontend/src)

```
router/index.js      路由表（meta.admin 权限标记）
store/auth.js       登录态 + 角色判断
utils/request.js    Axios 封装 + exportFile 导出下载
views/              22 页面
  Layout / Login / Dashboard
  IncomingDocs(+3打印: 呈批单/传阅卡/标签)
  Attendance(+1统计打印) / Leave / Vehicles(+1派车单打印)
  Study / Contacts / Duty / WorkCalendar / MajorEvents
  WeeklySummary / StandingCommittee / Users / Profile
```

## 六、已知架构问题

1. **Handler 职责混杂**：misc.go 为"杂项"包（通讯录+排班）；uploads.go 含文件+统计
2. **无 service/repository 层**：SQL 直写在 handler，难测试、难复用、改动易遗漏
3. **路由权限硬编码**：router.go 单文件，模块增多会失控；无统一响应封装
4. **SQLite 单写并发限制**：MaxOpenConns=1，当前可接受，量级上来是瓶颈
5. **前端权限依赖 localStorage**：role_code 明文存于前端，安全依赖后端校验（后端已校验）
6. **无自动化测试 / 无 CI / 无日志分级**
7. **Word 导出为手写精简 docx**：无复杂表格/图片排版能力，仅文本段落，已满足现行公文需求

## 七、结论与后续方向（暂不实施）

- 保持 SQLite（适合 2C2G 单机，零运维）
- 若重构，优先：按业务域拆分 handler → 抽 service/repository → 统一响应结构 → 补 go test
- 备选：数据库换 PostgreSQL（多写并发）、加 Redis 缓存（高频接口）
