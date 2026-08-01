#!/usr/bin/env bash
# =============================================================
# cef-osr-test 交叉构建脚本（在 WSL2 / Linux 上运行）
#
# 用途：在隔离环境（WSL2）里完成 CEF OSR 验证程序的 Linux 构建，
#       产物打包成 tar.gz，拷到 N100 服务器直接运行——服务器零编译环境。
#
# 用法：
#   1. Windows 侧装 WSL2 + Ubuntu（wsl --install -d Ubuntu）
#   2. 把本目录拷进 WSL：  wsl cp -r e:\WORK\Web-RTMP\tools\cef-osr-test ~/cef-osr-test
#      （建议拷进 WSL 家目录，避免 /mnt 挂载性能/权限问题）
#   3. 在 WSL 里：  cd ~/cef-osr-test && bash build-linux-cef.sh
#   4. 产物：  dist/cef-osr-test-linux.tar.gz（拷到服务器解压，dist/run.sh 运行）
#
# 清理（服务器零残留）：
#   wsl --unregister Ubuntu      # 删掉整个构建环境
# =============================================================
set -e

BUILD_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT_DIR="$BUILD_DIR/dist"
ENERGY_HOME="${ENERGY_HOME:-$HOME/cef-runtime}"
GO_VERSION="${GO_VERSION:-1.22.12}"
LOG="$BUILD_DIR/build.log"
SOURCEFORGE_CEF_URL="https://sourceforge.net/projects/liblcl/files/v3.0.0/lcl_cef_binary_linux64.zip/download"

log() { echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG"; }

mkdir -p "$OUT_DIR"
: > "$LOG"

log "==== cef-osr-test Linux 构建 ===="
log "构建目录: $BUILD_DIR"
log "输出目录: $OUT_DIR"

# ---- 0. 平台检查 ----
if [ "$(uname -s)" != "Linux" ]; then
  log "错误: 本脚本需在 Linux（WSL2）中运行"
  exit 1
fi
if ! grep -qi microsoft /proc/version 2>/dev/null; then
  log "提示: 当前不是 WSL（可能是原生 Linux），脚本仍可继续"
fi

# ---- 1. Go 工具链（没有才装，国内镜像）----
log "== 1/5 Go 工具链 =="
if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
  log "已存在 Go: $GO_BIN ($(go version))"
else
  log "未找到 Go，从阿里云镜像下载 go${GO_VERSION}.linux-amd64 ..."
  mkdir -p ~/go-install && cd ~/go-install
  curl -fsSL -o go.tar.gz "https://mirrors.aliyun.com/golang/go${GO_VERSION}.linux-amd64.tar.gz"
  sudo -E tar -C /usr/local -xzf go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
  grep -q '/usr/local/go/bin' ~/.bashrc || echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  log "Go 已安装: $(go version)"
fi
# 模块代理（国内）
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export CGO_ENABLED=1

# ---- 2. GTK3 开发库（编译 energye/lcl 需要）----
log "== 2/5 GTK3 开发库 =="
if ! pkg-config --exists gtk+-3.0 2>/dev/null; then
  log "安装 libgtk-3-dev（可能提示输 sudo 密码）..."
  sudo apt-get update -qq
  sudo apt-get install -y -qq libgtk-3-dev unzip curl
fi
log "GTK3: $(pkg-config --modversion gtk+-3.0)"

# ---- 3. CEF 二进制（liblcl + libcef），解压到本地（不进系统）----
log "== 3/5 CEF 二进制 =="
mkdir -p "$ENERGY_HOME"
if [ ! -f "$ENERGY_HOME/libenergy-gtk3-147.so" ]; then
  log "下载 CEF Linux64 二进制（sourceforge，可能较慢）..."
  cd /tmp
  curl -fSL -o lcl_cef.zip "$SOURCEFORGE_CEF_URL"
  unzip -o -q lcl_cef.zip -d "$ENERGY_HOME"
  log "已解压到 $ENERGY_HOME"
else
  log "CEF 二进制已存在: $ENERGY_HOME"
fi
# 完整性检查
log "CEF 文件:"
ls -la "$ENERGY_HOME"/libenergy*.so "$ENERGY_HOME"/libcef.so 2>/dev/null \
  | tee -a "$LOG" || log "警告: 未找到 libenergy/libcef，请检查下载的 zip 内容"

# ---- 4. 编译 ----
log "== 4/5 编译 cef-osr-test =="
cd "$BUILD_DIR"
go mod tidy 2>&1 | tee -a "$LOG"
go build -o cef-osr-test . 2>&1 | tee -a "$LOG"
log "编译完成: $BUILD_DIR/cef-osr-test ($(du -h cef-osr-test | cut -f1))"

# ---- 5. 打包产物（二进制 + CEF 运行库 + 运行脚本）----
log "== 5/5 打包 =="
cp cef-osr-test "$OUT_DIR/"
mkdir -p "$OUT_DIR/runtime"
cp -a "$ENERGY_HOME"/. "$OUT_DIR/runtime/" 2>/dev/null || true

cat > "$OUT_DIR/run.sh" <<'EOF'
#!/usr/bin/env bash
# cef-osr-test 运行脚本：指向包内 CEF 运行库，服务器无需安装
DIR="$(cd "$(dirname "$0")" && pwd)"
export LD_LIBRARY_PATH="$DIR/runtime:${LD_LIBRARY_PATH:-}"
export ENERGY_HOME="$DIR/runtime"
exec "$DIR/cef-osr-test" "$@"
EOF
chmod +x "$OUT_DIR/run.sh"

cd "$OUT_DIR"
tar czf cef-osr-test-linux.tar.gz ./*
log "完成! 产物: $OUT_DIR/cef-osr-test-linux.tar.gz"
log ""
log "下一步（N100 服务器）："
log "  1. 拷包:  scp $OUT_DIR/cef-osr-test-linux.tar.gz root@N100:~/"
log "  2. 解压:  tar xzf cef-osr-test-linux.tar.gz && cd cef-osr-test-linux"
log "  3. 起 Xvfb:  pkill Xvfb; Xvfb :99 -screen 0 1920x1080x24 -ac &"
log "  4. 运行:   DISPLAY=:99 ./run.sh -fps 30 -duration 20 -log test.log"
log ""
log "（若服务器缺 GTK3 运行时: sudo apt install -y libgtk-3-0）"
