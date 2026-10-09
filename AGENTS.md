# 项目工作流 — AGENTS.md

> 本文件是 AI 助手接手本项目的**权威工作流说明**，新会话自动读取。请严格遵循。  
> 当前版本：**V1.7.0**（更新日期：**2026-10-09**）

---

## 一、项目概述与双版本体系

**伊宁县委宣传部部务工作平台**（ynxwxcb-platform），县级党委宣传部内部部务工作平台，单机部署、离线可用、开源（Apache 2.0）。

### 1. 核心功能与侧边栏布局
- **工作台**：置顶核心统计指标、快捷操作、值班看板与业务趋势图。
- **业务办理**：
  - **收文管理**：单一办理状态闭环/呈批单/传阅登记卡/热敏标签打印；
  - **征求意见**（新）：公文草案发起/范围单位管控/外链分发/各单位意见汇编/无意见极速答复/防重复提交；
  - **材料下发**（新）：非回执类政务公文与材料查收即止下发/各单位查收率实时追踪/经办人实名存证；
  - **会务管理**：一键生成规范公文通知文案/公开匿名报名/签到册导出/管理员调度协调；
  - **日常业务**：公车报备、值守排班、工作日历（全屏大日历）、通讯录。
- **考勤休假**：考勤点到（含迟到/培训）、请假管理（含产检/探亲/培训/补休、支持按小时/半天）、加班管理（工时统计与补休联动）、年休假管理（配置+已休联动）。
- **材料报送**：各科室大事记（按月沉淀/按年导出）、每周工作总结、公共资料（分类沉淀）。
- **系统管理**（仅 admin）：常委管理（常委大事记）、用户管理、权限管理（**48项角色-功能矩阵**）、操作日志（安全审计与涉密阻断审计存证）。

### 2. 双版本体系（核心红线，严禁混淆）

| 维度 | 工程版（本地开发） | 开源版（GitHub 仓库） |
|---|---|---|
| **物理路径** | `/Users/dingwenjie/Documents/ynxwxcb-platform/ynxwxcb-platform` | `/Users/dingwenjie/Documents/ynxwxcb-platform/ynxwxcb-platform-open` |
| **备案号信息** | **真实备案号**（`新ICP备2023002840号-3` / `新公网安备65400202000233号`） | **占位符**（`[滇ICP备XXXXXXXX号-X]` 等） |
| **用途与归属** | 作者日常自用开发、构建自用部署包传生产服务器 | GitHub 远程开源发布（`BG0EXM/ynxwxcb-platform-open`） |
| **Git 仓库属性** | **不是 Git 仓库，绝对不准初始化 Git 或 Push 到 GitHub** | **是 Git 仓库**，绑定 GitHub 远程 `master` 分支 |
| **敏感配置文件** | 持有 `config.json`（内含生产 JWT 密钥及配置） | 仅持有 `deploy/config.json.example`，**绝不同步真实 config** |

> **⚠️ 公开页面同步与脱敏铁律（至关重要）**：
> 全站目前共有 **4 个带真实备案号的公开页面**：
> 1. 公开登录页 `frontend/src/views/Login.vue`
> 2. 公开会务报名页 `frontend/src/views/MeetingRegister.vue`
> 3. 公开征求意见反馈页 `frontend/src/views/SolicitFeedback.vue`（新）
> 4. 公开材料下发查收页 `frontend/src/views/DispatchReceipt.vue`（新）
> **每次有代码更新，必须同步最新代码到开源版，并将上述 4 个页面中的真实备案号统一替换为占位符**。**绝对不能整文件排除**！同步后必须全量 `grep -rn "新ICP备"` 验证真实备案号匹配数严格为 0！

---

## 二、当前版本核心架构（V1.7.0）

- **软件版本**：**V1.7.0**
- **数据库迁移版本**：**schema_versions = 18**
- **权限矩阵点数**：**48 项功能权限**

