// Package model 定义 HeadlessLive 的三层数据模型：源（Source）→ 场景（Scene）→ 输出（Output）。
package model

import "time"

// SourceType 源类型
type SourceType string

const (
	SourceVideoDevice SourceType = "video_device" // v4l2 采集卡
	SourceAudioDevice SourceType = "audio_device" // ALSA/PulseAudio
	SourceImage       SourceType = "image"        // 静态图片
	SourceText        SourceType = "text"         // 文字
	SourceMediaFile   SourceType = "media_file"   // 视频/音频文件
	SourceScreen      SourceType = "screen"       // 屏幕捕获 (X11)
	SourceRTMP        SourceType = "rtmp_source"  // RTMP 拉流
	SourceColor       SourceType = "color"        // 纯色背景
	SourceBrowser     SourceType = "browser"      // 浏览器网页 (Xvfb + Chromium + x11grab)
)

// IsVideoSource 判断源是否提供视频画面
func (s SourceType) IsVideoSource() bool {
	switch s {
	case SourceVideoDevice, SourceImage, SourceMediaFile, SourceScreen,
		SourceRTMP, SourceColor, SourceBrowser:
		return true
	}
	return false
}

// IsAudioSource 判断源是否提供音频
func (s SourceType) IsAudioSource() bool {
	switch s {
	case SourceAudioDevice, SourceMediaFile, SourceRTMP:
		return true
	}
	return false
}

// Filter 源级滤镜（crop, scale, rotate, color_correct, noise_reduction, deinterlace...）
type Filter struct {
	Type   string                 `json:"type" yaml:"type"`
	Params map[string]interface{} `json:"params" yaml:"params"`
}

// Source 一个独立的音视频输入
type Source struct {
	ID      string     `json:"id" yaml:"id"`
	Name    string     `json:"name" yaml:"name"`
	Type    SourceType `json:"type" yaml:"type"`
	Enabled bool       `json:"enabled" yaml:"enabled"`

	// --- 采集卡专用 ---
	DevicePath  string `json:"device_path,omitempty" yaml:"device_path,omitempty"`
	PixelFormat string `json:"pixel_format,omitempty" yaml:"pixel_format,omitempty"` // yuyv422, mjpeg, nv12
	Resolution  string `json:"resolution,omitempty" yaml:"resolution,omitempty"`     // "1920x1080"
	FPS         int    `json:"fps,omitempty" yaml:"fps,omitempty"`
	ColorSpace  string `json:"color_space,omitempty" yaml:"color_space,omitempty"` // bt709, bt601, smpte170m

	// --- 音频专用 ---
	AudioDevice string  `json:"audio_device,omitempty" yaml:"audio_device,omitempty"`
	SampleRate  int     `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
	Channels    int     `json:"channels,omitempty" yaml:"channels,omitempty"`
	Volume      float64 `json:"volume,omitempty" yaml:"volume,omitempty"` // 1.0 = 原始

	// --- 图片/文字/媒体 ---
	FilePath  string `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	Text      string `json:"text,omitempty" yaml:"text,omitempty"`
	FontSize  int    `json:"font_size,omitempty" yaml:"font_size,omitempty"`
	FontColor string `json:"font_color,omitempty" yaml:"font_color,omitempty"`
	Loop      bool   `json:"loop,omitempty" yaml:"loop,omitempty"`

	// --- 浏览器源 ---
	URL        string `json:"url,omitempty" yaml:"url,omitempty"`
	BrowserW   int    `json:"browser_w,omitempty" yaml:"browser_w,omitempty"`     // 渲染宽，默认 1280
	BrowserH   int    `json:"browser_h,omitempty" yaml:"browser_h,omitempty"`     // 渲染高，默认 720
	BrowserFPS int    `json:"browser_fps,omitempty" yaml:"browser_fps,omitempty"` // 捕获帧率，默认 30

	// --- 屏幕捕获 ---
	Display string `json:"display,omitempty" yaml:"display,omitempty"` // X11 DISPLAY，默认 :0

	// --- 纯色背景 ---
	Color string `json:"color,omitempty" yaml:"color,omitempty"` // 如 "0x2E2E2E" 或 "#RRGGBB"

	// --- 通用属性 ---
	Filters []Filter `json:"filters,omitempty" yaml:"filters,omitempty"`
}

// SceneItem 场景中的一个源实例（按 Z 轴叠放）
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

// Scene 一个场景 = 一组源的排列
type Scene struct {
	ID      string      `json:"id" yaml:"id"`
	Name    string      `json:"name" yaml:"name"`
	Items   []SceneItem `json:"items" yaml:"items"`
	CanvasW int         `json:"canvas_w" yaml:"canvas_w"`
	CanvasH int         `json:"canvas_h" yaml:"canvas_h"`
	FPS     int         `json:"fps" yaml:"fps"`
}

// OutputType 输出类型
type OutputType string

const (
	OutputRTMP   OutputType = "rtmp"
	OutputRecord OutputType = "record"
	OutputSRT    OutputType = "srt"
	OutputNDI    OutputType = "ndi"
)

// Output 一个场景可以推送到一个或多个输出
type Output struct {
	ID      string     `json:"id" yaml:"id"`
	Name    string     `json:"name" yaml:"name"`
	Type    OutputType `json:"type" yaml:"type"`
	SceneID string     `json:"scene_id" yaml:"scene_id"` // 绑定的场景
	Enabled bool       `json:"enabled" yaml:"enabled"`

	// RTMP/SRT
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	StreamKey string `json:"stream_key,omitempty" yaml:"stream_key,omitempty"`

	// 编码
	Encoder  string `json:"encoder" yaml:"encoder"`   // h264_vaapi, h264_qsv, libx264, hevc_vaapi
	Bitrate  int    `json:"bitrate" yaml:"bitrate"`   // kbps
	KeyFrame int    `json:"keyframe" yaml:"keyframe"` // GOP 间隔（帧数）

	// 录制
	FilePath string `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	Format   string `json:"format,omitempty" yaml:"format,omitempty"` // mp4, mkv, flv

	// 运行时状态（不持久化到 YAML）
	Running bool `json:"running" yaml:"-"`
}

// Store 整个配置集合（对应 scenes.yaml）
type Store struct {
	Sources       []*Source `json:"sources" yaml:"sources"`
	Scenes        []*Scene  `json:"scenes" yaml:"scenes"`
	Outputs       []*Output `json:"outputs" yaml:"outputs"`
	CurrentScene  string    `json:"current_scene" yaml:"current_scene"`
	UpdatedAt     time.Time `json:"updated_at" yaml:"updated_at"`
}

// FindSource 按 ID 查找源
func (s *Store) FindSource(id string) *Source {
	for _, src := range s.Sources {
		if src.ID == id {
			return src
		}
	}
	return nil
}

// FindScene 按 ID 查找场景
func (s *Store) FindScene(id string) *Scene {
	for _, sc := range s.Scenes {
		if sc.ID == id {
			return sc
		}
	}
	return nil
}

// FindOutput 按 ID 查找输出
func (s *Store) FindOutput(id string) *Output {
	for _, o := range s.Outputs {
		if o.ID == id {
			return o
		}
	}
	return nil
}

// GetCurrentScene 获取当前活动场景
func (s *Store) GetCurrentScene() *Scene {
	if s.CurrentScene != "" {
		if sc := s.FindScene(s.CurrentScene); sc != nil {
			return sc
		}
	}
	if len(s.Scenes) > 0 {
		return s.Scenes[0]
	}
	return nil
}
