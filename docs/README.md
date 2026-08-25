# 伊宁县委宣传部部务工作平台

面向县级党委宣传部的内部部务工作平台，包含收文管理（呈批单/传阅登记卡/标签打印）、考勤点到（含迟到）、请假管理、公车报备、公共资料、通讯录、值守排班、工作日历、大事记、每周工作总结、常委管理与用户权限。

## 技术栈

- 后端：Go 1.25（标准库 HTTP）+ SQLite（modernc.org/sqlite，纯 Go，无 CGO）
- 前端：Vue 3 + Element Plus + Pinia + Vue Router + Vite
- 部署：单二进制 + 静态资源，Nginx 反代 HTTPS，systemd 托管

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
| 考勤点到 | 管理员晨会手工点到、月度/年度统计与打印 |
| 请假管理 | 年假及各类假期登记、统计与 Excel 导出 |
| 公车管理 | 车辆基础信息、用车报备、派车单打印、Excel 导出 |
| 公共资料 | 共享资料发布与阅读 |
| 通讯录 | 按部门/姓名查询 |
| 值守排班 | 日历视图当天值守（至21:00收文）、可标记县委大院排班 |
| 工作日历 | 全屏大日历（按月/按年），各科室在格子中添加要做的工作，跨天连续显示，列表展示，Excel 导出（可单科室/全部） |
| 大事记 | 各科室每月重大事项，按年汇总整个宣传部导出 Word（仅管理员，按月公文格式） |
| 每周工作总结 | 各科室录入本周重点工作，管理员可增删改全部，汇总导出 Word（仅管理员） |
| 常委管理 | 常委大事记，按月分组展示，Word 导出（仅管理员） |
| 系统管理 | 用户、角色、部门管理 |

> 各模块台账（用车报备/请假/考勤/排班/收文/工作日历）均支持 Excel 导出；大事记/每周总结/常委大事记支持 Word 导出。

## 角色与权限

- 管理员（admin）：全部功能 + 用户管理 + 常委管理，**各模块 Word 导出**
- 科室工作人员（staff）：业务办理（工作日历/大事记/每周总结只能操作本科室）
- 乡镇/通讯员（reporter）：投稿、接收通知、上报材料

> 收文的新增/编辑/删除仅限"办公室"部门用户，其他部门只读；大事记/每周总结各科室自己录自己科室，管理员可增删改全部；常委大事记仅管理员操作；各类 Word 导出仅管理员。

## 部署

见 [DEPLOY.md](DEPLOY.md)。核心要点：
- 单二进制部署到 `/opt/ynxwxcb`
- systemd 托管自动重启
- Nginx 反代 HTTPS
- crontab 每日备份
