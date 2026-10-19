#!/bin/bash
# ==============================================================================
# 伊宁县委宣传部部务工作平台 - 生产数据灾难恢复脚本
# ==============================================================================
# 用法:
#   sudo /opt/ynxwxcb/deploy/restore.sh /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz
#   （脚本放在 /opt/ynxwxcb/restore.sh 也可自动识别应用目录；或用 APP_DIR=/实际路径 显式指定）
#
# 安全机制：
# 1. 先校验 SHA256、先解包并自检数据库，确认备份可用后才停服务，缩短停机窗口
# 2. 恢复前二次安全备份：自动将当前生产环境 data/ 备份为 data.pre_restore_bak_*
# 3. 自动停止与重启 systemd 服务；停服后任一步骤失败，自动从兜底备份回滚 data/ 并重新拉起服务
# 4. 正确处理 SQLite WAL：清理旧 -wal/-shm；若备份包内带 -wal（旧版脚本生成）则一并恢复
# 5. 自动修复 Linux 用户权限 (ynxwxcb:ynxwxcb，目录 750 / 文件 640)
# ==============================================================================

set -euo pipefail
umask 027

if [ $# -lt 1 ]; then
    echo "❌ 错误: 请指定要恢复的备份文件路径！"
    echo "用法: sudo $0 <备份文件.tar.gz>"
    echo "示例: sudo $0 /opt/ynxwxcb-backup/ynxwxcb_20261010_020000.tar.gz"
    exit 1
fi

BACKUP_TAR="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"

if [ ! -f "$BACKUP_TAR" ]; then
    echo "❌ 错误: 备份文件不存在: $BACKUP_TAR"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
resolve_app_dir() {
    local d
    for d in "$SCRIPT_DIR" "$SCRIPT_DIR/.."; do
        if [ -d "$d/data" ] || [ -f "$d/ynxwxcb-server" ]; then
            (cd "$d" && pwd)
            return
        fi
    done
    echo "/opt/ynxwxcb"
}
APP_DIR="${APP_DIR:-$(resolve_app_dir)}"

DATA_DIR="$APP_DIR/data"
SERVICE_NAME="ynxwxcb"
RUN_USER="ynxwxcb"
DATE=$(date +%Y%m%d_%H%M%S)
EMERGENCY_BAK=""
TEMP_RESTORE_DIR=""
SYSTEMD_MANAGED=0      # 是否由 systemd 管理
SERVICE_STOPPED=0      # 是否已由本脚本停止服务（异常时据此自动重启）
DATA_TOUCHED=0         # 是否已开始改写 data/
RESTORE_DONE=0         # 是否已全部完成

has_systemd_unit() {
    command -v systemctl >/dev/null 2>&1 && systemctl cat "$SERVICE_NAME" >/dev/null 2>&1
}

# 异常退出兜底：停服后任何一步失败，先回滚 data/，再自动重新拉起服务，避免业务长时间中断
on_exit() {
    local rc=$?
    if [ "$RESTORE_DONE" -ne 1 ] && [ "$SERVICE_STOPPED" -eq 1 ]; then
        echo "" >&2
        echo "❌ 恢复过程异常中断（退出码 $rc），正在执行自动兜底..." >&2
        if [ "$DATA_TOUCHED" -eq 1 ] && [ -n "$EMERGENCY_BAK" ] && [ -d "$EMERGENCY_BAK" ]; then
            echo "  -> 正在从兜底备份回滚 data/: $EMERGENCY_BAK" >&2
            rm -rf "$DATA_DIR" && cp -a "$EMERGENCY_BAK" "$DATA_DIR" \
                || echo "  ⚠️ 自动回滚失败，请手动执行: rm -rf $DATA_DIR && cp -a $EMERGENCY_BAK $DATA_DIR" >&2
        fi
        echo "  -> 正在重新启动服务 ($SERVICE_NAME)..." >&2
        if systemctl start "$SERVICE_NAME"; then
            echo "  -> 服务已重新拉起，业务数据保持恢复前状态。" >&2
        else
            echo "  ⚠️ 服务重启失败，请检查: systemctl status $SERVICE_NAME" >&2
        fi
    fi
    [ -n "$TEMP_RESTORE_DIR" ] && rm -rf "$TEMP_RESTORE_DIR"
    exit "$rc"
}
trap on_exit EXIT

echo "================================================================================"
echo "⚠️  【高危警示】即将执行伊宁县委宣传部部务工作平台数据恢复操作！"
echo "  - 目标应用目录: $APP_DIR"
echo "  - 待还原备份包: $BACKUP_TAR"
echo "================================================================================"

# 1. 校验 SHA256（仅在校验真正执行且通过时才提示“通过”）
echo "  [1/7] 正在校验备份包 SHA256 完整性..."
if [ -f "$BACKUP_TAR.sha256" ]; then
    SUM_DIR="$(dirname "$BACKUP_TAR")"
    SUM_FILE="$(basename "$BACKUP_TAR").sha256"
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$SUM_DIR" && sha256sum -c "$SUM_FILE") || { echo "❌ 错误: SHA256 校验失败，备份包可能已损坏或被篡改，恢复中止！" >&2; exit 1; }
        echo "  -> SHA256 校验通过！"
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$SUM_DIR" && shasum -a 256 -c "$SUM_FILE") || { echo "❌ 错误: SHA256 校验失败，备份包可能已损坏或被篡改，恢复中止！" >&2; exit 1; }
        echo "  -> SHA256 校验通过！"
    else
        echo "  ⚠️ 警告: 系统无 sha256sum / shasum 工具，未能校验备份包完整性（未校验，不代表通过）。"
    fi
else
    echo "  ⚠️ 警告: 未找到校验文件 $BACKUP_TAR.sha256，跳过完整性校验（未校验，不代表通过）。"
fi

# 2. 先解包到与 data/ 同磁盘的临时目录（不使用 /tmp 内存盘），并检验内容
echo "  [2/7] 正在解压并检验备份包内容（此时服务仍在运行）..."
TEMP_RESTORE_DIR=$(mktemp -d "$APP_DIR/.restore_XXXXXX")
tar -xzf "$BACKUP_TAR" -C "$TEMP_RESTORE_DIR"

if [ ! -f "$TEMP_RESTORE_DIR/ynxwxcb.db" ]; then
    echo "❌ 错误: 备份包内缺少关键数据库文件 ynxwxcb.db，恢复中止！" >&2
    exit 1
fi

HAS_WAL=0
if [ -s "$TEMP_RESTORE_DIR/ynxwxcb.db-wal" ]; then
    HAS_WAL=1
    echo "  -> 备份包内含 ynxwxcb.db-wal（旧版脚本生成的多文件备份），将与数据库一并恢复。"
fi
# -shm 仅为共享内存索引，可由 SQLite 自动重建；沿用旧 -shm 反而可能与 -wal 不匹配，一律丢弃
rm -f "$TEMP_RESTORE_DIR/ynxwxcb.db-shm"

if command -v sqlite3 >/dev/null 2>&1; then
    # 在临时副本上自检（若含 -wal，SQLite 会自动合并），不影响待恢复的原始文件
    CHECK_DIR="$TEMP_RESTORE_DIR/.check"
    mkdir -p "$CHECK_DIR"
    cp "$TEMP_RESTORE_DIR/ynxwxcb.db" "$CHECK_DIR/ynxwxcb.db"
    [ "$HAS_WAL" -eq 1 ] && cp "$TEMP_RESTORE_DIR/ynxwxcb.db-wal" "$CHECK_DIR/ynxwxcb.db-wal"
    INTEGRITY=$(sqlite3 -bail "$CHECK_DIR/ynxwxcb.db" "PRAGMA integrity_check;" 2>&1 || true)
    rm -rf "$CHECK_DIR"
    if [ "$INTEGRITY" != "ok" ]; then
        echo "❌ 错误: 备份包内数据库完整性自检未通过，恢复中止！检查结果：" >&2
        echo "$INTEGRITY" | head -20 >&2
        exit 1
    fi
    echo "  -> 数据库完整性自检通过 (ok)"
else
    echo "  ⚠️ 警告: 未安装 sqlite3，跳过数据库完整性自检（建议 sudo apt install -y sqlite3）。"
fi

# 3. 交互式二次确认
read -r -p "此操作将停用业务服务并覆盖当前数据库和附件，确认继续请按 [Enter]，取消请按 [Ctrl+C]: " _CONFIRM

# 4. 停止业务服务（停止失败则中止，绝不在服务运行时覆盖数据库）
echo "  [3/7] 正在安全停止系统服务 ($SERVICE_NAME)..."
if has_systemd_unit; then
    SYSTEMD_MANAGED=1
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        systemctl stop "$SERVICE_NAME"
        SERVICE_STOPPED=1
    else
        echo "  -> 服务当前未运行。"
    fi
else
    echo "  ⚠️ 未检测到 systemd 服务 $SERVICE_NAME，请确认 ynxwxcb-server 进程已手动停止。"
fi
if command -v pgrep >/dev/null 2>&1 && pgrep -x ynxwxcb-server >/dev/null 2>&1; then
    echo "❌ 错误: 检测到 ynxwxcb-server 进程仍在运行，为防止数据库损坏，恢复中止！" >&2
    exit 1
fi

# 5. 备份当前数据（防灾撤退兜底，保留原属主与权限）
if [ -d "$DATA_DIR" ]; then
    EMERGENCY_BAK="$APP_DIR/data.pre_restore_bak_$DATE"
    echo "  [4/7] 正在为当前运行数据创建防灾兜底备份: $EMERGENCY_BAK ..."
    cp -a "$DATA_DIR" "$EMERGENCY_BAK"
    chmod 700 "$EMERGENCY_BAK"
else
    echo "  [4/7] 当前无 data/ 目录，跳过兜底备份。"
fi

# 6. 写回数据库与附件
echo "  [5/7] 正在写回数据库与 uploads 物理附件..."
DATA_TOUCHED=1
mkdir -p "$DATA_DIR"
# 清理旧数据库及 WAL 临时文件，避免新旧合流冲突
rm -f "$DATA_DIR/ynxwxcb.db" "$DATA_DIR/ynxwxcb.db-wal" "$DATA_DIR/ynxwxcb.db-shm"
mv "$TEMP_RESTORE_DIR/ynxwxcb.db" "$DATA_DIR/ynxwxcb.db"
if [ "$HAS_WAL" -eq 1 ]; then
    mv "$TEMP_RESTORE_DIR/ynxwxcb.db-wal" "$DATA_DIR/ynxwxcb.db-wal"
fi

if [ -d "$TEMP_RESTORE_DIR/uploads" ]; then
    rm -rf "$DATA_DIR/uploads"
    mv "$TEMP_RESTORE_DIR/uploads" "$DATA_DIR/uploads"
else
    echo "  ⚠️ 备份包内无 uploads/ 目录，保留现有附件不变。"
fi

# 7. 修复文件归属与权限：目录 750、文件 640（不再 chmod -R 750 把所有文件变成可执行）
echo "  [6/7] 正在修复文件归属与权限..."
if id "$RUN_USER" >/dev/null 2>&1; then
    chown -R "$RUN_USER:$RUN_USER" "$DATA_DIR"
fi
find "$DATA_DIR" -type d -exec chmod 750 {} +
find "$DATA_DIR" -type f -exec chmod 640 {} +

# 8. 重新启动服务
echo "  [7/7] 正在启动系统服务 ($SERVICE_NAME)..."
# 数据已全部写回，此后即使启动失败也不再自动回滚，保留恢复结果供人工排查
RESTORE_DONE=1
if [ "$SYSTEMD_MANAGED" -eq 1 ]; then
    systemctl start "$SERVICE_NAME" || true
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "  -> 服务已成功恢复运行状态！"
    else
        echo "⚠️ 警告: 服务启动异常，请检查 systemctl status $SERVICE_NAME 与 journalctl -u $SERVICE_NAME"
        echo "   如需撤回本次恢复: systemctl stop $SERVICE_NAME && rm -rf $DATA_DIR && cp -a ${EMERGENCY_BAK:-<兜底备份>} $DATA_DIR"
    fi
else
    echo "  -> 未通过 systemd 管理，请手动启动 ynxwxcb-server。"
fi

echo "================================================================================"
echo "✅ [$(date '+%Y-%m-%d %H:%M:%S')] 平台数据灾难恢复圆满完成！"
echo "  - 数据目录: $DATA_DIR"
echo "  - 应急防灾兜底备份: ${EMERGENCY_BAK:-无}（确认业务正常后可手动删除）"
echo "================================================================================"