### 近期架构重大演进
- **V1.7.0（业务扩展、国家保密防线与全端安全加固）**：
  - **业务版图拓展**：全新上线“征求意见”与“材料下发”两大高频政务协同模块；会务管理支持“一键通知会议”公文规范文本生成；标准化全县 16 个乡镇与 56 家县直单位快捷填充体系。
  - **构建国家保密安全防线**：全链路集成四维内文涉密检测引擎（文件名 + Word XML 解构 + PDF 字符集/CMap 文本流还原 + 扫描件/图片 OCR 智能提取）；基于 B/S 架构服务端处理原则，客户电脑/手机零安装零依赖，服务端支持 Mac 原生 Vision / Linux 通用 Tesseract 自动探测；全屏高科技保密违法阻断警告（`SecrecyAlert.vue`），严格引用保密法第二十九条，30 秒安全熔断锁定会话；涉密违规与 WAF 拦截真实入库操作日志审计。
  - **安全与移动端深度适配**：修复 WAF 告警特定指令下的中文乱码；全面重构 `WafAlert.vue` 与 `SecrecyAlert.vue`，彻底消除固定宽度，实现桌面端一屏全景（防滚动溢出）与手机竖屏（`< 640px`）单手全宽触控深度适配。
- **V1.6.2（全站响应式深度重构）**：彻底取消独立移动端 H5 架构（移除 `/mobile` 视图、路由及 Vant 依赖）；PC 端全模块深度适配手机屏幕（360px~768px）；侧栏自适应平滑抽屉滑出；弹窗/抽屉小屏全宽展示；表单栅格单列化；日历横滑保护；输入框 16px 锁定根治 iOS Safari 聚焦放大。
- **V1.6.1（收文闭环与排版重构）**：彻底退役流转状态，全站统一回归标准“办理状态”（1待登记、2拟办中、3待批示、4办理中、5已办结）；快捷办理与归档弹窗升级；收文列表黄金视区重构，标题弹性展开，操作列扩充至 215px 防裁切。
- **V1.6.0（底层架构全面加固）**：SQLite WAL 连接池读写解限；全量查询补齐 `defer rows.Close()` 消除连接泄漏；切片截取边界防护杜绝 Panic；大表导出锁死 `LIMIT 10000` 消除 OOM；会议与资料补齐真分页；清理空 catch；表单接入规范 `:rules`；高频外键建索引；限流器并发锁与内存优化；全站 WAF 防护与防截屏自愈水印；Vite 按需引入使首屏 JS 打包体积降至 350KB。

---

## 三、技术栈与生产部署结构

### 1. 技术栈
- **后端**：Go 1.26（标准库 `net/http`，`internal/` 分层架构，无三方 Web 框架）。
- **数据库**：SQLite（`modernc.org/sqlite`，纯 Go 无需 CGO），启用 WAL 模式，`MaxOpenConns=20, MaxIdleConns=5`。
- **前端**：Vue 3（Composition API）+ Element Plus 2.9（按需加载）+ Pinia + Vue Router 4 + Vite 6。
- **生态组件**：
  - Excel 报表：`excelize/v2`；
  - 认证与安全：JWT HS256、密码 `bcrypt`、内置 WAF 规则引擎；
  - 文档与公文解析：纯 Go 原生 `docxBuilder`（无三方库）；PDF 解析使用 `github.com/dslipak/pdf` 与 `golang.org/x/text`；
  - 涉密 OCR 识别：本地开发机集成编译 `backend/bin/doc_vision_ocr`（原生视觉引擎，68KB），Linux 生产服务器自动探测对接通用 `tesseract` 引擎；
  - 水印防护：Canvas 顶层动态防篡改自愈水印。
- **开发与构建环境**：Go `~/dev/go/bin`、Node `~/dev/node/bin`；国内直连源（`goproxy.cn` / `npmmirror`）；国际网络使用本机 SOCKS5 代理（`127.0.0.1:1080`）。

### 2. 生产部署规范（单机同目录）
生产环境所有运行组件**必须全部位于 `/opt/ynxwxcb/` 同一目录**：
```text
/opt/ynxwxcb/
├── ynxwxcb-server         # 编译后的 Linux amd64 二进制（权限 755）
├── static/                # 前端构建生成的静态资源（SPA 回退 index.html）
├── config.json            # 生产环境私有配置文件（权限 600，严禁对外泄露）
├── data/                  # 运行时持久化数据目录（含 SQLite 数据库及 uploads 上传文件）
└── deploy/                # 运维辅助脚本与配置（backup.sh / nginx / systemd 等）
```
- **权限与服务保护**：systemd 服务 `ynxwxcb.service` 配置用户 `ynxwxcb`，`ProtectSystem=full`，`ReadWritePaths=/opt/ynxwxcb/data`。
- **升级绝不覆盖**：升级生产时**绝对严禁覆盖 `data/` 目录和 `config.json`**！
- **生产安全红线**：
  - `config.json` 的 `jwt.secret` 必须为 ≥32 位强随机字符串（`openssl rand -base64 48`），启动时会自动拒绝弱密钥、默认密钥或占位符。
  - 生产开启 `users.token_version` 即时吊销，用户改密、重置密码、禁用或退出登录时令牌立即失效。
  - 前置 Caddy/WAF 时 `server.trust_proxy` 设为 `true`。
  - 登录接口配置双重限流（按用户名 5 次锁 10 分钟，按 IP 20 次/分钟）。

