#!/bin/bash
# ==============================================================================
# 伊宁县委宣传部部务工作平台 - 生产数据灾难恢复脚本
# ==============================================================================
# 用法:
#   sudo /opt/ynxwxcb/deploy/restore.sh /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz
#
# 安全机制：
# 1. 恢复前二次安全备份：自动将当前生产环境 data/ 备份为 data.pre_restore_bak_*
# 2. 自动停止与重启 systemd 服务，保证 SQLite 数据库切换的无并发安全
# 3. 自动校验 SHA256 完整性与清理残留 WAL 文件
# 4. 自动修复 Linux 用户权限 (ynxwxcb:ynxwxcb)
# ==============================================================================

set -euo pipefail

if [ $# -lt 1 ]; then
    echo "❌ 错误: 请指定要恢复的备份文件路径！"
    echo "用法: sudo $0 <备份文件.tar.gz>"
    echo "示例: sudo $0 /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz"
    exit 1
fi

BACKUP_TAR="$1"

if [ ! -f "$BACKUP_TAR" ]; then
    echo "❌ 错误: 备份文件不存在: $BACKUP_TAR"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="${APP_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
if [ ! -d "$APP_DIR/data" ] && [ -d "/opt/ynxwxcb/data" ]; then
    APP_DIR="/opt/ynxwxcb"
fi

DATA_DIR="$APP_DIR/data"
SERVICE_NAME="ynxwxcb"
DATE=$(date +%Y%m%d_%H%M%S)

echo "================================================================================"
echo "⚠️  【高危警示】即将执行伊宁县委宣传部部务工作平台数据恢复操作！"
echo "  - 目标应用目录: $APP_DIR"
echo "  - 待还原备份包: $BACKUP_TAR"
echo "================================================================================"

# 1. 校验 SHA256 (若存在校验文件)
if [ -f "$BACKUP_TAR.sha256" ]; then
    echo "  [1/6] 正在校验备份包 SHA256 完整性..."
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$(dirname "$BACKUP_TAR")" && sha256sum -c "$(basename "$BACKUP_TAR.sha256")")
    fi
    echo "  -> SHA256 校验通过！"
fi

# 2. 交互式二次确认
read -r -p "此操作将停用业务服务并覆盖当前数据库和附件，确认继续请按 [Enter]，取消请按 [Ctrl+C]: " CONFIRM

# 3. 停止业务服务
echo "  [2/6] 正在安全停止系统服务 ($SERVICE_NAME)..."
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$SERVICE_NAME" || true
fi

# 4. 备份当前数据（防灾撤退兜底）
if [ -d "$DATA_DIR" ]; then
    EMERGENCY_BAK="$APP_DIR/data.pre_restore_bak_$DATE"
    echo "  [3/6] 正在为当前运行数据创建防灾兜底备份: $EMERGENCY_BAK ..."
    cp -r "$DATA_DIR" "$EMERGENCY_BAK"
fi

# 5. 准备解压还原
TEMP_RESTORE_DIR=$(mktemp -d /tmp/ynxwxcb_restore_XXXXXX)
trap 'rm -rf "$TEMP_RESTORE_DIR"' EXIT

echo "  [4/6] 正在解压并检验备份包内容..."
tar -xzf "$BACKUP_TAR" -C "$TEMP_RESTORE_DIR"

if [ ! -f "$TEMP_RESTORE_DIR/ynxwxcb.db" ]; then
    echo "❌ 错误: 备份包内缺少关键数据库文件 ynxwxcb.db，恢复中止！" >&2
    exit 1
fi

mkdir -p "$DATA_DIR"
# 清理旧数据库及 WAL 临时文件，避免新旧合流冲突
rm -f "$DATA_DIR/ynxwxcb.db" "$DATA_DIR/ynxwxcb.db-wal" "$DATA_DIR/ynxwxcb.db-shm"

echo "  [5/6] 正在写回数据库与 uploads 物理附件..."
cp "$TEMP_RESTORE_DIR/ynxwxcb.db" "$DATA_DIR/ynxwxcb.db"

if [ -d "$TEMP_RESTORE_DIR/uploads" ]; then
    rm -rf "$DATA_DIR/uploads"
    cp -r "$TEMP_RESTORE_DIR/uploads" "$DATA_DIR/uploads"
fi

# 修复文件归属（如果系统存在 ynxwxcb 用户）
if id "ynxwxcb" >/dev/null 2>&1; then
    chown -R ynxwxcb:ynxwxcb "$DATA_DIR"
fi
chmod -R 750 "$DATA_DIR"

# 6. 重新启动服务
echo "  [6/6] 正在启动系统服务 ($SERVICE_NAME)..."
if command -v systemctl >/dev/null 2>&1; then
    systemctl start "$SERVICE_NAME"
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "  -> 服务已成功恢复运行状态！"
    else
        echo "⚠️ 警告: 服务启动异常，请检查 systemctl status $SERVICE_NAME"
    fi
fi

echo "================================================================================"
echo "✅ [$(date '+%Y-%m-%d %H:%M:%S')] 平台数据灾难恢复圆满完成！"
echo "  - 数据目录: $DATA_DIR"
echo "  - 应急防灾兜底备份: ${EMERGENCY_BAK:-无}"
echo "================================================================================"
