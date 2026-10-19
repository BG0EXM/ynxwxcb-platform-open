#!/bin/bash
# ==============================================================================
# 伊宁县委宣传部部务工作平台 - 生产全量灾备脚本 (数据库 + 上传附件)
# ==============================================================================
# 功能特性：
# 1. 强事务一致性：强制调用 sqlite3 在线快照备份引擎 (.backup)，生成单文件一致性快照，
#    并执行 PRAGMA integrity_check 完整性自检（未安装 sqlite3 时直接报错退出，绝不裸拷运行中的库文件）
# 2. 全量物理灾备：打包完整 SQLite 数据库及全部公文、征求意见盖章件附件 (uploads/)
# 3. 磁盘安全预检：按「数据库 + 附件」实际体积预估所需空间，杜绝写满磁盘造成服务宕机
# 4. 原子安全写入：在备份目录内缓冲打包（不占用 /tmp 内存盘）+ sha256 完整性校验，杜绝损坏的半截备份
# 5. 权限收敛：备份目录 700、备份包 600，仅 root 可读（备份包内含用户密码哈希等敏感数据）
# 6. 自动过期轮转：默认安全保留 14 天历史快照，过期自动清理
#
# 前置依赖：sudo apt install -y sqlite3
#
# 脚本可放在 /opt/ynxwxcb/backup.sh 或 /opt/ynxwxcb/deploy/backup.sh，均可自动识别应用目录；
# 也可通过环境变量 APP_DIR / BACKUP_DIR / RETENTION_DAYS 显式指定。
#
# 建议配置 root 的 crontab 定时调度（每天凌晨 2:00 执行）:
#   0 2 * * * /opt/ynxwxcb/deploy/backup.sh >> /var/log/ynxwxcb-backup.log 2>&1
# ==============================================================================

set -euo pipefail
# 本脚本新建的所有文件/目录仅属主可读写（备份包 600、目录 700）
umask 077

# 1. 路径自动推导与配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 依次尝试：脚本所在目录（/opt/ynxwxcb/backup.sh）→ 上级目录（/opt/ynxwxcb/deploy/backup.sh）→ 标准路径
resolve_app_dir() {
    local d
    for d in "$SCRIPT_DIR" "$SCRIPT_DIR/.."; do
        if [ -f "$d/data/ynxwxcb.db" ] || [ -f "$d/ynxwxcb-server" ]; then
            (cd "$d" && pwd)
            return
        fi
    done
    echo "/opt/ynxwxcb"
}
APP_DIR="${APP_DIR:-$(resolve_app_dir)}"

DATA_DIR="$APP_DIR/data"
DB_PATH="$DATA_DIR/ynxwxcb.db"
UPLOADS_DIR="$DATA_DIR/uploads"
BACKUP_DIR="${BACKUP_DIR:-/opt/ynxwxcb-backup}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
RESERVE_KB=307200   # 备份完成后磁盘至少仍保留 300MB 可用空间

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="ynxwxcb_$DATE.tar.gz"
BACKUP_FINAL="$BACKUP_DIR/$BACKUP_NAME"
BACKUP_TMP="$BACKUP_DIR/.tmp_$BACKUP_NAME"
WORK_DIR=""

cleanup() {
    [ -n "$WORK_DIR" ] && rm -rf "$WORK_DIR"
    rm -f "$BACKUP_TMP"
}
trap cleanup EXIT

echo "================================================================================"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] 开始执行伊宁县委宣传部部务工作平台全量数据灾备..."
echo "  - 应用目录: $APP_DIR"
echo "  - 目标目录: $BACKUP_DIR"
echo "================================================================================"

# 2. 前置依赖与基础目录检查
if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "❌ 错误: 未安装 sqlite3 工具，无法生成事务一致的数据库快照，备份中止！" >&2
    echo "   请先执行: sudo apt install -y sqlite3" >&2
    exit 1
fi

if [ ! -f "$DB_PATH" ]; then
    echo "❌ 错误: 未找到主数据库文件 $DB_PATH，备份中止！（可用 APP_DIR=/实际路径 显式指定）" >&2
    exit 1