---

## 四、数据库迁移规范与历史台账（v1 ~ v18）

### 1. 迁移执行机制与六大铁律
- 库内维护 `schema_versions` 表记录版本与执行时间。程序启动自动对比版本号升序执行未应用的迁移。
- **铁律 1（永不回改）**：已发布的历史迁移条目与函数绝对不修改，保证旧环境平滑升级。
- **铁律 2（轻量优先）**：字段变更优先使用 `ALTER TABLE ADD COLUMN` 配合 `hasColumn()` 保证幂等。
- **铁律 3（原子重建）**：复杂表结构调整使用临时表备份数据后重命名替换。
- **铁律 4（严格幂等）**：所有迁移逻辑必须可重复执行不报错。
- **铁律 5（原子取号）**：连续自增 ID 使用 `INSERT INTO ... SELECT COALESCE(MAX(id),0)+1`，避免依赖 AUTOINCREMENT 跳号。
- **铁律 6（备份先行）**：升级生产前必须先执行 `deploy/backup.sh` 备份数据库。

### 2. 完整迁移台账

| 版本 | 迁移描述 | 影响表与核心逻辑 |
|---|---|---|
| **v1** | 初始化表结构与历史兼容迁移 | 创建核心业务表；幂等重建排班表 |
| **v2** | 用车报备增加开车人 | `vehicle_applies` 表增加 `driver_name` |
| **v3** | 公共资料分类管理 | 新增 `study_categories` 表并预置分类 |
| **v4** | 新增分管领导角色 | `roles` 表插入 `leader` 角色 |
| **v5** | 工作日历/常委大事记/大事记/每周总结 | 新建 4 张业务表，废除旧版周月年报 |
| **v6** | 新增加班记录表 | 新增 `overtime_records`，联动加班统计与补休 |
| **v7** | 清理废弃列 | 清理大事记与常委大事记冗余字段 |
| **v8** | 新增年休假配置表 | 新增 `annual_leave_configs` 记录每人核定天数 |
| **v9** | 新增会务管理表 | 新增 `meetings`、`meeting_participants` 会议及报名表 |
| **v10** | 会务人数上限控制 | `meetings` 表增加 `unit_limit`（单位限额） |
| **v11** | 请假支持按小时/半天 | `leave_records` 增加 `leave_hours` 精细折算 |
| **v12** | 新增操作日志审计表 | 新增 `operation_logs` 记录写操作与导出审计 |
| **v13** | 角色-权限矩阵体系 | 新增 `permissions`、`role_permissions`（基础46项矩阵） |
| **v14** | 用户令牌即时吊销机制 | `users` 增加 `token_version`，改密/登出旧令牌作废 |
| **v15** | 公文在办流转与归档卷盒 | `incoming_docs` 增加 `archive_box_no`、`archive_year` 等 |
| **v16** | 公文承办科室与流转备注 | `incoming_docs` 增加 `assigned_department`、`circulation_notes` |
| **v17** | **新增征求意见模块表（新）** | 新增 `solicits`、`solicit_feedbacks` 表，注入 `solicit.manage` 权限点 |
| **v18** | **新增材料下发模块表（新）** | 新增 `dispatches`、`dispatch_receipts` 表，注入 `dispatch.manage` 权限点 |

---

## 五、核心业务逻辑与权限模型

