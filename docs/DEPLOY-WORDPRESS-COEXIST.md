# 伊宁县委宣传部部务工作平台 — WordPress 服务器共存部署指南

> 适用版本：**V1.8.0** · 更新日期：**2026-10-19**  
> 适用环境：Debian (2C/2G) · LNMP（Nginx + MySQL + PHP-FPM）· 已运行 WordPress 网站  
> 目标：在现有服务器上部署本平台，**不影响 WordPress 正常访问**（支持 IPv6）

## 〇、核心原则（先读）

1. **平台与 WordPress 完全独立**：Go 进程独立运行 + SQLite 独立文件库，**不依赖 MySQL、不依赖 PHP-FPM**
2. **只动 Nginx，加一个 server 块**：不修改 WordPress 的任何配置
3. **用子域名隔离**：建议给平台单独一个子域名（如 `bw.yourdomain.com`），与 WordPress 主域名互不干扰
4. **资源影响极小**：Go 进程约 30-60MB 内存，SQLite 无独立服务，对 WordPress 无感知

## 一、规划建议

| 项 | 建议 | 说明 |
|---|---|---|
| 平台域名 | `bw.yourdomain.com` | 与 WordPress 主域名分开，或用全新域名 |
| 后端端口 | `8080`（仅本机访问） | 不与 80/443 冲突，不外网开放 |
| 安装目录 | `/opt/ynxwxcb/` | 独立目录，不影响 WordPress 的 `/www/...` 等目录 |
| SSL 证书 | 平台子域名**单独申请** | 复用 WordPress 证书不可行（域名不同）；用 Let's Encrypt 或云厂商免费证书 |

## 二、部署步骤

### 1. 上传平台文件

```bash
sudo mkdir -p /opt/ynxwxcb
# 上传发布包内全部文件到 /opt/ynxwxcb/：
#   ynxwxcb-server（二进制）、static/（前端页面，必须与主程序同目录）、config.json.example、
#   backup.sh、restore.sh、ynxwxcb.service、ynxwxcb.conf
cd /opt/ynxwxcb
sudo chmod 755 ynxwxcb-server backup.sh restore.sh
```

### 2. 创建独立运行用户

```bash
sudo useradd -r -s /usr/sbin/nologin ynxwxcb
# 主程序、脚本、static/ 归 root（服务只读，防止被入侵后篡改程序）；只有 data/ 归运行用户可写
# （不要对整个 /opt/ynxwxcb 递归 chown，否则已有的 data/ 会变成 root 所有，服务无法写库）
sudo chown root:root /opt/ynxwxcb /opt/ynxwxcb/ynxwxcb-server /opt/ynxwxcb/backup.sh /opt/ynxwxcb/restore.sh
sudo chown -R root:root /opt/ynxwxcb/static
sudo install -d -o ynxwxcb -g ynxwxcb -m 750 /opt/ynxwxcb/data
```

### 3. 创建并修改 config.json

```bash
sudo cp /opt/ynxwxcb/config.json.example /opt/ynxwxcb/config.json
# config.json 含 JWT 密钥和管理员密码：归 root、组 ynxwxcb 只读，其他用户不可读
sudo chown root:ynxwxcb /opt/ynxwxcb/config.json
sudo chmod 640 /opt/ynxwxcb/config.json
sudo nano /opt/ynxwxcb/config.json
```
- `jwt.secret`：改为随机长字符串（`openssl rand -base64 48`）
- `admin.password`：设置管理员初始密码
- `server.host` 保持 `"127.0.0.1"`（后端只接受本机 Nginx 转发，8080 不对外）、`server.trust_proxy` 保持 `true`（信任 Nginx 传来的 `X-Real-IP` 作为客户端真实 IP）。两者需配合使用，详见 [DEPLOY.md](DEPLOY.md) 第二节

> systemd 服务启用了 `ProtectSystem=strict` 等沙箱加固，**仅 `/opt/ynxwxcb/data` 可写**，因此 `config.json` 必须事先创建好；该加固只作用于本平台进程，不影响 WordPress。

### 4. 配置 systemd 服务

```bash
sudo cp /opt/ynxwxcb/ynxwxcb.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ynxwxcb
sudo systemctl status ynxwxcb        # 确认 running
```

> 平台进程独立运行，**与 WordPress 的 PHP-FPM/MySQL 服务无关**，不会影响它们。

### 5. Nginx 新增 server 块（关键步骤）

