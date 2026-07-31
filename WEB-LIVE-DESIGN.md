# Web-Live — Web 版 OBS for Linux

## 设计思路

核心思想：**源（Source）→ 场景（Scene）→ 输出（Output）** 三层模型，对标 OBS。

---

## 一、数据模型

### 1. 源（Source）

一个源是一个独立的音视频输入，有自己的配置。

```go
type SourceType string
const (
    SourceVideoDevice  SourceType = "video_device"   // v4l2 采集卡
    SourceAudioDevice  SourceType = "audio_device"   // ALSA/PulseAudio
    SourceImage        SourceType = "image"          // 静态图片
    SourceText         SourceType = "text"           // 文字
    SourceMediaFile    SourceType = "media_file"     // 视频/音频文件
    SourceScreen       SourceType = "screen"         // 屏幕捕获 (X11/PipeWire)
    SourceRTMP         SourceType = "rtmp_source"    // RTMP 拉流
    SourceColor        SourceType = "color"          // 纯色背景
    SourceBrowser      SourceType = "browser"        // 浏览器网页 (Xvfb + Chromium + x11grab)
)

type Source struct {
    ID       string     `json:"id" yaml:"id"`
    Name     string     `json:"name" yaml:"name"`
    Type     SourceType `json:"type" yaml:"type"`
    Enabled  bool       `json:"enabled" yaml:"enabled"`

    // 采集卡专用
    DevicePath  string `json:"device_path,omitempty" yaml:"device_path,omitempty"`
    PixelFormat string `json:"pixel_format,omitempty" yaml:"pixel_format,omitempty"` // yuyv422, mjpeg
    Resolution  string `json:"resolution,omitempty" yaml:"resolution,omitempty"`     // "1920x1080"
    FPS         int    `json:"fps,omitempty" yaml:"fps,omitempty"`
    ColorSpace  string `json:"color_space,omitempty" yaml:"color_space,omitempty"`

    // 音频专用
    AudioDevice  string  `json:"audio_device,omitempty" yaml:"audio_device,omitempty"`
    SampleRate   int     `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
    Channels     int     `json:"channels,omitempty" yaml:"channels,omitempty"`
    Volume       float64 `json:"volume,omitempty" yaml:"volume,omitempty"` // 1.0 = 原始

    // 图片/文字/媒体
    FilePath  string `json:"file_path,omitempty" yaml:"file_path,omitempty"`
    Text      string `json:"text,omitempty" yaml:"text,omitempty"`
    FontSize  int    `json:"font_size,omitempty" yaml:"font_size,omitempty"`
    FontColor string `json:"font_color,omitempty" yaml:"font_color,omitempty"`
    Loop      bool   `json:"loop,omitempty" yaml:"loop,omitempty"`

    // 浏览器源
    URL       string `json:"url,omitempty" yaml:"url,omitempty"`
    BrowserW  int    `json:"browser_w,omitempty" yaml:"browser_w,omitempty"` // 渲染宽（默认 1280）
    BrowserH  int    `json:"browser_h,omitempty" yaml:"browser_h,omitempty"` // 渲染高（默认 720）
    BrowserFPS int   `json:"browser_fps,omitempty" yaml:"browser_fps,omitempty"` // 捕获帧率（默认 30）

    // 通用属性（音视频源都可设置）
    Filters []Filter `json:"filters,omitempty" yaml:"filters,omitempty"`
}

type Filter struct {
    Type   string                 `json:"type"`   // crop, scale, rotate, color_correct, noise_reduction, deinterlace
    Params map[string]interface{} `json:"params"`
}
```

### 2. 场景（Scene）

一个场景 = 一组源的排列，按 Z 轴叠放。

```go
type Scene struct {
    ID      string       `json:"id" yaml:"id"`
    Name    string       `json:"name" yaml:"name"`
    Items   []SceneItem  `json:"items" yaml:"items"`
    CanvasW int          `json:"canvas_w" yaml:"canvas_w"` // 画布分辨率
    CanvasH int          `json:"canvas_h" yaml:"canvas_h"`
    FPS     int          `json:"fps" yaml:"fps"`
}

type SceneItem struct {
    SourceID string  `json:"source_id" yaml:"source_id"` // 引用 Source.ID
    X        int     `json:"x" yaml:"x"`
    Y        int     `json:"y" yaml:"y"`
    Width    int     `json:"width" yaml:"width"`
    Height   int     `json:"height" yaml:"height"`
    CropX    int     `json:"crop_x,omitempty" yaml:"crop_x,omitempty"`
    CropY    int     `json:"crop_y,omitempty" yaml:"crop_y,omitempty"`
    CropW    int     `json:"crop_w,omitempty" yaml:"crop_w,omitempty"`
    CropH    int     `json:"crop_h,omitempty" yaml:"crop_h,omitempty"`
    Opacity  float64 `json:"opacity" yaml:"opacity"` // 0.0 ~ 1.0
    ZIndex   int     `json:"z_index" yaml:"z_index"`
    Visible  bool    `json:"visible" yaml:"visible"`
}
```

### 3. 输出（Output）

一个场景可以推送到一个或多个输出。

```go
type OutputType string
const (
    OutputRTMP    OutputType = "rtmp"
    OutputRecord  OutputType = "record"   // 本地录制
    OutputSRT     OutputType = "srt"
    OutputNDI     OutputType = "ndi"
)

