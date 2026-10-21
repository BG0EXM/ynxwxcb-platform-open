# 伊宁县委宣传部部务工作平台 - 部署文档（Nginx）

> 适用版本：**V1.8.1** · 更新日期：**2026-10-21**  
> 服务器配置：Debian / Ubuntu (2C / 2G) · HTTPS 公网访问（支持 IPv6）

## 升级到 V1.8.1 必读
- 数据库自动升级到 v21（新增库房物资 4 张表），升级前务必先执行 `deploy/backup.sh` 备份。
- 新增权限点“库房物资管理”（`warehouse.manage`，共 49 项），**默认不授予任何角色**，升级后由管理员在“权限管理”中给办公室经办人勾选（管理员本身可直接使用）。
- 新增免登录公开页 `/warehouse/<令牌>`（库房门口扫码领用），nginx / Caddy 反代配置无需改动；二维码在“库房物资 → 门口二维码”中打印。
- 严格按下文“升级版本”步骤：先停服务，再一起替换 `ynxwxcb-server` 与 `static/`，最后启动；绝不覆盖 `data/` 与 `config.json`。

## 升级到 V1.8.0 必读
- 数据库自动升级到 v20，升级前务必先执行 `deploy/backup.sh` 备份。
- 会务、征求意见、材料下发的公开链接改为随机令牌，**升级前发出的旧链接全部失效**，正在进行的事项需要在后台重新“复制链接”或“一键通知”。
- 乡镇通讯员（reporter）角色默认收回“征求意见管理 / 材料下发管理”权限，如确有需要，可在权限管理中手动开启。
- `config.json` 的 `admin.password` 改为只在管理员账号不存在时生效，不再每次启动覆盖界面上改过的密码；找回密码需临时设置 `admin.force_reset: true`。
- 前置 nginx/Caddy 时，请设置 `server.trust_proxy: true`、`server.host: "127.0.0.1"`，并更新 nginx 配置中的 `X-Forwarded-For`。
- 只要服务器上的 `config.json` 里仍有 `admin.password`，升级后第一次启动不会再改动管理员密码，这是预期行为。

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
├── config.json           # 配置文件（由 config.json.example 复制修改，root:ynxwxcb 640）
├── config.json.example   # 配置模板
├── backup.sh             # 备份脚本（也可放在 deploy/backup.sh，脚本自动识别应用目录）
├── restore.sh            # 恢复脚本（同上）
└── data/                 # 运行时自动生成（数据库/上传文件），唯一允许服务写入的目录
```

> 使用 `deploy/release.sh` 生成的发布包已包含以上除 `config.json`、`data/` 外的全部文件（含 `static/`），解压到 `/opt/ynxwxcb/` 即可；发布包**不含**真实 `config.json`。

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
- `config.json.example`（配置模板，第 4 步要用）

```bash
# 在服务器上
sudo mkdir -p /opt/ynxwxcb
# 用 SFTP/SCP 上传 ynxwxcb-server 和 static/ 到 /opt/ynxwxcb/
cd /opt/ynxwxcb
sudo chmod +x ynxwxcb-server
```

**第 3 步：创建运行用户并收紧文件归属（安全加固，强烈建议）**

```bash
sudo useradd -r -s /usr/sbin/nologin ynxwxcb
# 主程序、static/ 归 root，服务进程只能读不能改（防止被入侵后篡改程序）
# （不要对整个 /opt/ynxwxcb 递归 chown，否则已有的 data/ 会变成 root 所有，服务无法写库）
sudo chown root:root /opt/ynxwxcb /opt/ynxwxcb/ynxwxcb-server
sudo chown -R root:root /opt/ynxwxcb/static
sudo chmod 755 /opt/ynxwxcb /opt/ynxwxcb/ynxwxcb-server
# 只有 data/ 归运行用户，可写
sudo install -d -o ynxwxcb -g ynxwxcb -m 750 /opt/ynxwxcb/data
```

**第 4 步：配置 config.json**

```bash
sudo cp /opt/ynxwxcb/config.json.example /opt/ynxwxcb/config.json
# config.json 含 JWT 密钥和管理员密码：归 root、组 ynxwxcb 只读，其他用户不可读
sudo chown root:ynxwxcb /opt/ynxwxcb/config.json
sudo chmod 640 /opt/ynxwxcb/config.json
sudo nano /opt/ynxwxcb/config.json
```

修改：
- `jwt.secret`：改为随机长字符串（可用 `openssl rand -base64 48` 生成）
- `admin.username` / `admin.password`：设置管理员账号。**只要 `admin.password` 非空，程序每次启动都会用 config 中的用户名和密码覆盖管理员账号**，改完重启即生效（默认 `admin` / `admin123`）

`server` 段两个与反向代理配套的配置项（模板中已按“前置 Nginx/Caddy”填好，一般无需改动）：

| 配置项 | 模板值 | 含义 |
|---|---|---|
| `server.host` | `"127.0.0.1"` | 后端监听地址。`127.0.0.1` 表示只接受本机（Nginx/Caddy）转发的请求，外部无法绕过反代直连 8080。**只有不经反代、直接用 IP:8080 访问时**才改为 `"0.0.0.0"` |
| `server.trust_proxy` | `true` | 是否信任反代传来的客户端 IP（优先取 `X-Real-IP`，其次取 `X-Forwarded-For` 最右一段），用于登录限流和操作日志记录真实 IP。**必须与 `host: 127.0.0.1` 配合使用**；若后端直接暴露在公网，请改为 `false`，否则客户端可伪造 IP 绕过限流 |

> `data/` 目录无需手动创建，systemd 服务会在启动前自动创建并设置归属（`ynxwxcb:ynxwxcb`，权限 750）。

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

> 如果目录不是 `/opt/ynxwxcb`，需修改 `ynxwxcb.service` 里的路径。使用发布包部署时，服务文件在包根目录：`sudo cp /opt/ynxwxcb/ynxwxcb.service /etc/systemd/system/`。

> **systemd 沙箱说明**：服务启用了 `ProtectSystem=strict`，整个文件系统对服务进程只读，**仅 `/opt/ynxwxcb/data` 可写**；同时启用 `ProtectHome`、`PrivateDevices`、`PrivateTmp`、`NoNewPrivileges`、`RestrictAddressFamilies` 等加固项，新建文件权限 `UMask=0027`。因此：
> - `config.json` 必须事先创建好（服务无权在只读目录下自动生成默认配置）；
> - 若 `config.json` 中 `database.path` / `upload.dir` 改到了 `data/` 以外的位置，需同步修改服务文件中的 `ReadWritePaths=`。

## 四、反向代理 + HTTPS（Nginx）

1. 把 `deploy/nginx/ynxwxcb.conf` 复制到服务器，修改域名和证书路径
   - 该文件顶部含两个 `map` 块，必须放在被 `http {}` 包含的目录（Debian/Ubuntu 默认的 `sites-available/` 即可）
   - HTTP/2 写法按 `nginx -v` 版本二选一：低于 1.25.1（Debian 12 / Ubuntu 24.04 及更早自带版本）用默认的 `listen 443 ssl http2;`；1.25.1 及以上可改为 `listen 443 ssl;` + `http2 on;`
   - 已启用 HSTS（1 年）、`index.html` 及接口 `no-cache`、`/assets/` 长缓存；`X-Real-IP` / `X-Forwarded-For` 均只传 `$remote_addr`，防止伪造 IP
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

### 0. 前置依赖：sqlite3（必装）
```bash
sudo apt install -y sqlite3
```
> 备份脚本强制使用 `sqlite3 .backup` 生成**单文件一致性快照**并执行 `PRAGMA integrity_check` 自检；未安装 sqlite3 时脚本会直接报错退出（不会再裸拷正在运行的数据库文件，避免得到不一致的备份）。

### 1. 手动立即执行备份
```bash
sudo /opt/ynxwxcb/deploy/backup.sh
# 若脚本放在应用根目录（发布包默认位置）：sudo /opt/ynxwxcb/backup.sh
```

### 2. 配置定时自动化备份（每天凌晨 2:00，保留 14 天）
```bash
sudo crontab -e
# 在末尾添加以下一行：
0 2 * * * /opt/ynxwxcb/deploy/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
```

> 备份包存储在独立的 `/opt/ynxwxcb-backup/`（目录权限 700、备份包 600，仅 root 可读——包内含用户密码哈希等敏感数据），自动保留最近 14 天并带有 SHA256 校验和。打包时附件直接从 `data/uploads/` 读取，不占用 `/tmp`；执行前按“数据库 + 附件”实际体积预检磁盘空间。**强烈建议定期将此目录同步到离线介质或异地存储。**

### 3. 数据灾难恢复（一键恢复或手动恢复）

**方式一：一键脚本自动恢复（推荐）**
```bash
sudo /opt/ynxwxcb/deploy/restore.sh /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz
```
- 脚本会先校验 SHA256、解包并自检数据库，确认无误后再停止服务、为当前数据创建安全兜底副本（`data.pre_restore_bak_*`）、写入数据库和附件、修复权限（目录 750 / 文件 640）并自动重启服务。
- 停服后任一步骤失败，脚本会自动从兜底副本回滚 `data/` 并重新拉起服务，不会让业务停在半恢复状态。
- 系统无 `sha256sum`/`shasum` 或缺少 `.sha256` 文件时只会提示“未校验”，不会误报“校验通过”。

**方式二：手动分步恢复**
```bash
# 1. 停止服务
sudo systemctl stop ynxwxcb