### 1. 核心业务计算与闭环口径
- **考勤与请假联动**：点到自动匹配当天请假区间并标记；补休（comp）考勤在点到记录中自动转为出勤（`status=1, auto_comp=1`），界面展示“补休”标签。
- **统计口径统一**：考勤月/年报统计以**点到记录**为唯一基准；请假天数计算统一按 `MIN(登记days, 重叠整天数)` 计算实际覆盖时长，严禁简单 `SUM(days)`。
- **会务协同规范**：支持“一键通知会议”自动提取时间、地点、主题及范围生成规范文本；会议开始时间即报名截止时间；单位支持整体报“不参加”并录入事由；签到单导出列出全量单位。
- **征求意见双轨响应**：支持目标单位一键选全县；单位端支持免登录在线阅览公文并双轨答复（“无修改意见”一键确认 / “有修改意见”上传修改稿件）；实时展示反馈率进度看板并支持精准催办。
- **材料下发极速触达**：非回执类公文支持各单位经办人实名查收，自动记录经办人、联系电话、查收时间与 IP，生成查收进度台账。
- **用户与业务 ID 连续性**：删除记录后需重置 `sqlite_sequence` 对齐 `MAX(id)`，保证新建记录编号不跳号。

### 2. 权限矩阵模型
- 权限矩阵扩充至 **48 项功能权限点**（含 `solicit.manage`、`dispatch.manage` 等）。
- **管理员旁路**：`admin` 角色在代码层全局拥有全部权限（旁路机制），且界面不可取消。
- **数据范围隔离**：功能权限由矩阵控制，但**数据范围仍由后端业务逻辑隔离**（普通科室用户请假/用车限本人；大事记/周总结/日历限本科室）。

---

## 六、版本发布全生命周期流程（铁律，按序）

```text
[本地修改] ➔ [用户验收满意并下达发版指令] ➔ [双端独立编译 static] ➔ [全量升级 10 处版本号] 
    ➔ [交叉编译 Linux 二进制] ➔ [桌面生成 4 件套] ➔ [同步开源版并脱敏 4 处公开页] ➔ [GitHub Release 发布] ➔ [8项自检核实]
```

### 步骤 0：【前置铁律】严禁未经用户验收擅自升级版本号或发版！
- 任何需求调整、功能开发或 Bug 修复完成后，**严禁 AI 助手自作主张修改版本号、交叉编译打包、打 Git Tag 或推送到 GitHub Release**！
- 必须先在本地完成代码修改，提示用户在本地检查或预览。
- **只有当用户明确验收满意，并明确下达发版指令（如“作为 X.Y.Z 版本发布”）时，方可启动发布流程！**

### 步骤 1 ~ 8：标准发布执行步骤
1. **工程版修改**：完成代码调整与验证。
2. **双端前端独立构建与静态资源重建（血泪铁律）**：
   - **工程版**：`cd ynxwxcb-platform/frontend && npm run build && cd .. && rm -rf backend/static/* && cp -r frontend/dist/* backend/static/`
   - **开源版**：在同步并脱敏后，`cd ynxwxcb-platform-open/frontend && npm run build && cd .. && rm -rf backend/static/* && cp -r frontend/dist/* backend/static/`
   - **标题核查**：必须核查两端 `backend/static/index.html` 中的 `<title>` 均已正确生成为新版本，**绝不能只 build 一端导致自用包打入陈旧静态资源**！
3. **升级版本号**：按照“版本号规范”全局字节级同步替换 10 处版本号。
4. **交叉编译**：
   `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ynxwxcb-server ./cmd/server`
5. **桌面打包（标准 4 件套，文件必须在包根目录、不套层）**：
   - **自用部署包**（桌面 `ynxwxcb-full-linux-vX.Y.Z.tar.gz` 和 `.zip`）：包含工程版 `ynxwxcb-server` + 最新 `static/` + 强随机 `config.json` + `data/uploads/` + 完整 `deploy/` + `部署说明.txt`。
   - **开源发布包**（桌面 `ynxwxcb-vX.Y.Z.tar.gz` 和 `.zip`）：包含开源版 `ynxwxcb-server` + 脱敏 `static/` + `config.json.example` + `backup.sh` + `ynxwxcb.service` + `ynxwxcb.conf` + `README.md`。
   - **包内标题抽检**：打包后必须通过脚本解压抽检包内 `static/index.html` 的 `<title>` 确保 100% 正确！
6. **服务器更新指引**：生成可复制命令供用户覆盖 `ynxwxcb-server` 与 `static/`，提醒绝对不覆盖 `data/` 和 `config.json`。
7. **同步开源版与 4 大公开页脱敏验证**：
   - 将工程版改动同步到开源版（排除 `config.json`、`release/` 及临时脚本）。
   - **对 `Login.vue`、`MeetingRegister.vue`、`SolicitFeedback.vue`、`DispatchReceipt.vue` 4 个页面的真实备案号替换为占位符**。
   - 全量 `grep -rn "新ICP备"` 严格确认匹配数为 0！