fi

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

# 防止两个备份任务同时运行（如手动执行恰逢 crontab 定时触发）
if command -v flock >/dev/null 2>&1; then
    exec 9>"$BACKUP_DIR/.backup.lock"
    if ! flock -n 9; then
        echo "❌ 错误: 已有另一个备份任务正在运行，本次备份中止！" >&2
        exit 1
    fi
fi

# 清理异常中断（如 kill -9 / 断电）遗留的超过 1 天的临时目录与半截包
find "$BACKUP_DIR" -maxdepth 1 \( -name ".work_*" -o -name ".tmp_ynxwxcb_*" \) -mmin +1440 -exec rm -rf {} + 2>/dev/null || true

# 3. 磁盘可用空间预检
#    需要空间 ≈ 数据库快照 + 压缩包（最坏按数据库 + 附件未压缩体积估算）+ 300MB 保留余量
DB_KB=$(du -sk "$DB_PATH" | awk '{print $1}')
if [ -f "$DB_PATH-wal" ]; then
    DB_KB=$((DB_KB + $(du -sk "$DB_PATH-wal" | awk '{print $1}')))
fi
UPLOADS_KB=0
if [ -d "$UPLOADS_DIR" ]; then
    UPLOADS_KB=$(du -sk "$UPLOADS_DIR" | awk '{print $1}')
fi
NEED_KB=$((DB_KB * 2 + UPLOADS_KB + RESERVE_KB))
AVAIL_KB=$(df -Pk "$BACKUP_DIR" | tail -1 | awk '{print $4}')
echo "  [0/5] 空间预检: 数据库约 $((DB_KB / 1024))MB，附件约 $((UPLOADS_KB / 1024))MB，需要约 $((NEED_KB / 1024))MB，可用 $((AVAIL_KB / 1024))MB"
if [ "$AVAIL_KB" -lt "$NEED_KB" ]; then
    echo "❌ 错误: 目标磁盘可用空间不足（需要约 $((NEED_KB / 1024))MB，剩余 $((AVAIL_KB / 1024))MB），为保证生产安全，备份中止！" >&2
    exit 1
fi

# 4. 数据库一致性快照提取（暂存于备份目录内，不使用 /tmp，避免 tmpfs 占用内存）
WORK_DIR=$(mktemp -d "$BACKUP_DIR/.work_XXXXXX")
SNAPSHOT_DB="$WORK_DIR/ynxwxcb.db"

echo "  [1/5] 正在执行 sqlite3 在线事务一致性快照 (.backup)..."
sqlite3 -bail "$DB_PATH" <<EOF
.timeout 10000
.backup '$SNAPSHOT_DB'
EOF

# root 执行 sqlite3 时若新建了 -wal/-shm，需归还给数据库属主，否则业务服务可能因无权限打不开库
if [ "$(id -u)" -eq 0 ]; then
    DB_OWNER=$(stat -c '%u:%g' "$DB_PATH" 2>/dev/null || true)
    if [ -n "$DB_OWNER" ]; then
        for f in "$DB_PATH-wal" "$DB_PATH-shm"; do
            if [ -e "$f" ]; then chown "$DB_OWNER" "$f" || true; fi
        done
    fi
fi

# 将快照切换为单文件模式（不依赖 -wal/-shm），恢复时直接替换即可；业务服务启动时会自动重新开启 WAL
sqlite3 -bail "$SNAPSHOT_DB" "PRAGMA journal_mode=DELETE;" >/dev/null

echo "  [2/5] 正在执行快照完整性自检 (PRAGMA integrity_check)..."
INTEGRITY=$(sqlite3 -bail "$SNAPSHOT_DB" "PRAGMA integrity_check;")
if [ "$INTEGRITY" != "ok" ]; then
    echo "❌ 错误: 数据库快照完整性自检未通过，备份中止！检查结果：" >&2
    echo "$INTEGRITY" | head -20 >&2
    exit 1
