// Package scene 将"场景 → 源"组合转换为 FFmpeg 渲染规格：
// 输入参数列表 + filter_complex 滤镜链，供推流/预览/录制共用。
package scene

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/T-Ming-L/HeadlessLive/internal/capture"
	"github.com/T-Ming-L/HeadlessLive/internal/model"
)

// ErrDeviceUnavailable 设备/文件不可用（采集卡未连接、声卡未插入、文件缺失）。
// Build 时跳过此类源，避免单个输入失败导致整个 FFmpeg 进程退出。
var ErrDeviceUnavailable = errors.New("设备不可用")

// InputKind FFmpeg 输入方式
type InputKind string

const (
	InputV4L2   InputKind = "v4l2"
	InputImage  InputKind = "image"
	InputColor  InputKind = "color"
	InputFile   InputKind = "file"
	InputX11    InputKind = "x11grab"
	InputRTMP   InputKind = "rtmp"
	InputALSA   InputKind = "alsa"
	InputPulse  InputKind = "pulse"
	InputText   InputKind = "text"
)

// 文字源渲染画布尺寸（场景内会被等比缩放铺满场景项，字体大小以此画布为基准）
const (
	textCanvasW = 1280
	textCanvasH = 720
)

// Input 一个 FFmpeg 输入
type Input struct {
	Index   int           // -i 顺序索引（滤镜标签用）
	Kind    InputKind     // 输入方式
	Source  *model.Source // 对应的源
	HasVideo bool
	HasAudio bool
	// 输入参数（-i 之前）
	params []string
	path   string
}

// Label 返回滤镜标签，如 "0:v"
func (in *Input) Label(stream string) string {
	return fmt.Sprintf("[%d:%s]", in.Index, stream)
}

// FFmpegArgs 返回该输入完整的命令行参数。
// 若 path 为空，则 params 已内含 "-i ..."（如 lavfi color、pulse），直接返回。
func (in *Input) FFmpegArgs() []string {
	if in.path == "" {
		return in.params
	}
	args := append([]string{}, in.params...)
	return append(args, "-i", in.path)
}

// RenderSpec 场景渲染规格
type RenderSpec struct {
	CanvasW  int
	CanvasH  int
	FPS      int
	Inputs   []*Input
	Filter   string // filter_complex 字符串；空表示无需滤镜，直接输出输入流
	VideoOut string // 输出视频标签，如 "[vout]"
	AudioOut string // 输出音频标签，如 "[aout]"，无音频则为 ""
	Skipped  []string // 因设备不可用被跳过的源（供提示）
}

// VideoOnlyFilter 返回仅视频的滤镜链（剔除音频 volume/amix 部分）。
// 预览进程只输出视频，若 filter_complex 含有未连接的音频滤镜（如 [1:a] volume=..[a0]），
// ffmpeg 会报 "Filter ... has an unconnected output" 导致预览失败，因此预览必须使用此方法。
func (rs *RenderSpec) VideoOnlyFilter() string {
	if rs.Filter == "" {
		return ""
	}
	parts := strings.Split(rs.Filter, ";")
	video := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 跳过音频链：amix 混音 或 以 [N:a] 音频输入开头
		if strings.Contains(p, "amix=") {
			continue
		}
		if strings.HasPrefix(p, "[") && strings.Contains(p, ":a]") {
			continue
		}
		video = append(video, p)
	}
	return strings.Join(video, ";")
}

// 输入索引 -> 源 ID，避免同一源重复打开
type sourceKey struct {
	id  string
	idx int
}