8. **GitHub 自动化发布与 Release 资产挂载**：
   - 提交 commit（规范单行标题，使用 noreply 邮箱）；
   - 打本地 Git Tag 并推送到 GitHub `master` 与新 tag（**绝不删改历史 tag**）；
   - 调原生 GitHub API 创建 Release（`draft=false, prerelease=false`）；
   - 使用 `--data-binary` 原始流通过 SOCKS5 代理上传 `ynxwxcb-vX.Y.Z.tar.gz` 与 `.zip` 两个资产。

### 发布后核实清单（8 项严格必测）
在开源版目录执行，确认 100% 通过后方可交付：
1. `git rev-parse HEAD` vs `git ls-remote origin refs/heads/master` —— 本地远端一致。
2. `git status --short` —— 工作区完全干净（0 变更）。
3. `git branch -a` —— 仅 master 分支，无残留分支。
4. `git tag` vs `git ls-remote --tags origin` —— 本地远端 tag 完全一致。
5. GitHub API 检查对应 Release：`draft=false`，`assets` 数量严格等于 2。
6. `git fsck --unreachable` 与 `--dangling` —— 均为 0（遇孤儿对象执行 `git gc --prune=now`）。
7. 工程版与开源版源码 diff —— 仅公开页备案号存在脱敏差异。
8. 本地 tag 拓扑检查 —— tag 均在 master 历史中（`git merge-base --is-ancestor <tag> master`）。

---

## 七、工程高精尖军规与避坑宝典

### 板块 1：版本控制与发布防呆
1. **全量 10 处版本号地毯式核查**：升级版本号必须覆盖 `frontend/index.html` 的 `<title>`、`router/index.js` 的 `document.title` 后缀、`Login.vue`、`MeetingRegister.vue`、`AttendanceProfileDrawer.vue`、6 个打印模板页脚及 `README.md`。打包前必须执行 `grep` 验证旧版本号匹配数为 0！
2. **README 更新日志严禁“概括版”**：每个修复单独一行，写清“现象 → 修复”，分类归档，严禁用“修复大量缺陷”一句话概括。
3. **桌面产物双格式 4 件套齐全**：自用包与开源包均必须同时生成 `.tar.gz` 和 `.zip`，严禁偷懒只打一个格式。

### 板块 2：双版本脱敏红线（4大公开页面）
4. **公开页面新功能不可丢，备案号必须占位**：开源版必须拥有最新前端代码，`Login.vue`、`MeetingRegister.vue`、`SolicitFeedback.vue`、`DispatchReceipt.vue` 4 个页面的真实备案号替换为占位符。严禁整文件排除。
5. **工程版绝不 push GitHub**：工程版没有 `.git`，只作为本地开发目录；所有向 GitHub 的操作只能在开源版目录执行。

### 板块 3：Git 与 GitHub 自动化规范
6. **Release 资产上传参数避坑**：必须使用 `Authorization: token`（非 Bearer）、路径 `/releases/$ID/assets?name=`、body 用 `--data-binary`（严禁 `-F` multipart，否则报 400/422）。
7. **严禁删改历史 Tag**：改动或删除历史 tag 会导致 GitHub Release 变为悬空草稿。发布只新增新 tag。
8. **git fsck 指针报错修复**：若遇到 `refs/remotes/origin/HEAD invalid sha1 pointer`，执行 `git remote set-head origin master` 即可彻底消除。
9. **提交前严防大文件混入**：同步开源版时必须严格排除二进制产物和临时日志，保持 git 提交体积轻量，杜绝代理推送超时。

