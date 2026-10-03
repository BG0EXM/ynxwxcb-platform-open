# 伊宁县委宣传部部务工作平台 — 架构梳理文档

> 版本：**V1.5.1** · 更新日期：**2026-10-03**
> 状态：现行权威架构快照

## 一、总体架构

单体全栈应用，Go 单二进制同时提供 API 和前端静态资源，SQLite 单文件数据库，Nginx / Caddy 反代 HTTPS，支持 IPv6 双栈访问。

```
浏览器 (Vue3 SPA + Element Plus + 本地思源宋体子集)
   │  HTTPS (IPv4/IPv6)
   ▼
Nginx / Caddy (443, 反代 + 静态缓存 + WAF)
   │  127.0.0.1:8080
   ▼
Go 单二进制 (ynxwxcb-server, ~15MB)
   ├─ Router (http.ServeMux, Go 1.26 模式路由)
   ├─ Middleware: Auth(JWT) → RequirePerm(动态权限矩阵) + 限流 + 令牌版本校验(token_version)
   ├─ Handlers: 17 大业务域
   └─ SQLite (WAL) + 文件存储 (data/uploads/)
```

## 二、技术栈

| 层 | 技术 | 版本/说明 |
|---|---|---|
| 前端 | Vue 3 (Composition API) | 3.5.x |
| UI | Element Plus | 2.9.x + zh-cn，次级模块全量抽屉式交互（Drawer） |
| 前端路由 | Vue Router | 4.5.x history 模式 |
| 状态 | Pinia | 2.x（auth / 动态权限缓存） |
| HTTP | Axios + 拦截器 | token 注入 / 401 自动跳转 / 异步别名兼容 |
| 后端 | Go net/http | 1.26，标准库，未用 Web 框架 |
| 数据库 | SQLite (modernc.org/sqlite) | 纯 Go 无 CGO，WAL，MaxOpenConns=1，自动迁移（当前 v14） |
| Excel 导出 | excelize/v2 | 纯 Go 生成 xlsx |
| Word 导出 | 标准库手工构造 docx | internal/handlers/docx.go，无需第三方依赖 |
| 认证与安全 | JWT HS256 + 令牌吊销 | golang-jwt/v5 + users.token_version 吊销机制 |
| 密码 | bcrypt | golang.org/x/crypto |
| 品牌与视觉 | 官方党徽 PNG + 思源宋体子集 | 内嵌 740KB 本地 woff2 字体 + 原生 SVG 趋势折线图 |
| 部署 | Nginx / Caddy + systemd + crontab | 单机单进程，全离线可用，支持 IPv6 |

## 三、后端代码结构 (backend/internal)

```
cmd/server/main.go      入口：配置→JWT→数据库→启动
├── config/             JSON 配置（server/database/jwt/upload/admin）
├── database/           建表 + 迁移 + 种子数据 + 密码哈希
├── models/             数据模型
├── auth/               JWT 生成/解析
├── middleware/         Auth / RequirePerm / 限流 / 令牌版本吊销 / JSON 辅助
├── handlers/           业务逻辑（SQL 直写，无 service 层）
│   ├── auth.go         登录/资料/改密/用户管理/角色/部门
│   ├── permissions.go  46 项权限点目录、角色-权限矩阵配置与重载
│   ├── operation_logs.go 管理员操作日志审计（写操作/登录/导出留痕）
│   ├── incoming.go     收文登记/传阅（含呈批单/传阅卡数据源，带兼容别名）
│   ├── attendance.go   考勤点到（含迟到/培训/补休联动）+ 月度/年度统计
│   ├── leave.go        请假管理（按小时/半天、假条审批流打印，带兼容别名）
│   ├── overtime.go     加班工时统计与补休联动（8小时=1天）
│   ├── annualleave.go  年休假天数配置与已休核销联动
│   ├── meeting.go      会务管理（公开匿名报名/移动端回执/签到单导出）
│   ├── vehicle.go      公车信息 + 用车报备 + 派车单打印
│   ├── misc.go         通讯录 + 值守排班
│   ├── calendar.go     工作日历（全屏大日历，各科室任务跨天展示）
│   ├── standing.go     常委管理（常委大事记，仅管理员，按月分组导出 Word）
│   ├── events.go       各科室大事记（每月，按年汇总导出 Word 公文格式）
│   ├── weekly.go       每周工作总结（各科室，管理员汇总导出 Word）
│   ├── study.go        公共资料（附件安全下载）
│   ├── export.go       Excel 导出（8 模块台账）
│   ├── docx.go         Word 导出工具（标准库构造 docx）
│   └── uploads.go      文件上传下载 + 首页统计（含近半年趋势数据聚合）
└── router/             全部路由集中注册（单文件，带单复数与历史别名兼容）
```