// Build 将场景 + 源集合编译为渲染规格
func Build(scene *model.Scene, srcs map[string]*model.Source) (*RenderSpec, error) {
	if scene == nil {
		return nil, fmt.Errorf("场景为空")
	}
	w, h, fps := scene.CanvasW, scene.CanvasH, scene.FPS
	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}
	if fps <= 0 {
		fps = 30
	}

	rs := &RenderSpec{CanvasW: w, CanvasH: h, FPS: fps}

	// 1. 收集可见项，按 ZIndex 升序（先渲染的在底层）
	visible := make([]model.SceneItem, 0, len(scene.Items))
	for _, it := range scene.Items {
		if !it.Visible {
			continue
		}
		visible = append(visible, it)
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return visible[i].ZIndex < visible[j].ZIndex
	})

	// 2. 建立输入映射（源 → 输入索引）
	used := make(map[string]int) // sourceID -> input index
	skipped := make(map[string]bool)
	videoInputs := make([]model.SceneItem, 0)

	for _, it := range visible {
		src := srcs[it.SourceID]
		if src == nil || !src.Enabled {
			continue
		}
		if !src.Type.IsVideoSource() {
			continue
		}
		if _, ok := used[src.ID]; !ok {
			in, err := buildVideoInput(src)
			if err != nil {
				if errors.Is(err, ErrDeviceUnavailable) {
					if !skipped[src.ID] {
						skipped[src.ID] = true
						rs.Skipped = append(rs.Skipped, fmt.Sprintf("%s（%v）", src.Name, err))
					}
					continue
				}
				return nil, fmt.Errorf("源 %s: %w", src.Name, err)
			}
			in.Index = len(rs.Inputs)
			rs.Inputs = append(rs.Inputs, in)
			used[src.ID] = in.Index
		}
		videoInputs = append(videoInputs, it)
	}

	// 3. 音频输入（场景里可见的音频源，或混入视频源的音频）
	audioInputs := make([]*Input, 0)
	for _, src := range srcs {
		if src == nil || !src.Enabled || !src.Type.IsAudioSource() {
			continue
		}
		if _, ok := used[src.ID]; ok {
			// 已有视频输入（如 media_file），直接复用其音频流
			continue
		}
		in, err := buildAudioInput(src)
		if err != nil {
			if errors.Is(err, ErrDeviceUnavailable) {
				if !skipped[src.ID] {
					skipped[src.ID] = true
					rs.Skipped = append(rs.Skipped, fmt.Sprintf("%s（%v）", src.Name, err))
				}
				continue
			}
			return nil, fmt.Errorf("音频源 %s: %w", src.Name, err)
		}
		in.Index = len(rs.Inputs)
		rs.Inputs = append(rs.Inputs, in)
		used[src.ID] = in.Index
		audioInputs = append(audioInputs, in)
	}

	// 4. 构建 filter_complex
	if err := rs.buildFilter(w, h, fps, videoInputs, srcs, used, audioInputs); err != nil {
		return nil, err
	}

	return rs, nil
}