### 板块 4：后端高性能与高可用军规（P0/P1）
10. **SQLite WAL 连接池解限（P0）**：必须设置 `DB.SetMaxOpenConns(20)` 与 `DB.SetMaxIdleConns(5)`，严禁锁死为 1。
11. **查询必须无脑 `defer rows.Close()`（P0）**：在 `rows, err := db.Query(...)` 检查错误后的下一行必须立即声明 defer，严禁在循环内手工关闭，杜绝提前 return 导致池枯竭死锁。
12. **切片截取边界防御（P0）**：日期等截取（如 `[:4]`）前必须进行长度防御判断，严禁裸切片引发运行时 Panic 崩溃。
13. **大表导出锁死 `LIMIT 10000`（P1）**：所有 Excel 导出底层 SQL 必须强制锁死数量上限，杜绝全表无限制加载击穿单机内存（OOM）。
14. **业务列表必须配真分页（P1）**：凡是增长性业务列表，后端必须注入 `LIMIT ? OFFSET ?` 并返回 `total`，前端必须绑定 `<el-pagination>`。
15. **接口错误必须返回标准 JSON**：必须使用 `middleware.JSON` 返回 `{"error":"中文提示"}`，严禁使用返回纯文本的 `http.Error`。
16. **SQLite 可空列防 Scan 零值错位**：可空字段必须使用 `sql.NullString` 中转赋值，且 `rows.Scan` 错误必须处理，不可忽略。
17. **本地时间时区规范**：SQL 中时间必须使用 `datetime('now','localtime')`，避免使用 UTC 的 `CURRENT_TIMESTAMP` 导致少 8 小时。

### 板块 5：国家保密合规与涉密安全红线（P0·新）
18. **非涉密系统绝对禁涉密原则（P0）**：本平台为非涉密政务协同系统，全站所有上传入口必须加粗高亮禁止上传国家秘密（秘密/机密/绝密）、工作秘密及内部级文件资料。
19. **文件上传全链路四维涉密质检（P0）**：所有后台及公开上传接口，必须强制挂载 `CheckFileSecrecy` 校验，覆盖文件名、Word 正文 XML、PDF 文本流与字库、图片及扫描件 OCR。
20. **B/S 架构服务端解耦原则（P0）**：OCR 与公文解析全量封闭在服务端执行，绝对不得对客户端（Windows/手机）提出任何软件或环境安装要求。
21. **保密违规即时熔断与存证闭环（P0）**：触发涉密上传时，立即返回 403 专属阻断码，前端弹出 `SecrecyAlert.vue` 全屏警报并锁定 30 秒登出；同时强制向 `operation_logs` 表落盘审计记录。

### 板块 6：前端健壮性与现代体验准则
22. **退出登录 0ms 同步清空凭据**：登出必须先立即同步清除本地持久化 token 并 `replace('/login')`，后端注销请求转为后台静默异步通知，彻底消除慢网下的路由守卫竞态拦截。
23. **Vite 按需引入与 350KB 体积红线**：禁止全量 `app.use(ElementPlus)`，由 unplugin 自动按需引入，首屏核心 JS 严格控制在 ~350KB。
24. **表单必须绑定响应式 `:rules`**：申请与登记表单严禁仅依赖按钮处的简单 if 拦截，必须走统一校验规则阻断。
25. **严禁空 catch 吞没异常**：所有 try-catch 块至少包含 `console.error(e)`，并在关键异步操作中弹窗提醒，严禁静默生吞排查无门。
26. **输入框 16px 锁定防 Safari 放大**：移动端表单输入控件必须锁定 16px 字号，杜绝 iOS 点击输入框页面缩放漂移。
27. **防泄密水印自愈防护**：Canvas 顶层水印配合 `MutationObserver` 监听 DOM 树，被 F12 恶意篡改或删除时 0ms 瞬间自愈。
28. **告警弹窗流式自适应与单手操作**：所有安全拦截弹窗（WAF、保密警告）必须支持流式自适应并做好小屏滚动保护，移动端操作按钮必须为 100% 全宽拇指大按钮。

### 板块 7：环境与网络避坑
29. **国内源直连与境外代理分离**：下载国内镜像与工具链时必须直连（unset SOCKS5 代理变量），仅访问 GitHub 时启用 `socks5h://127.0.0.1:1080`。
30. **进程清理避坑**：macOS 下杀端口进程若有多 PID，需使用循环逐个 kill，避免语法错误。

---

## 八、交流约定

- **需求确认前置**：用户的所有新需求，理解后必须先用中文清晰复述并经用户确认后，方可动手编写代码。
- **纯正通俗中文**：用户为政务部门工作人员，所有工作过程、回复、讲解、命令注释一律使用通俗易懂的中文。
- **文档全量同步与日期刷新（铁律）**：
  - 每次功能变更或修复 Bug 后，必须主动排查与 `AGENTS.md`、`README.md`、`docs/` 下各类部署说明是否冲突并全量同步更新；
  - **修改或同步任何文档时，必须同步将文档顶部的更新日期（YYYY-MM-DD）与适用版本号刷新为当天最新**，严禁遗留陈旧历史日期。