## 四、数据库（SQLite 26 张表，迁移至 v14）

- 系统管理：`users` / `roles` / `departments` / `permissions` / `role_permissions` / `operation_logs`
- 考勤与休假：`attendances` / `leave_records` / `overtime_records` / `annual_leave_configs`
- 公务用车：`vehicles` / `vehicle_applies`
- 日常行政：`contacts` / `duty_schedules` / `calendar_tasks`
- 文书大事记：`standing_committee_events` / `major_events` / `weekly_summaries`
- 收文与传阅：`incoming_docs` / `circulation_records` / `attachments`
- 会务协作：`meetings` / `meeting_participants`
- 公共资料：`study_materials` / `study_categories`

> 业务规则要点：
> - 权限控制：采用 46 项功能权限矩阵（`permissions` + `role_permissions` 表），`admin` 拥有全权限（代码旁路），数据范围按科室/本人物理隔离
> - 令牌吊销（v14）：`users.token_version` 在改密、禁用、角色调整、登出时自增，服务端带缓存校验，旧 JWT 立即失效
> - 值守排班：当天值守至 21:00 收文，一天最多可安排 3 人值守，`is_dawangyuan` 标记县委大院排班
> - 收文管理：上级来文登记与传阅流程，可打印呈批单、传阅登记卡、80×50mm 热敏标签
> - 会务管理：免登录匿名公开报名（`/meeting/:id`），支持单人/多人参会、不参加事由登记、会议签到单全量导出
> - 考勤与休假联动：点到自动识别请假与补休；请假支持天数与小时假；加班与补休联动扣减；年休假配置与已休精准核算

### 数据库版本迁移机制（当前：v14）

程序启动时按版本号升序依次执行未应用的迁移（定义于 `backend/internal/database/database.go` 的 `migrate()`）：

- `schema_versions` 表记录当前已应用的版本号（`version INTEGER PRIMARY KEY, applied_at DATETIME`），已执行的迁移绝不重复执行
- 迁移定义在 `migrations` 切片中，每个条目包含 `{version, desc, fn}`
- **平滑升级**：升级数据库只需追加新的迁移条目与函数，程序启动时自动执行，用户生产数据完全保留
- 生产环境升级前必须备份数据库（`bash deploy/backup.sh`）

#### 历史迁移版本全览（v1 ~ v14）

| 版本 | 迁移描述 | 影响表与核心逻辑 |
|---|---|---|
| **v1** | 初始化表结构与历史兼容迁移 | 建核心基础表；重建排班表结构以保证幂等 |
| **v2** | 用车报备增加开车人字段 | `vehicle_applies` 表增加 `driver_name` 字段 |
| **v3** | 公共资料分类管理表 | 新增 `study_categories` 表，支持资料分类沉淀 |
| **v4** | 新增分管领导角色 | `roles` 表初始化插入 `leader` 角色 |
| **v5** | 新增工作日历/常委大事记/大事记/每周总结 | 新建 4 张业务表，全面废除旧版 reports 周月年报 |
| **v6** | 新增加班记录表 | 新增 `overtime_records` 表，支持加班工时统计与补休联动 |
| **v7** | 清理废弃列 | 清理大事记与常委大事记早期设计的冗余字段 |
| **v8** | 新增年休假配置表 | 新增 `annual_leave_configs` 表，记录每人每年核定天数 |
| **v9** | 新增会务管理表 | 新增 `meetings`、`meeting_participants` 会议与报名表 |
| **v10** | 会务增加参会人数上限 | `meetings` 表增加 `unit_limit`（单位参会人数限制） |
| **v11** | 请假支持按小时/半天 | `leave_records` 增加 `leave_hours`，支持精细化折算 |
| **v12** | 新增操作日志表 | 新增 `operation_logs` 表，管理员写操作与导出审计留痕 |
| **v13** | 新增权限点与角色-权限矩阵 | 新增 `permissions`、`role_permissions` 表，46 项权限矩阵化 |
| **v14** | 用户令牌版本吊销机制 | `users` 表增加 `token_version`，支持改密/禁用/登出即时吊销 |