// buildVideoInput 为视频源构建 FFmpeg 输入。
// 设备/文件不存在时返回 ErrDeviceUnavailable（Build 会跳过该源）。
func buildVideoInput(src *model.Source) (*Input, error) {
	in := &Input{Source: src, HasVideo: true}
	switch src.Type {
	case model.SourceVideoDevice:
		dev := src.DevicePath
		if dev == "" {
			dev = "/dev/video0"
		}
		if _, err := os.Stat(dev); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrDeviceUnavailable, dev)
		}
		fps := src.FPS
		if fps <= 0 {
			fps = 30
		}
		res := src.Resolution
		if res == "" {
			res = "1920x1080"
		}
		in.Kind = InputV4L2
		in.params = []string{"-f", "v4l2", "-framerate", strconv.Itoa(fps),
			"-video_size", res, "-input_format", src.PixelFormat}
		if src.PixelFormat == "" {
			in.params = []string{"-f", "v4l2", "-framerate", strconv.Itoa(fps), "-video_size", res}
		}
		// 色彩空间（BT.709 / BT.601 / SMPTE 170M）
		if src.ColorSpace != "" {
			in.params = append(in.params,
				"-colorspace", src.ColorSpace,
				"-color_primaries", src.ColorSpace,
				"-color_trc", src.ColorSpace)
		}
		in.path = dev

	case model.SourceImage:
		if src.FilePath == "" {
			return nil, fmt.Errorf("%w: 图片源缺少文件路径", ErrDeviceUnavailable)
		}
		if _, err := os.Stat(src.FilePath); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrDeviceUnavailable, src.FilePath)
		}
		in.Kind = InputImage
		in.params = []string{"-loop", "1"}
		in.path = src.FilePath

	case model.SourceText:
		if src.Text == "" {
			return nil, fmt.Errorf("%w: 文字源缺少内容", ErrDeviceUnavailable)
		}
		// drawtext 需要中文字体文件（font=Sans 只会匹配英文 DejaVu，中文变方块）
		font, err := findCJKFont()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDeviceUnavailable, err)
		}
		// 用 lavfi 生成透明画布 + drawtext 文字层
		// （textfile 传内容避免转义，画布 1280x720，场景内等比缩放铺满场景项）
		tf, err := writeDrawTextFile(src.Text)
		if err != nil {
			return nil, fmt.Errorf("文字源: %w", err)
		}
		fs := src.FontSize
		if fs <= 0 {
			fs = 48
		}
		fc := src.FontColor
		if fc == "" {
			fc = "white"
		}
		in.Kind = InputText
		in.params = []string{"-f", "lavfi", "-i", fmt.Sprintf(
			"color=c=black@0.0:s=%dx%d:r=30,drawtext=fontfile='%s':textfile='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=(h-text_h)/2",
			textCanvasW, textCanvasH, font, tf, fs, fc)}
		in.path = ""

	case model.SourceColor:
		// 用 lavfi 生成纯色，尺寸任意（场景内会被 scale 到目标大小）
		color := src.Color
		if color == "" {
			color = "black"
		}
		in.Kind = InputColor
		in.params = []string{"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=%s:s=640x360:r=30", color)}
		in.path = "" // params 已含 -i

	case model.SourceMediaFile:
		if src.FilePath == "" {
			return nil, fmt.Errorf("%w: 媒体文件源缺少文件路径", ErrDeviceUnavailable)
		}
		if _, err := os.Stat(src.FilePath); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrDeviceUnavailable, src.FilePath)
		}
		in.Kind = InputFile
		in.HasAudio = true
		if src.Loop {
			in.params = []string{"-stream_loop", "-1"}
		}
		in.path = src.FilePath

	case model.SourceScreen:
		disp := src.Display
		if disp == "" {
			disp = ":0"
		}
		bw, bh, bfps := src.BrowserW, src.BrowserH, src.BrowserFPS
		if bw <= 0 {
			bw = 1280
		}
		if bh <= 0 {
			bh = 720
		}
		if bfps <= 0 {
			bfps = 30
		}
		in.Kind = InputX11
		in.params = []string{"-f", "x11grab", "-framerate", strconv.Itoa(bfps),
			"-video_size", fmt.Sprintf("%dx%d", bw, bh)}
		in.path = disp

	case model.SourceRTMP:
		if src.FilePath == "" {
			return nil, fmt.Errorf("RTMP 拉流源缺少地址")
		}
		in.Kind = InputRTMP
		in.HasAudio = true
		in.params = []string{"-fflags", "nobuffer"}
		in.path = src.FilePath

	default:
		return nil, fmt.Errorf("不支持的视频源类型: %s", src.Type)
	}
	return in, nil
}

// buildAudioInput 为音频源构建 FFmpeg 输入。
// "usb" 自动探测失败时返回 ErrDeviceUnavailable（Build 会跳过，不再 fallback hw:0，
// 避免无声卡时整个 FFmpeg 进程因打开失败而退出）。
func buildAudioInput(src *model.Source) (*Input, error) {
	in := &Input{Source: src, HasAudio: true}
	switch src.Type {
	case model.SourceAudioDevice:
		dev := src.AudioDevice
		// "usb" 或空值：自动探测 USB 声卡（即插即用，hw 编号会变化）
		if dev == "" || dev == "usb" {
			dev = capture.FindUSBAudioDevice()
			if dev == "" {
				return nil, fmt.Errorf("%w: 未检测到 USB 声卡", ErrDeviceUnavailable)
			}
		}
		if strings.HasPrefix(dev, "pulse") || strings.Contains(dev, "@") {
			in.Kind = InputPulse
			in.params = []string{"-f", "pulse", "-i", dev}
			in.path = ""
			return in, nil
		}
		in.Kind = InputALSA
		sr := src.SampleRate
		if sr <= 0 {
			sr = 48000
		}
		ch := src.Channels
		if ch <= 0 {
			ch = 2
		}
		in.params = []string{"-f", "alsa", "-ar", strconv.Itoa(sr), "-ac", strconv.Itoa(ch)}
		in.path = dev

	case model.SourceMediaFile:
		if src.FilePath == "" {
			return nil, fmt.Errorf("媒体文件源缺少文件路径")
		}
		in.Kind = InputFile
		in.HasVideo = true
		in.path = src.FilePath

	case model.SourceRTMP:
		if src.FilePath == "" {
			return nil, fmt.Errorf("RTMP 拉流源缺少地址")
		}
		in.Kind = InputRTMP
		in.HasVideo = true
		in.params = []string{"-fflags", "nobuffer"}
		in.path = src.FilePath

	default:
		return nil, fmt.Errorf("不支持的音频源类型: %s", src.Type)
	}
	return in, nil
}

