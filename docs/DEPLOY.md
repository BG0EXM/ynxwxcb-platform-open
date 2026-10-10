# 伊宁县委宣传部部务工作平台 - 部署文档（Nginx）

> 适用版本：**V1.7.1** · 更新日期：**2026-10-11**  
> 服务器配置：Debian / Ubuntu (2C / 2G) · HTTPS 公网访问（支持 IPv6）

## 〇、系统组成与服务端可选增强组件

### 1. 核心运行组件（必选）
部署到服务器需要**两样核心东西**：

| 名称 | 说明 |
|---|---|
| `ynxwxcb-server` | 主程序（可执行文件） |
| `static/` 文件夹 | 前端网页页面 |

> **`ynxwxcb-server` 和 `static/` 必须放在同一个目录**（后端在运行目录下找 static）。
> 本仓库的 `backend/static/` 已帮你准备好前端页面，直接用即可。

### 2. 国家保密审查环境依赖（强烈推荐）
系统全链路集成四维国家涉密安全审查引擎。为使服务器具备**公文扫描件图片 OCR 识别**以及**纯扫描式 PDF 视觉深度审查**能力，推荐在 Linux 服务器上一键安装 Tesseract OCR 和 Poppler 工具库：

```bash
# Debian / Ubuntu 环境一键安装
sudo apt update
sudo apt install -y tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-chi-tra poppler-utils
```
- **`tesseract-ocr` + `tesseract-ocr-chi-sim` + `tesseract-ocr-chi-tra`**：提供底层离线 OCR 光学字符识别引擎，支持简体中文与繁体中文双向词库审查；
- **`poppler-utils`（内置 `pdftoppm` 工具）**：负责将纯图像/扫描式 PDF 公文各页面高保真光栅化渲染为图像送入 OCR，彻底杜绝扫描公文规避检测；
- *提示*：若未安装上述组件，系统启动时会在日志中清晰提示，并平滑降级为 Word XML 解构与 PDF 原生文本流检测（不影响基础部务业务正常运行，但无法识别纯图片扫描件与扫描式 PDF 内部的密级印记）。

## 一、部署后的目录结构

```
/opt/ynxwxcb/
├── ynxwxcb-server          # 主程序（可执行文件）
├── static/               # 前端页面文件夹（与主程序同级）
├── config.json           # 配置文件
├── backup.sh             # 备份脚本
└── data/                 # 运行时自动生成（数据库/上传文件）
```

## 二、两种部署方式

### 方式 A：本地编译好后上传（推荐，最简单，服务器不用装环境）

> 适合新手：在你自己电脑上编译一次，然后把两个东西传到服务器。

**第 1 步：在电脑上编译**

打开终端，进入项目 `backend` 目录：

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ynxwxcb-server ./cmd/server
```

- Windows 装 Go：https://go.dev/dl
- Linux 装 Go：`sudo apt install golang`
- 实在不想装 Go，可以让已编译好的人给你一份 `ynxwxcb-server` 文件

**第 2 步：上传到服务器**

把这两样放进同一目录（如 `/opt/ynxwxcb/`）：
- `ynxwxcb-server`（编译出的文件）
- `static/`（用仓库 `backend/static/` 里的，整个文件夹复制）

```bash
# 在服务器上
sudo mkdir -p /opt/ynxwxcb
# 用 SFTP/SCP 上传 ynxwxcb-server 和 static/ 到 /opt/ynxwxcb/
cd /opt/ynxwxcb
sudo chmod +x ynxwxcb-server
```

**第 3 步：创建运行用户（安全加固，可跳过）**

```bash
sudo useradd -r -s /usr/sbin/nologin ynxwxcb
sudo chown -R ynxwxcb:ynxwxcb /opt/ynxwxcb
```

**第 4 步：配置 config.json**

```bash
sudo cp /opt/ynxwxcb/config.json.example /opt/ynxwxcb/config.json
sudo nano /opt/ynxwxcb/config.json
```

修改：
- `jwt.secret`：改为随机长字符串（可用 `openssl rand -base64 48` 生成）
- `admin.username` / `admin.password`：设置管理员账号。**只要 `admin.password` 非空，程序每次启动都会用 config 中的用户名和密码覆盖管理员账号**，改完重启即生效（默认 `admin` / `admin123`）

> `data/` 目录无需手动创建，systemd 服务会在启动前自动创建并设置权限。

### 方式 B：在服务器上直接编译（懂行的用，服务器要装 Go）

```bash
# 1. 把整个项目源码传到服务器
# 2. 编译
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ynxwxcb-server ./cmd/server
# 3. 确保 static/ 和 ynxwxcb-server 同目录
# 4. 后续步骤同方式 A 的第 3、4 步
```

---

## 三、配置 systemd 服务（开机自启）

```bash
sudo cp /opt/ynxwxcb/deploy/systemd/ynxwxcb.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ynxwxcb
sudo systemctl status ynxwxcb
```

> 如果目录不是 `/opt/ynxwxcb`，需修改 `ynxwxcb.service` 里的路径。

## 四、反向代理 + HTTPS（Nginx）

1. 把 `deploy/nginx/ynxwxcb.conf` 复制到服务器，修改域名和证书路径
2. 启用：

```bash
sudo cp /opt/ynxwxcb/deploy/nginx/ynxwxcb.conf /etc/nginx/sites-available/
sudo ln -s /etc/nginx/sites-available/ynxwxcb.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