#### 迁移六大规范（铁律，绝不破坏）

1. **已发布的迁移永不回改**：严禁修改历史迁移函数逻辑，只追加新版本号，保证任意旧环境平滑升至最新版
2. **字段变更优先使用 `ALTER TABLE ADD COLUMN`**：简单轻量，保留原有历史数据
3. **复杂结构调整使用原子重建**：建新临时表 → 复制数据迁移 → 删除旧表 → 临时表重命名为原表
4. **迁移函数必须严格幂等**：使用 `hasColumn()` 或 `IF NOT EXISTS` 判断，即使异常重跑也不报错
5. **严禁依赖 AUTOINCREMENT 跳号**：核心表通过单条 `INSERT INTO ... SELECT COALESCE(MAX(id),0)+1` 取号实现 ID 连续与复用
6. **备份先行原则**：生产升级前执行 `backup.sh`，任何迁移失败均有回滚保障

## 五、前端结构 (frontend/src)

```
components/          通用政务风组件（0 依赖）
  PageHeader / StatCard / StatusDot / TrendChart(原生SVG) / ActionMenu / EmptyState
router/index.js      前端路由（动态权限守卫 + 免登白名单放行）
store/auth.js       登录态 + Pinia 状态（即时同步清除 + 权限校验）
utils/request.js    Axios 封装 + exportFile 导出下载 + 别名兼容
styles/             政务视觉规范（tokens.css / theme.css 鎏金细顶线与呼吸环）
views/              26 个业务与管理页面
  Layout / Login / Dashboard / Meetings / MeetingRegister
  IncomingDocs(+3打印) / Attendance(+1统计打印) / Leave(+1假条打印)
  Vehicles(+1派车单打印) / Overtime / AnnualLeave / Study / Contacts
  Duty / WorkCalendar / MajorEvents / WeeklySummary / StandingCommittee
  Users / Permissions / OperationLogs / Profile
```

## 六、关键演进与加固记录

1. **权限架构重构（V1.4.2）**：将最初硬编码角色判断全面升级为 46 个权限点的“角色-功能权限矩阵”，支持管理员动态勾选与实时内存热重载。
2. **令牌实时吊销（V1.4.2）**：引入 `token_version` 版本号，彻底解决传统 JWT 无法在服务端立即注销、改密或禁用账号后仍可偷跑的缺陷。
3. **安全防线建立**：登录用户名+IP 双重限流、公开接口频率限制、安全响应头（CSP/nosniff/X-Frame-Options）、大请求体拦截。
4. **视觉与交互重构（V1.5.0）**：全站次级模块抽屉化（Drawer）、内置 740KB 思源宋体离线子集、0 依赖原生 SVG 趋势图、鎏金卡片与呼吸状态灯。
5. **健壮性修复**：退出登录 0ms 同步清除凭据（防路由守卫死锁）、后端核心接口单复数兼容路由、打印单独立访问降级兜底、长文本自适应折行。

## 七、结论与后续方向（暂不实施）

- 保持 SQLite（适合 2C2G 单机，零运维）
- 若重构，优先：按业务域拆分 handler → 抽 service/repository → 统一响应结构 → 补 go test
- 备选：数据库换 PostgreSQL（多写并发）、加 Redis 缓存（高频接口）