// buildFilter 构建 filter_complex
func (rs *RenderSpec) buildFilter(w, h, fps int, videoItems []model.SceneItem,
	srcs map[string]*model.Source, used map[string]int, audioInputs []*Input) error {

	var chain []string

	// 基底：场景画布纯色（保证始终有输出画面）
	baseIdx := len(rs.Inputs)
	rs.Inputs = append(rs.Inputs, &Input{
		Index: baseIdx, Kind: InputColor, HasVideo: true,
		params: []string{"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=black:s=%dx%d:r=%d", w, h, fps)},
	})
	cur := fmt.Sprintf("[%d:v]", baseIdx)

	// 视频层：逐层 scale + overlay（cur 逐层更新为当前合成结果）
	videoCount := 0
	for _, it := range videoItems {
		src := srcs[it.SourceID]
		idx := used[src.ID]
		inLabel := fmt.Sprintf("[%d:v]", idx)

		// 每源滤镜链（crop/scale/透明度/源级滤镜）
		filters := buildSourceFilters(src, &it)
		if len(filters) > 0 {
			label := fmt.Sprintf("[v%d]", videoCount)
			chain = append(chain, inLabel+strings.Join(filters, ",")+label)
			inLabel = label
		}

		// overlay 到当前合成基底，输出作为新的基底
		out := fmt.Sprintf("[vo%d]", videoCount)
		chain = append(chain, fmt.Sprintf("%s%s overlay=%d:%d%s",
			cur, inLabel, it.X, it.Y, out))
		cur = out
		videoCount++
	}

	// 音频：volume → amix
	audioLabels := make([]string, 0, len(audioInputs))
	audioIdx := 0
	for _, in := range audioInputs {
		label := fmt.Sprintf("[a%d]", audioIdx)
		vol := 1.0
		if in.Source != nil && in.Source.Volume > 0 {
			vol = in.Source.Volume
		}
		if vol != 1.0 {
			chain = append(chain, fmt.Sprintf("%s volume=%.2f%s",
				in.Label("a"), vol, label))
		} else {
			// 直接透传
			label = in.Label("a")
		}
		audioLabels = append(audioLabels, label)
		audioIdx++
	}

	// 同时收集视频输入里的音频流（media_file / rtmp 拉流）
	for _, in := range rs.Inputs {
		if in.HasAudio && !in.HasVideo {
			// 纯音频输入已处理
			continue
		}
		if in.HasAudio && in.HasVideo {
			label := fmt.Sprintf("[a%d]", audioIdx)
			vol := 1.0
			if in.Source != nil && in.Source.Volume > 0 {
				vol = in.Source.Volume
			}
			if vol != 1.0 {
				chain = append(chain, fmt.Sprintf("%s volume=%.2f%s",
					in.Label("a"), vol, label))
				audioLabels = append(audioLabels, label)
			} else {
				audioLabels = append(audioLabels, in.Label("a"))
			}
			audioIdx++
		}
	}

	if len(audioLabels) > 0 {
		if len(audioLabels) == 1 {
			rs.AudioOut = audioLabels[0]
		} else {
			rs.AudioOut = "[aout]"
			chain = append(chain, fmt.Sprintf("%samix=inputs=%d:normalize=0[aout]",
				strings.Join(audioLabels, ""), len(audioLabels)))
		}
	}

	if len(chain) == 0 {
		// 无任何滤镜（如所有源被跳过或场景无源）：直接输出基底纯色
		rs.Filter = ""
		rs.VideoOut = cur
		return nil
	}

	rs.Filter = strings.Join(chain, ";")
	rs.VideoOut = cur
	return nil
}

