# HeadlessLive

> 无头直播演播室（Headless OBS）—— 部署在 Linux / Windows 服务器上、无界面运行的直播工具，
> 全部通过 **Web 浏览器远程控制**，采用 **源（Source）→ 场景（Scene）→ 输出（Output）**
> 三层模型，对标 OBS。内置 **B 站直播助手**（扫码登录 → 一键开播 → 自动填入推流密钥），
> 也支持通用 RTMP / SRT 推流与本地录制。提供 **Linux / Windows 单文件版**。

## 功能

- ✅ **无头运行（Headless）**：纯命令行部署在 Linux 服务器，无需显示器/GUI，全部通过 Web 控制
- ✅ **源（Source）**：采集卡 / 音频 / 图片 / 文字 / 媒体文件 / 屏幕 / RTMP 拉流 / 纯色 / **浏览器**
- ✅ **场景（Scene）**：多场景、画布分辨率、Z 轴叠放、位置/尺寸/裁剪/透明度、拖拽布局
- ✅ **输出（Output）**：RTMP 推流 / 本地录制 / SRT，多输出并行
- ✅ **动态滤镜链**：`filter_complex` 多输入叠加（overlay）+ 多路音频混音（amix）
- ✅ **MJPEG 预览流**：独立 FFmpeg 进程，浏览器实时预览
- ✅ **配置持久化**：`scenes.yaml` 保存源/场景/输出，Web UI 编辑即存
- ✅ **VAAPI/QSV 硬件编码**：自动探测 Intel 核显，低 CPU 占用
- ✅ **WebSocket**：实时日志 + 输出状态推送
- ✅ **Vue 3 + Vite 前端**：三栏布局（源/画布/属性）+ 拖拽 + 实时预览
- ✅ **单文件部署**：Linux / Windows 单文件二进制，前端已内嵌，无需安装运行时
- ✅ **B 站直播助手**：扫码登录 → 一键开播获取推流地址/密钥 → 自动填入输出配置（RTMP 推流到 B 站）

## 环境要求

### 运行环境

**Linux（推荐，功能完整）**

| 依赖          | 用途                                 |
| ------------- | ------------------------------------ |
| FFmpeg 4.x+   | 核心引擎：采集、滤镜合成、编码、推流 |
| v4l-utils     | 采集卡设备探测（`v4l2-ctl`）       |
| alsa-utils    | 声卡设备探测（`arecord`）          |
| vainfo        | Intel VAAPI 硬编探测                 |
| Xvfb + Chrome | 可选，仅「浏览器源」需要             |

```bash
sudo apt install ffmpeg v4l-utils vainfo alsa-utils
# 文字源（可选，中文需 CJK 字体）：sudo apt install fonts-noto-cjk
# 浏览器源（可选）：sudo apt install xvfb，浏览器安装见「浏览器源」章节（推荐 deb 版）
```

**Windows（单文件版）**

