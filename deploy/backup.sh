#!/bin/bash
# ==============================================================================
# 伊宁县委宣传部部务工作平台 - 生产全量灾备脚本 (数据库 + 上传附件)
# ==============================================================================
# 功能特性：
# 1. 强事务一致性：优先调用 sqlite3 在线快照备份引擎 (.backup)，防 WAL 读写脏读
# 2. 全量物理灾备：打包完整 SQLite 数据库及全部公文、征求意见盖章件附件 (uploads/)
# 3. 磁盘安全预检：执行前检查磁盘可用空间，杜绝写满磁盘造成服务宕机
# 4. 原子安全写入：采用 .tmp 缓冲打包与 sha256 完整性校验，杜绝损坏的半截备份
# 5. 自动过期轮转：默认安全保留 14 天历史快照，过期自动清理
#
# 建议配置 crontab 定时调度（每天凌晨 2:00 执行）:
#   0 2 * * * /opt/ynxwxcb/deploy/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
# ==============================================================================

set -euo pipefail

# 1. 路径自动推导与配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="${APP_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
# 若自动推导不是有效路径，默认采用标准部署路径 /opt/ynxwxcb
if [ ! -d "$APP_DIR/data" ] && [ -d "/opt/ynxwxcb/data" ]; then
    APP_DIR="/opt/ynxwxcb"
fi

DATA_DIR="$APP_DIR/data"
DB_PATH="$DATA_DIR/ynxwxcb.db"
UPLOADS_DIR="$DATA_DIR/uploads"
BACKUP_DIR="${BACKUP_DIR:-/opt/ynxwxcb-backup}"
RETENTION_DAYS=14

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="ynxwxcb_$DATE.tar.gz"
BACKUP_FINAL="$BACKUP_DIR/$BACKUP_NAME"
BACKUP_TMP="$BACKUP_DIR/.tmp_$BACKUP_NAME"
WORK_DIR=$(mktemp -d /tmp/ynxwxcb_bak_XXXXXX)

trap 'rm -rf "$WORK_DIR" "$BACKUP_TMP"' EXIT

echo "================================================================================"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 开始执行伊宁县委宣传部部务工作平台全量数据灾备..."
echo "  - 应用目录: $APP_DIR"
echo "  - 目标目录: $BACKUP_DIR"
echo "================================================================================"

# 2. 基础目录就绪与权限检查
mkdir -p "$BACKUP_DIR"

if [ ! -f "$DB_PATH" ]; then
    echo "❌ 错误: 未找到主数据库文件 $DB_PATH，备份中止！" >&2
    exit 1
fi

# 3. 磁盘可用空间预检（至少保留 300MB 可用空间）
AVAIL_KB=$(df -P "$BACKUP_DIR" | tail -1 | awk '{print $4}')
if [ "$AVAIL_KB" -lt 307200 ]; then
    echo "❌ 错误: 目标磁盘可用空间不足 300MB (剩余: ${AVAIL_KB}KB)，为保证生产安全，备份中止！" >&2
    exit 1
fi

# 4. 数据库一致性快照提取
TEMP_DB="$WORK_DIR/ynxwxcb.db"
if command -v sqlite3 >/dev/null 2>&1; then
    echo "  [1/4] 检测到系统 sqlite3 工具，正在执行在线事务一致性快照 (.backup)..."
    sqlite3 "$DB_PATH" ".backup '$TEMP_DB'"
else
    echo "  [1/4] ⚠️ 未安装 sqlite3 工具 (建议执行 sudo apt install -y sqlite3)，正在执行全量多文件快照..."
    cp "$DB_PATH" "$TEMP_DB"
    [ -f "$DB_PATH-wal" ] && cp "$DB_PATH-wal" "$WORK_DIR/ynxwxcb.db-wal"
    [ -f "$DB_PATH-shm" ] && cp "$DB_PATH-shm" "$WORK_DIR/ynxwxcb.db-shm"
fi

# 5. 准备上传附件归档
echo "  [2/4] 检查公文与材料上传附件目录 (uploads/)..."
if [ -d "$UPLOADS_DIR" ]; then
    cp -r "$UPLOADS_DIR" "$WORK_DIR/uploads"
else
    mkdir -p "$WORK_DIR/uploads"
fi

# 6. 原子性压缩打包并计算 SHA256
echo "  [3/4] 正在归档压缩生成物理备份包..."
tar -czf "$BACKUP_TMP" \
    --exclude="*.tmp" \
    --exclude="*.bak" \
    --exclude=".DS_Store" \
    --exclude="Thumbs.db" \
    -C "$WORK_DIR" .

if [ ! -s "$BACKUP_TMP" ]; then
    echo "❌ 错误: 生成的备份文件大小为 0，备份失败！" >&2
    exit 1
fi

mv "$BACKUP_TMP" "$BACKUP_FINAL"

# 生成校验和文件
if command -v sha256sum >/dev/null 2>&1; then
    (cd "$BACKUP_DIR" && sha256sum "$BACKUP_NAME" > "$BACKUP_FINAL.sha256")
elif command -v shasum >/dev/null 2>&1; then
    (cd "$BACKUP_DIR" && shasum -a 256 "$BACKUP_NAME" > "$BACKUP_FINAL.sha256")
fi

BACKUP_SIZE=$(du -h "$BACKUP_FINAL" | cut -f1)
echo "  [4/4] 备份归档完成: $BACKUP_FINAL (体积: $BACKUP_SIZE)"

# 7. 清理超过保留周期的历史备份
echo "  [-] 正在清理超过 $RETENTION_DAYS 天的历史旧备份..."
find "$BACKUP_DIR" -maxdepth 1 -name "ynxwxcb_*.tar.gz" -mtime +"$RETENTION_DAYS" -exec rm -f {} +
find "$BACKUP_DIR" -maxdepth 1 -name "ynxwxcb_*.tar.gz.sha256" -mtime +"$RETENTION_DAYS" -exec rm -f {} +

echo "================================================================================"
echo "✅ [$(date '+%Y-%m-%d %H:%M:%S')] 数据灾备全流程执行成功！"
echo "  - 备份文件: $BACKUP_FINAL"
echo "  - 校验文件: $BACKUP_FINAL.sha256"
echo "  - 灾难恢复指引: 详见 deploy/restore.sh 或 docs/DEPLOY.md"
echo "================================================================================"
