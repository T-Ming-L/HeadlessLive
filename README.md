# HeadlessLive

> 无头直播演播室（Headless OBS）—— 部署在 Linux 服务器上、无界面运行的直播工具，
> 全部通过 **Web 浏览器远程控制**，采用 **源（Source）→ 场景（Scene）→ 输出（Output）**
> 三层模型，对标 OBS。内置 **B 站直播助手**（扫码登录 → 一键开播 → 自动填入推流密钥），
> 也支持通用 RTMP / SRT 推流与本地录制。

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
- ✅ **单文件部署**：前端构建产物 embed 进 Go 二进制，一个文件搞定
- ✅ **B 站直播助手**：扫码登录 → 一键开播获取推流地址/密钥 → 自动填入输出配置（RTMP 推流到 B 站）

## 环境要求

### 运行环境（Linux 服务器）

| 依赖         | 版本/说明                         | 用途                         |
| ------------ | --------------------------------- | ---------------------------- |
| Linux        | x86_64（amd64），推荐 Debian/Ubuntu | 部署目标；N100 等小主机即可  |
| FFmpeg       | 4.x+（含 v4l2 / alsa / vaapi 支持） | 采集、滤镜合成、编码、推流   |
| v4l-utils    | 任意                              | 采集卡设备探测（`v4l2-ctl`） |
| alsa-utils   | 任意                              | 声卡设备探测（`arecord`）    |
| vainfo       | 任意                              | Intel VAAPI 硬编探测         |
| Xvfb + Chromium | 可选，仅「浏览器源」需要       | 浏览器源渲染与捕获           |

```bash
sudo apt install ffmpeg v4l-utils vainfo alsa-utils
# 浏览器源（可选）：
sudo apt install xvfb chromium
```

> 硬件：推荐 Intel 核显（VAAPI 硬编，低 CPU 占用）；采集用 USB 采集卡（UVC）+ USB 声卡（如 Synido Voice 100）。

### 编译环境（可选 —— 直接下载 Release 二进制则不需要）

| 依赖    | 版本      | 用途               |
| ------- | --------- | ------------------ |
| Go      | 1.22+     | 后端编译           |
| Node.js | 18+       | 仅构建前端（`web/`）|

> Windows 仅用于本地编译与体验；正式部署目标为 Linux 服务器。

## 依赖与引用

### Go 后端（`go.mod`）

| 库                                        | 版本     | 用途                          |
| ----------------------------------------- | -------- | ----------------------------- |
| `github.com/gin-gonic/gin`                | v1.8.1   | HTTP 路由 / REST API          |
| `github.com/gorilla/websocket`            | v1.5.1   | WebSocket 日志与状态推送      |
| `gopkg.in/yaml.v3`                        | v3.0.1   | `scenes.yaml` 配置持久化      |

### 前端（`web/`）

| 库     | 版本     | 用途                     |
| ------ | -------- | ------------------------ |
| Vue    | 3.x      | 前端框架                 |
| Vite   | 5.x      | 构建工具                 |
| qrcode | ^1.5.4   | B 站扫码登录二维码生成   |

### 系统级依赖

- **FFmpeg**：核心引擎（采集 → 滤镜 → 编码 → 推流）
- **Chromium + Xvfb**：浏览器源（可选）

### 参考项目

- [GamerNoTitle/BiliLive-Utility](https://github.com/GamerNoTitle/BiliLive-Utility)（AGPL-3.0）—— 本项目的 `internal/bilibili` 按其公开的 **B 站直播接口协议**（扫码登录 / 开播 / 取推流密钥）独立编写的 Go 实现，未引用其源代码。

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

Linux 下用虚拟 X 显示器 + Chromium 渲染，FFmpeg `x11grab` 捕获：

```bash
Xvfb :99 -screen 0 1280x720x24
DISPLAY=:99 chromium --no-sandbox --disable-gpu --hide-scrollbars \
    --window-size=1280x720 --autoplay-policy=no-user-gesture-required <URL>
```

## 快速开始

### 1. 安装依赖（Linux 服务器）

```bash
sudo apt install ffmpeg v4l-utils vainfo alsa-utils
# 浏览器源需要：
sudo apt install xvfb chromium
```

### 2. 部署

```bash
scp HeadlessLive root@your-server:/opt/headlesslive/
ssh root@your-server
cd /opt/headlesslive
chmod +x HeadlessLive
./HeadlessLive
```

首次运行自动生成 `config.yaml`（服务器配置）和 `scenes.yaml`（默认示例场景）。

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

### 3. 打开控制台

浏览器访问 `http://服务器IP:8080`。左侧源列表、中间场景画布（MJPEG 预览）、右侧属性面板、底部推流控制。

## 编译

需要 **Go 1.22+** 和 **Node.js 18+**（前端构建）。

```bash
# 一键构建（Windows 用 BUILD.bat）：
./build.sh          # 自动：npm build → go build → HeadlessLive

# 或手动：
cd web && npm install && npm run build && cd ..   # 前端 → static/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o HeadlessLive .
```

## API

| 方法                | 路径                                      | 说明                            |
| ------------------- | ----------------------------------------- | ------------------------------- |
| GET/POST/PUT/DELETE | `/api/sources`                            | 源 CRUD（`/api/sources/:id`）   |
| POST                | `/api/sources/:id/probe`                  | 探测源属性                      |
| GET/POST/PUT/DELETE | `/api/scenes`                             | 场景 CRUD（`/api/scenes/:id`）  |
| POST                | `/api/scenes/:id/activate`                | 切换当前场景                    |
| GET/POST/PUT/DELETE | `/api/outputs`                            | 输出 CRUD（`/api/outputs/:id`） |
| POST                | `/api/outputs/:id/start`                  | 启动输出（推流/录制）           |
| POST                | `/api/outputs/:id/stop`                   | 停止输出                        |
| GET                 | `/api/status`                             | 全量状态                        |
| POST                | `/api/preview/start` · `/stop`            | 预览控制                        |
| GET                 | `/preview`                                | MJPEG 预览流                    |
| POST                | `/api/upload/image` · `/media`            | 上传素材                        |
| GET                 | `/api/devices/video` · `/audio`           | 扫描设备                        |
| GET/PUT             | `/api/devices/:dev/controls` · `/control` | 设备控制项                      |
| GET                 | `/api/bili/qrcode` · `/poll` · `/status`  | B 站扫码登录                    |
| GET/POST            | `/api/bili/areas` · `/start` · `/stop`    | B 站开播/停播/取推流密钥        |
| WS                  | `/ws`                                     | WebSocket（日志+状态）          |

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