> 证书可用阿里云/腾讯云 SSL，或 Let's Encrypt（`certbot`）。

## 五、防火墙

```bash
sudo ufw allow 443/tcp
sudo ufw allow 80/tcp   # 用于证书续期
sudo ufw enable
```

> 后端 8080 端口无需对外开放（Nginx 内网反代）。

## 六、数据灾备与恢复

系统采用**服务器级全量自动化灾备**，打包完整 SQLite 数据库及全部公文与附件材料（`uploads/`）。

### 1. 手动立即执行备份
```bash
sudo /opt/ynxwxcb/deploy/backup.sh
```

### 2. 配置定时自动化备份（每天凌晨 2:00，保留 14 天）
```bash
sudo crontab -e
# 在末尾添加以下一行：
0 2 * * * /opt/ynxwxcb/deploy/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
```

> 备份包存储在独立的 `/opt/ynxwxcb-backup/`，自动保留最近 14 天并带有 SHA256 校验和。**强烈建议定期将此目录同步到离线介质或异地存储。**

### 3. 数据灾难恢复（一键恢复或手动恢复）

**方式一：一键脚本自动恢复（推荐）**
```bash
sudo /opt/ynxwxcb/deploy/restore.sh /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz
```
- 脚本会自动停止服务、为当前数据创建安全兜底副本、写入数据库和附件、修复权限并自动重启服务。

**方式二：手动分步恢复**
```bash
# 1. 停止服务
sudo systemctl stop ynxwxcb

# 2. 解包到临时目录
mkdir -p /tmp/restore && tar -xzf /opt/ynxwxcb-backup/ynxwxcb_xxx.tar.gz -C /tmp/restore

# 3. 替换数据库与附件
sudo cp /tmp/restore/ynxwxcb.db /opt/ynxwxcb/data/ynxwxcb.db
sudo cp -r /tmp/restore/uploads /opt/ynxwxcb/data/
sudo chown -R ynxwxcb:ynxwxcb /opt/ynxwxcb/data

# 4. 重启服务
sudo systemctl start ynxwxcb
```

## 七、日常维护

```bash
# 查看日志
sudo journalctl -u ynxwxcb -f

# 升级版本（只替换主程序，static 不用动）
sudo systemctl stop ynxwxcb
# 上传新 ynxwxcb-server 覆盖 /opt/ynxwxcb/ynxwxcb-server
sudo chmod +x /opt/ynxwxcb/ynxwxcb-server
sudo systemctl start ynxwxcb

# 磁盘检查
df -h /opt/ynxwxcb
du -sh /opt/ynxwxcb/data/uploads
```

## 八、默认账号

首次部署后登录（**登录后立即修改密码**）：
- 管理员：`admin` / `admin123`

> **安全机制**：使用默认密码 `admin123` 或初始密码 `123456` 登录后，系统会**强制要求修改密码**，未修改前无法使用其他功能。

## 九、常见问题

**页面 404 或空白？**
- 99% 是 `static/` 文件夹没放对或没传
- 确认服务器上 `ynxwxcb-server` 和 `static/` 在**同一目录**

**升级后页面没变？**
- 浏览器强刷（Ctrl+F5）

## 十、注意事项

1. 平台使用 SQLite（WAL 模式），勿在 NFS 等网络盘上运行数据库
2. 上传附件默认限制 50MB，按需调整
3. 2C2G 服务器已通过 systemd 内存限制（1G）防止 OOM

## 十一、无互联网（内网/隔离网）部署

平台运行时不依赖任何在线资源。在联网机器上完成编译后，把 `ynxwxcb-server` + `static/` 传到内网即可。详见 [OFFLINE-DEPLOYMENT.md](OFFLINE-DEPLOYMENT.md)。