type Output struct {
    ID        string     `json:"id" yaml:"id"`
    Name      string     `json:"name" yaml:"name"`
    Type      OutputType `json:"type" yaml:"type"`
    SceneID   string     `json:"scene_id" yaml:"scene_id"` // 绑定的场景
    Enabled   bool       `json:"enabled" yaml:"enabled"`

    // RTMP
    URL       string `json:"url,omitempty" yaml:"url,omitempty"`
    StreamKey string `json:"stream_key,omitempty" yaml:"stream_key,omitempty"`

    // 编码
    Encoder   string `json:"encoder" yaml:"encoder"`        // h264_vaapi, h264_qsv, libx264, hevc_vaapi
    Bitrate   int    `json:"bitrate" yaml:"bitrate"`        // kbps
    KeyFrame  int    `json:"keyframe" yaml:"keyframe"`      // GOP 间隔（帧数）

    // 录制
    FilePath  string `json:"file_path,omitempty" yaml:"file_path,omitempty"`
    Format    string `json:"format,omitempty" yaml:"format,omitempty"` // mp4, mkv, flv
}
```

---

## 二、核心架构

```
┌─────────────────────────────────────────────────┐
│                   Web UI (Vue 3)                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────────────┐ │
│  │ 源管理   │ │ 场景编辑 │ │ 输出 + 流控制     │ │
│  │ (列表)   │ │ (画布+   │ │ (开始/停止/      │ │
│  │          │ │  属性栏) │ │  录制/统计)       │ │
│  └──────────┘ └──────────┘ └──────────────────┘ │
├─────────────────────────────────────────────────┤
│              REST API + WebSocket                │
├─────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────┐    │
│  │           场景引擎 (Scene Engine)        │    │
│  │  ┌──────┐ ┌──────┐ ┌──────────────────┐ │    │
│  │  │ 源   │ │ 混合 │ │ FFmpeg 滤镜链    │ │    │
│  │  │管理  │→│ 器   │→│ 构建器           │→│    │
│  │  └──────┘ └──────┘ └──────────────────┘ │    │
│  └─────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────┐    │
│  │           输出管理 (Output Manager)      │    │
│  │  ┌──────┐ ┌──────┐ ┌──────┐            │    │
│  │  │ RTMP │ │ 录制 │ │ SRT  │ ...        │    │
│  │  └──────┘ └──────┘ └──────┘            │    │
│  └─────────────────────────────────────────┘    │
├─────────────────────────────────────────────────┤
│              FFmpeg (子进程)                     │
│  输入 → 滤镜链 → 编码 → flv/mp4/... → 输出     │
├─────────────────────────────────────────────────┤
│              v4l2 / ALSA / X11 / PipeWire       │
└─────────────────────────────────────────────────┘
```

### 关键技术点

1. **FFmpeg 滤镜链动态构建**：根据场景中的源及其属性，动态生成 `-filter_complex`
   - 视频源: `[0:v]scale=...,crop=...[v0]`
   - 图片源: `-loop 1 -i logo.png` → `[1:v]scale=...[ov1]`
   - 叠加: `[v0][ov1]overlay=x:y:alpha=...[vout]`
   - 音频: `[0:a]volume=...[1:a]volume=...; [...]amix=...[aout]`

2. **配置持久化**：`scenes.yaml` 存储所有场景/源/输出配置
   - Web UI 编辑 → 保存到 YAML → 热加载或提示重启
   - 支持导入导出（类似 OBS 的场景集合）

3. **预览**：
   - Canvas 分辨率 MJPEG 预览流（通过独立 FFmpeg 进程）
   - 预览使用相同的滤镜链但输出为 MJPEG 而非编码流
   - 支持低分辨率代理预览（节省带宽）

4. **音频混音**：
   - 多个音频源 → `amix` / `amerge` 滤镜混合
   - 每个音频源独立音量控制
   - 支持音频延迟补偿（lip-sync）

5. **场景切换**：
   - 平滑过渡（crossfade）由 FFmpeg `blend` / `fade` 滤镜实现
   - 或即时切换（stop → restart with new filter）

6. **浏览器源（browser）**：Linux 下用虚拟 X 显示器 + Chromium 渲染，再由 FFmpeg `x11grab` 捕获（对标 OBS 的 CEF Browser Source，但走系统管线）：

   ```
   Xvfb :99 -screen 0 1280x720x24
   DISPLAY=:99 chromium --no-sandbox --disable-gpu --hide-scrollbars \
       --window-size=1280x720 --autoplay-policy=no-user-gesture-required <URL>
   ffmpeg -f x11grab -framerate 30 -video_size 1280x720 -i :99 ...
   ```

   - 与 screen 源共用 `x11grab` 捕获逻辑，仅 DISPLAY 指向虚拟屏
   - 渲染分辨率（BrowserW/H）、FPS 独立可配，与场景画布无关，进滤镜链后按 SceneItem 缩放叠加
   - 无音频：网页音频走 `pactl load-module module-null-sink` 或 PulseAudio 环回（Phase 3 可选）

---

## 三、API 设计

```
GET    /api/sources              → 列出所有源
POST   /api/sources              → 创建源
PUT    /api/sources/:id          → 更新源
DELETE /api/sources/:id          → 删除源
POST   /api/sources/:id/probe    → 探测源属性（分辨率、格式等）