# 2. 解包到临时目录
mkdir -p /tmp/restore && tar -xzf /opt/ynxwxcb-backup/ynxwxcb_xxx.tar.gz -C /tmp/restore

# 3. 替换数据库与附件（必须先删除旧的 -wal/-shm，否则旧日志会与新库合流损坏数据）
sudo rm -f /opt/ynxwxcb/data/ynxwxcb.db /opt/ynxwxcb/data/ynxwxcb.db-wal /opt/ynxwxcb/data/ynxwxcb.db-shm
sudo cp /tmp/restore/ynxwxcb.db /opt/ynxwxcb/data/ynxwxcb.db
# 旧版脚本生成的备份包内若含 ynxwxcb.db-wal，需一并复制（-shm 不要复制）
[ -f /tmp/restore/ynxwxcb.db-wal ] && sudo cp /tmp/restore/ynxwxcb.db-wal /opt/ynxwxcb/data/
sudo rm -rf /opt/ynxwxcb/data/uploads && sudo cp -r /tmp/restore/uploads /opt/ynxwxcb/data/
sudo chown -R ynxwxcb:ynxwxcb /opt/ynxwxcb/data
sudo find /opt/ynxwxcb/data -type d -exec chmod 750 {} +
sudo find /opt/ynxwxcb/data -type f -exec chmod 640 {} +

# 4. 重启服务
sudo systemctl start ynxwxcb
```

## 七、日常维护

```bash
# 查看日志
sudo journalctl -u ynxwxcb -f

# 升级版本（主程序与 static/ 必须一起替换；绝不覆盖 data/ 和 config.json）
# 前端页面不在主程序内，后端运行时从 /opt/ynxwxcb/static/ 读取，只换主程序会导致新旧前后端不匹配
sudo /opt/ynxwxcb/deploy/backup.sh          # 1. 升级前先备份（脚本在根目录则用 /opt/ynxwxcb/backup.sh）
sudo systemctl stop ynxwxcb                  # 2. 停服务
# 3. 上传新版 ynxwxcb-server 与 static/ 到服务器临时目录（如 /tmp/upgrade/），然后：
sudo install -o root -g root -m 755 /tmp/upgrade/ynxwxcb-server /opt/ynxwxcb/ynxwxcb-server
sudo rm -rf /opt/ynxwxcb/static && sudo cp -r /tmp/upgrade/static /opt/ynxwxcb/static
sudo chown -R root:root /opt/ynxwxcb/static
sudo systemctl start ynxwxcb                 # 4. 启动并确认
sudo systemctl status ynxwxcb

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