| 依赖   | 说明                                                                                                      |
| ------ | --------------------------------------------------------------------------------------------------------- |
| FFmpeg | 需自行安装并加入 PATH（如[gyan.dev 构建](https://www.gyan.dev/ffmpeg/builds/)），程序启动时调用 `ffmpeg` |

- ✅ 单文件 `HeadlessLive-windows-amd64.exe`，无需 Go / Node 或其它运行时
- ⚠️ 硬件采集（USB 采集卡 / USB 声卡）与 VAAPI 硬编为 **Linux 专属**，Windows 不可用
- ✅ Windows 可用源：图片 / 文字 / 纯色 / 媒体文件 / RTMP 拉流；编码用软件 `libx264`
- ✅ 推流（RTMP / SRT）与本地录制在 Windows 完全可用

> 硬件建议（Linux）：Intel 核显（VAAPI 硬编，低 CPU 占用）+ USB 采集卡（UVC）+ USB 声卡（如 Synido Voice 100）。

### 编译环境（可选 —— 直接下载 Release 二进制则不需要）

| 依赖    | 版本  | 用途                   |
| ------- | ----- | ---------------------- |
| Go      | 1.22+ | 后端编译               |
| Node.js | 18+   | 仅构建前端（`web/`） |

## 依赖与引用

### Go 后端（`go.mod`）

| 库                               | 版本   | 用途                       |
| -------------------------------- | ------ | -------------------------- |
| `github.com/gin-gonic/gin`     | v1.8.1 | HTTP 路由 / REST API       |
| `github.com/gorilla/websocket` | v1.5.1 | WebSocket 日志与状态推送   |
| `gopkg.in/yaml.v3`             | v3.0.1 | `scenes.yaml` 配置持久化 |

### 前端（`web/`）

| 库     | 版本   | 用途                   |
| ------ | ------ | ---------------------- |
| Vue    | 3.x    | 前端框架               |
| Vite   | 5.x    | 构建工具               |
| qrcode | ^1.5.4 | B 站扫码登录二维码生成 |

### 系统级依赖

- **FFmpeg**：核心引擎（采集 → 滤镜 → 编码 → 推流）
- **Chromium + Xvfb**：浏览器源（可选）

### 参考项目

- [GamerNoTitle/BiliLive-Utility](https://github.com/GamerNoTitle/BiliLive-Utility)（AGPL-3.0）—— 本项目的"internal/bilibili"按其公开的 **B 站直播接口协议**（扫码登录 / 开播 / 取推流密钥）独立编写的 Go 实现，未引用其源代码。

## 架构

```
Web UI (web/ —— Vue 3 + Vite，构建产物 → static/)
   │  REST API + WebSocket
   ▼
┌──────────────────────────────────────────────┐
│ internal/model   —— 源/场景/输出 数据结构      │
│ internal/store   —— scenes.yaml 持久化 + CRUD  │
│ internal/scene   —— 场景 → filter_complex 构建 │
│ internal/ffmpeg  —— 多输出进程 + MJPEG 预览     │
│ internal/capture —— v4l2/ALSA 设备探测/控制     │
│ internal/websocket —— 日志/状态推送 Hub         │
└──────────────────────────────────────────────┘
   │
   ▼ FFmpeg 子进程（每输出一个，共用渲染规格）
   输入 → filter_complex → 编码 → flv/mp4/... → 输出
```

### 前端开发模式（需要 Node.js）

```bash
# 终端 1：启动 Go 后端（:8080）
./HeadlessLive

# 终端 2：启动 Vite 开发服务器（:5173，自动代理 /api /preview /ws）
cd web
npm install
npm run dev
```

浏览器访问 `http://localhost:5173` 进行前端开发，API 请求会代理到后端 8080。

### 浏览器源（browser）

场景中加入浏览器源后，**启动预览或推流时会自动拉起 Xvfb 虚拟显示器 + 浏览器**，
无需手动启动；退出程序时自动清理。

> **安装浏览器（必须，推荐 deb 版，三选一）**
>
> 1. **Google Chrome**（与 Xvfb 兼容最好，Google 源可达时首选）：
>    ```bash
>    sudo apt install xvfb
>    wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo gpg --dearmor -o /usr/share/keyrings/google-chrome.gpg
>    echo "deb [arch=amd64 signed-by=/usr/share/keyrings/google-chrome.gpg] http://dl.google.com/linux/chrome/deb/ stable main" | sudo tee /etc/apt/sources.list.d/google-chrome.list
>    sudo apt update && sudo apt install google-chrome-stable
>    ```
> 2. **deb 版 Chromium（受限网络首选，Linux Mint 仓库）**：
>    新 Debian/Ubuntu 官方源已移除 chromium 的 deb 包，而 **Linux Mint 仓库提供真正的 deb 版 chromium**
>    （Mint 21 基于 Ubuntu 22.04，包可通用）。内网/受限网络无需 Google 源：
>    ```bash
>    sudo apt install xvfb
>    # 添加 Mint 仓库；内网服务器可直接 trusted=yes 跳过签名校验
>    echo "deb [arch=amd64 trusted=yes] http://packages.linuxmint.com victoria main upstream import" | sudo tee /etc/apt/sources.list.d/mint.list
>    # 限定只从 Mint 仓库装 chromium，防止覆盖系统其它包
>    printf 'Package: *\nPin: origin packages.linuxmint.com\nPin-Priority: 100\n' | sudo tee /etc/apt/preferences.d/mint-pin
>    sudo apt update && sudo apt install chromium
>    ```
>    装完后二进制位于 `/usr/bin/chromium`，程序自动探测。若想严格验证签名，可先导入新版公钥：
>    `gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys A6616109451BBBF2`
>    （旧版 `linuxmint-keyring`（2022 及以前）不含 2024 年更换的新公钥，会报 `NO_PUBKEY`）。
> 3. **snap 版 Chromium**（`sudo snap install chromium`，程序后备）：
>    程序会自动探测 `/snap/bin/chromium` 并作为后备使用，多数环境可正常渲染；
>    个别环境受 snap 沙箱限制可能黑屏，**优先 deb 版**。

浏览器源在 Web UI 中配置：**URL**（要打开的网页）、**渲染宽/高**（默认 1280×720）、
**帧率**（默认 30）；虚拟显示器默认 `:99`，也可在源属性中指定。

- 浏览器以 **kiosk 全屏模式**启动，无窗口边框/工具栏，x11grab 只捕获网页内容
- **切换网站**：修改源属性里的 URL，重新启动预览/推流即会自动重载浏览器
- **心率/数据插件**（如 hypertate）：把插件提供的网页 URL 填进浏览器源即可，
  例如 hypertate 的 OBS 浏览器源地址可直接使用

## 快速开始

> 📥 从 [最新 Release](https://github.com/T-Ming-L/HeadlessLive/releases/latest) 下载对应平台的单文件二进制，**无需自行编译**。

### Linux

```bash
# 1. 安装系统依赖（浏览器源另需 xvfb + Chrome，见「浏览器源」章节）
sudo apt install ffmpeg v4l-utils vainfo alsa-utils

# 2. 下载 Linux 单文件版（最新 Release）
wget https://github.com/T-Ming-L/HeadlessLive/releases/latest/download/HeadlessLive-linux-amd64
chmod +x HeadlessLive-linux-amd64

# 3. 启动（默认端口 8080）
./HeadlessLive-linux-amd64
```

### Windows

```powershell
# 1. 安装 FFmpeg 并加入 PATH（https://www.gyan.dev/ffmpeg/builds/）

# 2. 下载 Windows 单文件版（最新 Release）：
#    https://github.com/T-Ming-L/HeadlessLive/releases/latest/download/HeadlessLive-windows-amd64.exe

# 3. 启动（默认端口 8080）
.\HeadlessLive-windows-amd64.exe
```

首次运行自动生成 `config.yaml`（服务器配置）和 `scenes.yaml`（默认示例场景）。
浏览器访问 `http://服务器IP:8080` 打开控制台。

### 服务器配置（config.yaml）

```yaml
server:
  port: 8080 # HTTP 端口
  debug: false # 调试模式：true 输出详细 FFmpeg 日志
  log: true # 将日志写入 logs/ 目录文件
```

- `port` 也可用 `-port` 参数临时覆盖：`./HeadlessLive -port 9000`
- `debug: true` 时 FFmpeg 输出详细日志（方便排查推流/预览问题）
- `log: true` 时所有日志（含 FFmpeg 输出）写入 `logs/headlesslive-日期-时间.log`

### USB 声卡音频

采集音频默认走 **USB 声卡**（即插即用，如 Synido Voice 100），不依赖采集卡内嵌音频：

```yaml
# scenes.yaml 中的音频源
- id: src-mic
  name: USB 声卡
  type: audio_device
  enabled: true
  audio_device: usb # "usb" = 自动探测第一个 USB 声卡
  sample_rate: 48000
  channels: 2
```

- `audio_device: usb`（或留空）：运行时自动探测 USB 声卡，插拔后依然有效
- 也可手动指定设备名：`plughw:CARD=S100,DEV=0`（带软件转换，兼容性最好）或 `hw:CARD=S100,DEV=0`
- 设备列表通过 `GET /api/devices/audio` 获取（`arecord -L` 解析，USB 设备优先并标记）
- 查看声卡设备名：`arecord -L`

## 编译

需要 **Go 1.22+** 和 **Node.js 18+**（前端构建）。

```bash
# 一键构建（Windows 用 BUILD.bat）：
./build.sh          # 自动：npm build → go build → Linux 单文件

# 或手动：
cd web && npm install && npm run build && cd ..   # 前端 → static/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o HeadlessLive-linux-amd64 .      # Linux
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o HeadlessLive-windows-amd64.exe . # Windows
```

## API

| 方法                | 路径                                             | 说明                              |
| ------------------- | ------------------------------------------------ | --------------------------------- |
| GET/POST/PUT/DELETE | `/api/sources`                                 | 源 CRUD（`/api/sources/:id`）   |
| POST                | `/api/sources/:id/probe`                       | 探测源属性                        |
| GET/POST/PUT/DELETE | `/api/scenes`                                  | 场景 CRUD（`/api/scenes/:id`）  |
| POST                | `/api/scenes/:id/activate`                     | 切换当前场景                      |
| GET/POST/PUT/DELETE | `/api/outputs`                                 | 输出 CRUD（`/api/outputs/:id`） |
| POST                | `/api/outputs/:id/start`                       | 启动输出（推流/录制）             |
| POST                | `/api/outputs/:id/stop`                        | 停止输出                          |
| GET                 | `/api/status`                                  | 全量状态                          |
| POST                | `/api/preview/start` · `/stop`              | 预览控制                          |
| GET                 | `/preview`                                     | MJPEG 预览流                      |
| POST                | `/api/upload/image` · `/media`              | 上传素材                          |
| GET                 | `/api/devices/video` · `/audio`             | 扫描设备                          |
| GET/PUT             | `/api/devices/:dev/controls` · `/control`   | 设备控制项                        |
| GET                 | `/api/bili/qrcode` · `/poll` · `/status` | B 站扫码登录                      |
| GET/POST            | `/api/bili/areas` · `/start` · `/stop`   | B 站开播/停播/取推流密钥          |
| WS                  | `/ws`                                          | WebSocket（日志+状态）            |

## 项目结构

```
HeadlessLive
├── main.go                # 入口，embed 前端构建产物
├── config.yaml            # 服务器配置（端口/debug/log，自动生成）
├── scenes.yaml            # 源/场景/输出配置（自动生成）
├── web/                   # Vue 3 + Vite 前端源码
│   ├── vite.config.js     # dev 代理到 :8080，构建输出到 ../static
│   └── src/
│       ├── App.vue        # 三栏布局
│       ├── store.js       # 全局状态
│       ├── api.js / ws.js # REST / WebSocket 客户端
│       └── components/    # 源面板/画布/属性/底部栏/对话框
├── internal/
│   ├── api/               # REST API + WebSocket 路由
│   ├── capture/           # v4l2/ALSA 设备探测/控制
│   ├── config/            # 服务器配置（端口/debug/log）
│   ├── ffmpeg/            # 多输出进程管理 + MJPEG 预览
│   ├── logging/           # 文件日志（logs/）
│   ├── model/             # 三层数据模型
│   ├── scene/             # 场景 → 滤镜链构建器
│   ├── store/             # scenes.yaml 持久化
│   └── websocket/         # WebSocket Hub
├── logs/                  # 日志文件（log: true 时生成）
└── static/                # 前端构建产物（npm run build 生成）
```

## 路线图

- **Phase 1 ✅** 数据模型 + YAML 持久化 + 滤镜链构建 + 多输出管理
- **Phase 2 🚧** Vue3 Web UI：源管理 / 场景编辑 / MJPEG 预览 / 推流控制（前端已写完，待 Node 构建验证）
- **Phase 3** 场景切换过渡 / 音频混音器 / 统计面板 / 快捷键

## License

GPL-3.0