GET    /api/scenes               → 列出所有场景
POST   /api/scenes               → 创建场景
PUT    /api/scenes/:id           → 更新场景（含场景项）
DELETE /api/scenes/:id           → 删除场景

GET    /api/outputs              → 列出所有输出
POST   /api/outputs              → 创建输出
PUT    /api/outputs/:id          → 更新输出
DELETE /api/outputs/:id          → 删除输出
POST   /api/outputs/:id/start    → 启动输出（推流/录制）
POST   /api/outputs/:id/stop     → 停止输出

GET    /api/status               → 系统状态（当前场景、运行中的输出、FPS、码率）
GET    /preview?scene=xxx        → MJPEG 预览流（按场景）
WS     /ws                       → WebSocket（日志+状态推送）

POST   /api/upload/image         → 上传图片
POST   /api/upload/media         → 上传媒体文件

GET    /api/devices/video        → 扫描 v4l2 设备
GET    /api/devices/audio        → 扫描音频设备
GET    /api/devices/:dev/controls → 获取设备控制项
PUT    /api/devices/:dev/control  → 设置设备控制项
```

---

## 四、前端布局

```
┌─────────────┬───────────────────────┬──────────────┐
│  源 面板    │     场景画布          │  属性 面板   │
│             │  ┌───────────────┐    │              │
│ 📹 采集卡   │  │               │    │ 选中项属性:  │
│ 🔊 音频     │  │   960x540     │    │ 位置 X/Y    │
│ 🖼 logo     │  │   MJPEG预览   │    │ 尺寸 W/H    │
│ 📝 文字     │  │               │    │ 裁剪        │
│             │  └───────────────┘    │ 透明度      │
│ [+ 添加源]  │                       │ 层级        │
│             │  缩放: [====○====]    │              │
├─────────────┴───────────────────────┴──────────────┤
│  底部栏: 场景选择 | 输出控制 | FPS/Bitrate/时长     │
└─────────────────────────────────────────────────────┘
```

左侧面板：树形结构的源列表，可拖拽到场景，可开关显示。
中间：MJPEG 实时预览 + 元素拖拽定位（坐标联动）。
右侧：选中场景项时的属性编辑。
底部栏：场景切换标签、推流/录制按钮、状态信息。

---

## 五、实现计划

### Phase 1：核心引擎

- 源/场景/输出的 Go 数据结构 + YAML 存取
- 场景 → FFmpeg 滤镜链转换器
- FFmpeg 进程管理（启动/停止/重启）

### Phase 2：基础 Web UI

- 源管理 CRUD
- 场景编辑（位置/裁剪/层级）
- 预览（MJPEG）
- 输出控制（RTMP 推流/停止）

### Phase 3：高级功能

- 场景切换过渡
- 音频混音器
- 录制功能
- 统计面板（码率/丢帧/CPU）
- 快捷键支持

---

## 六、与当前 HeadlessLive 的差异

|           | HeadlessLive（当前） | Web-Live（新项目）    |
| --------- | -------------------- | --------------------- |
| 模型      | 以采集卡为中心       | 源→场景→输出三层      |
| 裁剪/叠加 | 全局滤镜参数         | 每源每场景独立        |
| 音频      | 单一 ALSA 设备       | 多源混音              |
| 配置      | 全局 config.yaml     | 场景 ID 化的 YAML     |
| 设备设置  | 全局固定             | 每源独立像素格式/色彩 |
| 多场景    | ❌                   | ✅                    |
| 录制      | ❌                   | ✅                    |
| 过渡效果  | ❌                   | ✅                    |

---

## 七、技术选型

- 后端：Go + Gin + gorilla/websocket + FFmpeg 子进程
- 配置：gopkg.in/yaml.v3（同当前项目）
- 前端：Vue 3 + Vite（交互复杂度上来了，vanilla JS 扛不住）
- 预览：MJPEG 流（同当前方案）
- 编译：CGO_ENABLED=0 交叉编译 x86_64 Linux 单文件