fi
echo "  -> 完整性自检通过 (ok)"

# 5. 原子性压缩打包：数据库快照取自暂存目录，附件直接从 DATA_DIR 打包（不再整份复制）
echo "  [3/5] 正在归档压缩生成物理备份包（数据库快照 + uploads/ 附件）..."
TAR_SOURCES=(-C "$WORK_DIR" ynxwxcb.db)
if [ -d "$UPLOADS_DIR" ]; then
    TAR_SOURCES+=(-C "$DATA_DIR" uploads)
else
    echo "  -> 未发现 uploads/ 目录，备份包内将生成空的 uploads/"
    mkdir -p "$WORK_DIR/uploads"
    TAR_SOURCES+=(-C "$WORK_DIR" uploads)
fi

set +e
tar -czf "$BACKUP_TMP" \
    --exclude="*.tmp" \
    --exclude="*.bak" \
    --exclude=".DS_Store" \
    --exclude="Thumbs.db" \
    "${TAR_SOURCES[@]}"
TAR_RC=$?
set -e
# GNU tar 返回 1 表示打包期间有附件被新增/修改/删除（业务仍在运行），归档本身可用，仅提示
if [ "$TAR_RC" -eq 1 ]; then
    echo "  ⚠️ 提示: 打包期间有附件发生变动（业务运行中属正常现象），本次备份仍然有效。"
elif [ "$TAR_RC" -ne 0 ]; then
    echo "❌ 错误: tar 打包失败（退出码 $TAR_RC），备份中止！" >&2
    exit 1
fi

if [ ! -s "$BACKUP_TMP" ] || ! tar -tzf "$BACKUP_TMP" >/dev/null 2>&1; then
    echo "❌ 错误: 生成的备份包为空或无法读取，备份失败！" >&2
    exit 1
fi

mv "$BACKUP_TMP" "$BACKUP_FINAL"
chmod 600 "$BACKUP_FINAL"

# 6. 生成校验和文件
echo "  [4/5] 正在生成 SHA256 校验文件..."
if command -v sha256sum >/dev/null 2>&1; then
    (cd "$BACKUP_DIR" && sha256sum "$BACKUP_NAME" > "$BACKUP_NAME.sha256")
elif command -v shasum >/dev/null 2>&1; then
    (cd "$BACKUP_DIR" && shasum -a 256 "$BACKUP_NAME" > "$BACKUP_NAME.sha256")
else
    echo "  ⚠️ 警告: 系统无 sha256sum / shasum，未生成校验文件。"
fi
[ -f "$BACKUP_FINAL.sha256" ] && chmod 600 "$BACKUP_FINAL.sha256"

BACKUP_SIZE=$(du -h "$BACKUP_FINAL" | cut -f1)
echo "  [5/5] 备份归档完成: $BACKUP_FINAL (体积: $BACKUP_SIZE)"

# 7. 收敛历史备份权限（旧版脚本生成的备份包为 644 所有人可读），并清理超过保留周期的旧备份
find "$BACKUP_DIR" -maxdepth 1 -type f -name "ynxwxcb_*.tar.gz*" ! -perm 600 -exec chmod 600 {} + 2>/dev/null || true
echo "  [-] 正在清理超过 $RETENTION_DAYS 天的历史旧备份..."
find "$BACKUP_DIR" -maxdepth 1 -name "ynxwxcb_*.tar.gz" -mtime +"$RETENTION_DAYS" -exec rm -f {} +
find "$BACKUP_DIR" -maxdepth 1 -name "ynxwxcb_*.tar.gz.sha256" -mtime +"$RETENTION_DAYS" -exec rm -f {} +

echo "================================================================================"
echo "✅ [$(date '+%Y-%m-%d %H:%M:%S')] 数据灾备全流程执行成功！"
echo "  - 备份文件: $BACKUP_FINAL"
echo "  - 校验文件: $BACKUP_FINAL.sha256"
echo "  - 灾难恢复指引: 详见 restore.sh 或 docs/DEPLOY.md"
echo "================================================================================"
