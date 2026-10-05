# 伊宁县委宣传部部务工作平台

> 版本：**V1.6.0** · 更新日期：**2026-10-05**

面向县级党委宣传部的内部部务工作平台，包含收文管理（呈批单/传阅登记卡/热敏标签打印）、考勤点到（含迟到/培训）、请假管理（含按小时/半天、假条打印与审批流联动）、公车报备、会务管理（公开匿名报名/签到单导出）、公共资料、通讯录、值守排班、工作日历、大事记、每周工作总结、加班管理（工时折算与补休联动）、年休假管理（配置与已休联动）、常委管理、用户管理、角色-功能权限矩阵与操作日志审计。

## 技术栈

- 后端：Go 1.26（标准库 HTTP）+ SQLite（modernc.org/sqlite，纯 Go，无 CGO，WAL 模式）
- 前端：Vue 3 + Element Plus 2.9 + Pinia + Vue Router 4 + Vite 6
- 部署：单二进制 + 静态资源，Nginx / Caddy 反代 HTTPS，systemd 托管，支持 IPv6

## 项目结构

```
ynxwxcb-platform/
├── backend/                 # Go 后端
│   ├── cmd/server/          # 入口
│   ├── internal/
│   │   ├── auth/            # JWT 认证
│   │   ├── config/          # 配置加载
│   │   ├── database/        # SQLite 建表与初始化
│   │   ├── handlers/        # 业务接口
│   │   ├── middleware/      # 认证/权限中间件
│   │   ├── models/          # 数据模型
│   │   └── router/          # 路由
│   └── static/              # 前端构建产物（SPA）
├── frontend/                # Vue 前端源码
│   └── src/
│       ├── views/           # 各业务页面
│       ├── router/          # 前端路由
│       ├── store/           # Pinia 状态
│       └── utils/           # axios 封装
└── deploy/                  # 部署文件
    ├── backup.sh            # 备份脚本
    ├── release.sh           # 发布包生成脚本
    ├── config.json.example  # 配置模板
    ├── nginx/ynxwxcb.conf     # Nginx HTTPS 配置
    └── systemd/ynxwxcb.service
```

> 全部文档集中在 `docs/` 目录：[README](README.md)（本项目）· [DEPLOY](DEPLOY.md)（部署·Nginx）· [DEPLOY-CADDY](DEPLOY-CADDY.md)（部署·Caddy）· [DEPLOY-WORDPRESS-COEXIST](DEPLOY-WORDPRESS-COEXIST.md)（WordPress 共存）· [OFFLINE-DEPLOYMENT](OFFLINE-DEPLOYMENT.md)（离线部署）· [ARCHITECTURE](ARCHITECTURE.md)（架构梳理）

## 本地开发

### 后端
```bash
cd backend
go run ./cmd/server -config config.json
# 默认监听 :8080，首次运行自动生成 config.json 和数据库
```

### 前端
```bash
cd frontend
npm install
npm run dev   # 开发服务器 :5173，代理 /api 到 :8080
npm run build # 构建到 dist/
```

构建后需将 `frontend/dist/*` 复制到 `backend/static/`。

### 交叉编译（Linux amd64）
在 Windows/Linux 上均可：
```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ynxwxcb-server ./cmd/server
```

## 功能模块

| 模块 | 说明 |
|------|------|
| 收文管理 | 上级来文登记、呈批单/传阅登记卡/热敏标签生成与打印、文件退回管理 |
| 考勤点到 | 管理员晨会手工点到、月度/年度统计与打印（支持迟到/培训/补休联动） |
| 请假管理 | 年假及各类假期登记、支持按小时/半天、假条打印（自动匹配审批流）、Excel 导出 |
| 公车管理 | 车辆基础信息、用车报备、派车单打印、Excel 导出 |
| 会务管理 | 会议录入、公开匿名报名链接与移动端回执、参会登记（单人/多人）、签到单导出 |
| 公共资料 | 共享资料发布与阅读、附件安全下载 |
| 通讯录 | 按部门/姓名查询 |
| 值守排班 | 日历视图当天值守（至21:00收文）、可标记县委大院排班 |
| 工作日历 | 全屏大日历（按月/按年），各科室在格子中添加要做的工作，跨天连续显示，列表展示，Excel 导出（可单科室/全部） |
| 大事记 | 各科室每月重大事项，按年汇总整个宣传部导出 Word（按月公文格式） |
| 每周工作总结 | 各科室录入本周重点工作，管理员可增删改全部，汇总导出 Word |
| 加班管理 | 加班工时录入、加班统计与补休管理（8 小时=1 天折算），Excel 导出 |
| 年休假管理 | 管理员配置每人每年年休假天数，已休天数与请假系统自动联动核销 |
| 常委管理 | 常委大事记，按月分组展示，Word 导出（仅管理员） |
| 系统管理 | 用户、角色、部门管理 |
| 权限管理 | 角色-功能权限矩阵，46 个权限点按角色勾选（仅管理员） |
| 操作日志 | 关键写操作、登录、导出全程审计留痕，按模块/动作/日期/人员查询（仅管理员） |

> 各模块台账（用车报备/请假/考勤/排班/收文/工作日历/加班/年休假）均支持 Excel 导出；大事记/每周总结/常委大事记支持 Word 导出。

## 角色与权限

- 采用 **角色-功能权限矩阵（V1.4.2+）**：系统预设管理员（`admin`）、分管领导（`leader`）、科室人员（`staff`）、通讯员（`reporter`），共 46 项功能权限点由管理员灵活勾选控制。
- `admin` 拥有全量功能权限（代码旁路保障，不可取消）。
- 数据范围按科室/本人物理隔离（如工作日历/大事记/每周总结仅限本科室，请假/加班仅限本人）。

## 部署

见 [DEPLOY.md](DEPLOY.md)。核心要点：
- 单二进制部署到 `/opt/ynxwxcb`
- systemd 托管自动重启
- Nginx 反代 HTTPS
- crontab 每日备份
