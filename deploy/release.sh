#!/bin/bash
# 生成 Linux amd64 发布包
# 用法: ./release.sh   或  bash release.sh
#
# 包内文件（均在包根目录，不套层）：
#   ynxwxcb-server        主程序（Linux amd64）
#   static/               前端页面（后端运行时从工作目录读取 static/，未内嵌进二进制，必须随包发布）
#   config.json.example   配置模板（绝不打包真实 config.json）
#   backup.sh / restore.sh  灾备与恢复脚本
#   ynxwxcb.service       systemd 服务单元
#   ynxwxcb.conf          Nginx 反代配置
#   README.md             部署文档（docs/DEPLOY.md）

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="$ROOT/release"
VERSION=$(date +%Y%m%d)
STAGE="$RELEASE_DIR/stage-$VERSION"
PKG="$ROOT/ynxwxcb-release-$VERSION.tar.gz"

# 每次使用全新的暂存目录，杜绝旧产物（尤其是真实 config.json）混入发布包
rm -rf "$STAGE"
mkdir -p "$STAGE"

echo "==> 构建前端"
cd "$ROOT/frontend"
npm run build
rm -rf "$ROOT/backend/static"
mkdir -p "$ROOT/backend/static"
cp -r dist/* "$ROOT/backend/static/"

echo "==> 构建后端 (linux/amd64)"
cd "$ROOT/backend"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$STAGE/ynxwxcb-server" ./cmd/server

echo "==> 复制前端静态资源与部署文件"
cp -r "$ROOT/backend/static" "$STAGE/static"
cp "$ROOT/deploy/config.json.example" "$STAGE/config.json.example"
cp "$ROOT/deploy/backup.sh" "$ROOT/deploy/restore.sh" "$STAGE/"
cp "$ROOT/deploy/systemd/ynxwxcb.service" "$STAGE/"
cp "$ROOT/deploy/nginx/ynxwxcb.conf" "$STAGE/"
cp "$ROOT/docs/DEPLOY.md" "$STAGE/README.md"
chmod 755 "$STAGE/ynxwxcb-server" "$STAGE/backup.sh" "$STAGE/restore.sh"

echo "==> 打包前自检"
if [ ! -f "$STAGE/static/index.html" ]; then
    echo "错误: static/index.html 不存在，前端未正确构建" >&2
    exit 1
fi
if [ -e "$STAGE/config.json" ]; then
    echo "错误: 暂存目录中出现真实 config.json，拒绝打包" >&2
    exit 1
fi
echo "    前端标题: $(grep -o '<title>[^<]*</title>' "$STAGE/static/index.html" || echo '未找到 <title>')"

echo "==> 打包"
# COPYFILE_DISABLE=1：macOS 上打包时不夹带 ._* 资源分叉文件
COPYFILE_DISABLE=1 tar -czf "$PKG" -C "$STAGE" .

echo "完成: $PKG"
echo "上传到服务器后按 README.md 部署（升级时只覆盖 ynxwxcb-server 与 static/，绝不覆盖 data/ 与 config.json）"
