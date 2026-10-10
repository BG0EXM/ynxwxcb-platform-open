# 伊宁县委宣传部部务工作平台 - 部署文档（Caddy）

> 适用版本：**V1.7.1** · 更新日期：**2026-10-11**  
> 运行环境：Debian / Ubuntu · Caddy Web 服务器（自动 HTTPS / IPv6 双栈）· 可与 WordPress 共存

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
└── data/                 # 运行时自动生成
```

## 二、两种部署方式

### 方式 A：本地编译好后上传（推荐，最简单）

**第 1 步：在电脑上编译**

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ynxwxcb-server ./cmd/server
```

（Windows 装 Go：https://go.dev/dl ；Linux：`sudo apt install golang`；不想装 Go 可用别人编译好的文件）

**第 2 步：上传两个东西到服务器同一目录**

```bash
sudo mkdir -p /opt/ynxwxcb
# 上传 ynxwxcb-server 和 static/（用仓库 backend/static/）到 /opt/ynxwxcb/
cd /opt/ynxwxcb
sudo chmod +x ynxwxcb-server
```

**第 3 步：创建运行用户 + 配置**

```bash
sudo useradd -r -s /usr/sbin/nologin ynxwxcb
sudo chown -R ynxwxcb:ynxwxcb /opt/ynxwxcb
sudo cp config.json.example config.json
sudo nano config.json   # 改 jwt.secret 和 admin.password
```

**第 4 步：systemd 自启**

```bash
sudo cp deploy/systemd/ynxwxcb.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ynxwxcb
sudo systemctl status ynxwxcb
```

### 方式 B：服务器上直接编译（懂行的用）

```bash
# 传源码到服务器 → cd backend → 编译（同上命令）→ static 同目录 → 后续步骤同方式 A
```

---

## 三、Caddy 反代（自动 HTTPS）

在 Caddyfile（`/etc/caddy/Caddyfile`）末尾追加，替换 `bw.yourdomain.com` 为你的域名：

```caddyfile
bw.yourdomain.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080 {
        header_up X-Real-IP {remote_host}
    }
    request_body {
        max_size 50MB
    }
}
```

> 域名需先解析到服务器 IP。Caddy 自动申请/续期 Let's Encrypt 证书，无需手动配置。

```bash
sudo caddy reload
```

## 四、防火墙

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

## 五、数据灾备与恢复

系统采用**服务器级全量自动化灾备**，打包完整 SQLite 数据库及全部公文与附件材料（`uploads/`）。

### 1. 手动备份与定时调度
```bash
# 手动立即备份
sudo /opt/ynxwxcb/deploy/backup.sh

# crontab 每日凌晨 2:00 自动执行：
0 2 * * * /opt/ynxwxcb/deploy/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
```

备份包保存在独立的 `/opt/ynxwxcb-backup/`，自动保留 14 天并带有 SHA256 校验和。建议定期同步到异机或对象存储。

### 2. 灾难恢复
```bash
sudo /opt/ynxwxcb/deploy/restore.sh /opt/ynxwxcb-backup/ynxwxcb_xxx.tar.gz
```

## 六、默认账号

首次部署后：`admin` / `admin123`。

> **安全机制**：使用默认密码登录后，系统会**强制要求修改密码**，未修改前无法使用其他功能。

## 七、常见问题

**页面 404 或空白？**
- 99% 是 `static/` 文件夹没放对或没传
- 确认服务器上 `ynxwxcb-server` 和 `static/` 在**同一目录**

**525 / SSL 握手失败？**
- 确认域名 DNS 已解析到服务器公网 IP
- 关闭 CDN/边缘加速（如阿里云 ESA）直连源站，否则证书验证可能失败
- `sudo journalctl -u caddy` 查看证书申请日志

**平台服务检查：**
```bash
sudo systemctl status ynxwxcb
curl -I http://127.0.0.1:8080
```