**不要修改 WordPress 的 server 配置！** 在 `/etc/nginx/sites-available/` 新建平台专用配置：

```bash
sudo cp /opt/ynxwxcb/ynxwxcb.conf /etc/nginx/sites-available/ynxwxcb.conf
sudo nano /etc/nginx/sites-available/ynxwxcb.conf
```

修改三处：
- `server_name bw.yourdomain.com` → 你的平台域名
- `ssl_certificate` / `ssl_certificate_key` → 平台域名的证书路径
- 确认 `proxy_pass http://127.0.0.1:8080` 不变

> 配置顶部的两个 `map` 变量已带 `ynxwxcb_` 前缀，不会与 WordPress 或其他站点已定义的 `$connection_upgrade` 等变量重名冲突。HTTP/2 写法请按 `nginx -v` 版本选择（见配置文件内注释）。

启用并测试：

```bash
sudo ln -s /etc/nginx/sites-available/ynxwxcb.conf /etc/nginx/sites-enabled/
sudo nginx -t        # 语法检查，务必看到 syntax is ok
sudo systemctl reload nginx
```

> `nginx -t` 通过才说明新配置没和 WordPress 冲突。若报错，先检查 `server_name` 是否有重复。

### 6. 证书申请（平台子域名）

若用 Let's Encrypt：

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d bw.yourdomain.com
```

或从云厂商（阿里云/腾讯云）申请免费证书，下载 nginx 版后填入上面的 `ssl_certificate` 路径。

### 7. 防火墙确认

LNMP 通常已放行 80/443。确认：

```bash
sudo ufw status
# 若启用了 ufw，确保 80、443 放行；8080 不需要放行
```

## 三、共存验证清单

部署完成后逐项检查：

```bash
# 1. 平台进程
systemctl status ynxwxcb

# 2. 平台访问（浏览器）https://bw.yourdomain.com
#    - 应看到登录页（注意强刷 Ctrl+F5）

# 3. WordPress 访问（浏览器）https://yourdomain.com
#    - 应完全正常，无任何变化

# 4. 端口占用检查（确认 8080 仅本机）
ss -tlnp | grep -E ':(80|443|8080)'

# 5. 平台日志无报错
journalctl -u ynxwxcb -n 50
```

## 四、备份策略（独立于 WordPress）

平台数据备份**与 WordPress 备份分开**，互不干扰：

```bash
# 前置依赖（必装，备份脚本用它生成一致性快照并做完整性自检）
sudo apt install -y sqlite3

# 手动备份（须以 root 执行：备份目录为 700、备份包为 600，仅 root 可读）
sudo /opt/ynxwxcb/backup.sh

# 定时备份（每天 2:00，仅平台数据）
sudo crontab -e
# 加入：
0 2 * * * /opt/ynxwxcb/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
```

- 平台备份文件：`/opt/ynxwxcb-backup/`（SQLite 数据库 + 上传附件，自动保留 14 天，带 SHA256 校验）
- 灾难恢复：`sudo /opt/ynxwxcb/restore.sh /opt/ynxwxcb-backup/ynxwxcb_xxx.tar.gz`
- WordPress 备份：维持你现有方案，两套互不影响

## 五、注意事项

1. **不要修改 WordPress 相关配置**：本方案只新增 server 块，WordPress 的 PHP-FPM、MySQL、上传目录均不动
2. **域名必须不同**：`server_name` 不可与 WordPress 重复，否则 Nginx 会按顺序匹配出错
3. **资源预留**：2C2G 下运行 WordPress + 本平台总体内存压力可控（本平台约 60MB），但若 WordPress 常驻内存较高，建议 `free -m` 确认空闲内存，必要时给 WordPress 优化（如安装缓存插件）
4. **升级平台**：先 `sudo /opt/ynxwxcb/backup.sh` 备份，停服务后**同时替换** `/opt/ynxwxcb/ynxwxcb-server` 与 `/opt/ynxwxcb/static/`（前端页面不在二进制内，只换主程序会导致新旧前后端不匹配），再启动服务；**绝不覆盖 `data/` 与 `config.json`**，不影响 WordPress。具体命令见 [DEPLOY.md](DEPLOY.md) 第七节
5. **回滚**：`systemctl disable --now ynxwxcb` + 删除 `/etc/nginx/sites-enabled/ynxwxcb.conf` 并 reload，即可完全移除平台，WordPress 不受影响

## 六、默认账号

- 管理员：`admin` / config.json 中 `admin.password` 设置的密码（登录后立即修改）