// writeDrawTextFile 将文字内容写入临时文件（drawtext textfile 用，避免转义）。
// drawtext 会把 %{...} 当展开序列，字面 % 需写成 %%
func writeDrawTextFile(content string) (string, error) {
	content = strings.ReplaceAll(content, "%", "%%")
	f, err := os.CreateTemp("", "headlesslive-text-*.txt")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// findCJKFont 查找中文字体文件（drawtext fontfile 用）。
// font=Sans 会被 fontconfig 解析为英文 DejaVu，中文渲染成方块；
// 必须显式指定 CJK 字体文件路径。
func findCJKFont() (string, error) {
	candidates := []string{
		// Linux：Noto CJK（sudo apt install fonts-noto-cjk）
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
		"/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc",
		// Debian 系路径
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		// 文泉驿 / Droid fallback
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		// macOS
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		// Windows
		`C:/Windows/Fonts/msyh.ttc`,   // 微软雅黑
		`C:/Windows/Fonts/simhei.ttf`,  // 黑体
		`C:/Windows/Fonts/simsun.ttc`,  // 宋体
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	// 兜底：用 fontconfig 查 CJK 族名对应的实际文件
	if out, err := exec.Command("fc-match", "-f", "%{file}", "Noto Sans CJK SC").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" && p != "nil" {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("未找到中文字体文件，请安装 fonts-noto-cjk（sudo apt install fonts-noto-cjk）")
}

// buildSourceFilters 构建单个源 + 场景项的滤镜链（crop → scale → 透明度）
// 返回滤镜片段（不含标签），如 ["crop=100:100:0:0","scale=640:360","format=rgba,colorchannelmixer=aa=0.5"]
func buildSourceFilters(src *model.Source, it *model.SceneItem) []string {
	var parts []string

	// 1. 裁剪（先裁原图）
	if it.CropW > 0 && it.CropH > 0 {
		cw, ch, cx, cy := it.CropW, it.CropH, it.CropX, it.CropY
		if cw <= 0 {
			cw = it.Width
		}
		if ch <= 0 {
			ch = it.Height
		}
		parts = append(parts, fmt.Sprintf("crop=%d:%d:%d:%d", cw, ch, cx, cy))
	}

	// 2. 缩放（文字源：等比放大铺满并居中裁剪，避免文字被拉伸变形）
	if it.Width > 0 && it.Height > 0 {
		if src.Type == model.SourceText {
			parts = append(parts, fmt.Sprintf(
				"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
				it.Width, it.Height, it.Width, it.Height))
		} else {
			parts = append(parts, fmt.Sprintf("scale=%d:%d", it.Width, it.Height))
		}
	}

	// 3. 源级滤镜（按顺序）
	parts = append(parts, buildGenericFilters(src.Filters)...)

	// 4. 透明度（需要 rgba 通道）
	if it.Opacity > 0 && it.Opacity < 1.0 {
		parts = append(parts, "format=rgba",
			fmt.Sprintf("colorchannelmixer=aa=%.2f", it.Opacity))
	}

	return parts
}

// buildGenericFilters 将通用 Filter 定义映射为 FFmpeg 滤镜
func buildGenericFilters(filters []model.Filter) []string {
	var out []string
	for _, f := range filters {
		switch f.Type {
		case "crop":
			out = append(out, fmt.Sprintf("crop=%v:%v:%v:%v",
				filterParam(f, "w", "0"), filterParam(f, "h", "0"),
				filterParam(f, "x", "0"), filterParam(f, "y", "0")))
		case "scale":
			out = append(out, fmt.Sprintf("scale=%v:%v",
				filterParam(f, "w", "-1"), filterParam(f, "h", "-1")))
		case "rotate":
			switch strings.ToLower(fmt.Sprint(filterParam(f, "angle", "90"))) {
			case "90", "clockwise":
				out = append(out, "transpose=1")
			case "180":
				out = append(out, "transpose=1,transpose=1")
			case "270", "counterclockwise":
				out = append(out, "transpose=2")
			default:
				out = append(out, fmt.Sprintf("rotate=%v", filterParam(f, "angle", "0")))
			}
		case "hflip":
			out = append(out, "hflip")
		case "vflip":
			out = append(out, "vflip")
		case "color_correct":
			out = append(out, fmt.Sprintf("eq=brightness=%v:contrast=%v:saturation=%v",
				filterParam(f, "brightness", "0"), filterParam(f, "contrast", "1"),
				filterParam(f, "saturation", "1")))
		case "noise_reduction":
			out = append(out, "hqdn3d")
		case "deinterlace":
			out = append(out, "yadif")
		case "chromakey":
			// 抠除背景色（配合浏览器源黑底实现透明融合），输出带 alpha
			out = append(out, fmt.Sprintf("chromakey=color=%s:similarity=%v:blend=%v",
				filterParam(f, "color", "black"),
				filterParam(f, "similarity", "0.1"),
				filterParam(f, "blend", "0.05")))
		}
	}
	return out
}

// filterParam 从 Filter.Params 取参数，支持默认值
func filterParam(f model.Filter, key string, def interface{}) interface{} {
	if f.Params != nil {
		if v, ok := f.Params[key]; ok {
			return v
		}
	}
	return def
}
