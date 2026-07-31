#!/bin/bash
# HeadlessLive 交叉编译脚本：编译 Linux x86_64 静态单文件
set -e

cd "$(dirname "$0")"

echo "=== HeadlessLive Build (x86_64 Linux) ==="
echo "Cleaning previous build..."
rm -f HeadlessLive

# 1. 前端构建（输出到 static/，供 Go embed）
echo "[1/3] Building frontend (Vue3 + Vite)..."
if [ -d "web" ]; then
  (cd web && npm install && npm run build)
fi

# 2. 依赖
# echo "[2/3] go mod tidy..."
go mod tidy

# 3. 后端交叉编译
echo "[3/3] Building Linux amd64 (static, no CGO)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o HeadlessLive .

echo ""
echo "=== Build Complete ==="
echo "Output: HeadlessLive"
ls -lh HeadlessLive
echo ""
echo "Deploy to server:"
echo "  scp HeadlessLive user@<server>:~/"
echo "  ssh user@<server> 'chmod +x ~/HeadlessLive && mkdir -p ~/uploads && ./HeadlessLive'"
echo "  Browser: http://<server>:8080"
