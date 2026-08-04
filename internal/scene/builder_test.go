package scene

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/T-Ming-L/HeadlessLive/internal/model"
)

func TestVideoOnlyFilter(t *testing.T) {
	rs := &RenderSpec{
		Filter:   "[0:v]scale=1920:1080[v0];[2:v][v0] overlay=0:0[vo0];[1:a] volume=3.00[a0];[a0][a1]amix=inputs=2:normalize=0[aout]",
		VideoOut: "[vo0]",
	}
	v := rs.VideoOnlyFilter()
	// 音频滤镜必须被剔除
	if strings.Contains(v, "volume") || strings.Contains(v, "amix") {
		t.Errorf("视频滤镜不应含音频: %s", v)
	}
	// 视频链保留
	if !strings.Contains(v, "overlay=0:0") {
		t.Errorf("应保留视频 overlay: %s", v)
	}
	if !strings.Contains(v, "scale=1920:1080") {
		t.Errorf("应保留视频 scale: %s", v)
	}
}

func TestVideoOnlyFilterEmpty(t *testing.T) {
	rs := &RenderSpec{Filter: "", VideoOut: "[0:v]"}
	if v := rs.VideoOnlyFilter(); v != "" {
		t.Errorf("空滤镜应返回空，实际 %s", v)
	}
}

func TestBuildColorSpace(t *testing.T) {
	// v4l2 输入应携带色彩空间参数（用临时文件绕过设备存在性检查）
	tmpDev := filepath.Join(t.TempDir(), "video0")
	if err := os.WriteFile(tmpDev, []byte{}, 0o644); err != nil {
		t.Fatalf("创建临时设备: %v", err)
	}
	src := &model.Source{
		ID: "cam", Name: "采集卡", Type: model.SourceVideoDevice,
		Enabled: true, DevicePath: tmpDev,
		PixelFormat: "yuyv422", Resolution: "1920x1080", FPS: 30,
		ColorSpace: "bt709",
	}
	in, err := buildVideoInput(src)
	if err != nil {
		t.Fatalf("buildVideoInput: %v", err)
	}
	joined := strings.Join(in.FFmpegArgs(), " ")
	for _, want := range []string{"-colorspace bt709", "-color_primaries bt709", "-color_trc bt709"} {
		if !strings.Contains(joined, want) {
			t.Errorf("缺少 %q: %s", want, joined)
		}
	}
}

func TestBuildImage(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatalf("创建临时图片: %v", err)
	}
	src := &model.Source{ID: "img", Name: "Logo", Type: model.SourceImage, Enabled: true, FilePath: tmp}
	in, err := buildVideoInput(src)
	if err != nil {
		t.Fatalf("buildVideoInput: %v", err)
	}
	args := in.FFmpegArgs()
	if strings.Join(args, " ") != "-loop 1 -i "+tmp {
		t.Errorf("图片参数错误: %v", args)
	}
}

func TestBuildText(t *testing.T) {
	src := &model.Source{ID: "txt", Name: "标题", Type: model.SourceText, Enabled: true,
		Text: "直播标题", FontSize: 64, FontColor: "#ffffff"}
	in, err := buildVideoInput(src)
	if err != nil {
		t.Fatalf("buildVideoInput: %v", err)
	}
	joined := strings.Join(in.FFmpegArgs(), " ")
	if !strings.Contains(joined, "color=c=black@0.0") {
		t.Errorf("文字输入应含透明画布: %s", joined)
	}
	if !strings.Contains(joined, "drawtext=fontfile=") {
		t.Errorf("文字输入应含 drawtext fontfile: %s", joined)
	}
	if !strings.Contains(joined, "fontsize=64") || !strings.Contains(joined, "fontcolor=#ffffff") {
		t.Errorf("缺少字号/颜色: %s", joined)
	}
}

func TestBuildTextEmptySkip(t *testing.T) {
	src := &model.Source{ID: "txt", Name: "标题", Type: model.SourceText, Enabled: true}
	_, err := buildVideoInput(src)
	if !errors.Is(err, ErrDeviceUnavailable) {
		t.Errorf("空文字应返回 ErrDeviceUnavailable，实际 %v", err)
	}
}

func TestBuildMediaLoop(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(tmp, []byte("x"), 0o644); err != nil {
		t.Fatalf("创建临时媒体: %v", err)
	}
	src := &model.Source{ID: "mv", Name: "片头", Type: model.SourceMediaFile, Enabled: true,
		FilePath: tmp, Loop: true}
	in, err := buildVideoInput(src)
	if err != nil {
		t.Fatalf("buildVideoInput: %v", err)
	}
	args := in.FFmpegArgs()
	if strings.Join(args, " ") != "-stream_loop -1 -i "+tmp {
		t.Errorf("媒体循环参数错误: %v", args)
	}
}

// 构造一个含多源的测试场景。
// 注意：视频层使用 color 源（lavfi 生成，不依赖真实设备/文件），
// 音频使用显式 hw:0（不触发 USB 探测），保证测试在任意平台可运行。
func testSources() map[string]*model.Source {
	return map[string]*model.Source{
		"cam": {
			ID: "cam", Name: "背景", Type: model.SourceColor,
			Enabled: true, Color: "black",
		},
		"logo": {
			ID: "logo", Name: "Logo", Type: model.SourceColor,
			Enabled: true, Color: "red",
		},
		"mic": {
			ID: "mic", Name: "麦克风", Type: model.SourceAudioDevice,
			Enabled: true, AudioDevice: "hw:0", SampleRate: 48000, Channels: 2, Volume: 0.5,
		},
	}
}

func TestBuildBasic(t *testing.T) {
	sc := &model.Scene{
		ID: "s1", Name: "主场景", CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1.0, ZIndex: 0, Visible: true},
			{SourceID: "logo", X: 20, Y: 20, Width: 240, Height: 135, Opacity: 1.0, ZIndex: 1, Visible: true},
		},
	}

	rs, err := Build(sc, testSources())
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 输入：背景(color) + Logo(color) + 麦克风 + 基底 color
	if len(rs.Inputs) != 4 {
		t.Fatalf("期望 4 个输入，实际 %d", len(rs.Inputs))
	}
	if rs.Inputs[0].Kind != InputColor {
		t.Errorf("输入0 应为 color，实际 %s", rs.Inputs[0].Kind)
	}
	if rs.Inputs[1].Kind != InputColor {
		t.Errorf("输入1 应为 color，实际 %s", rs.Inputs[1].Kind)
	}
	// 基底 color 必须位于最后一个输入
	last := rs.Inputs[len(rs.Inputs)-1]
	if last.Kind != InputColor {
		t.Errorf("最后一个输入应为 color 基底，实际 %s", last.Kind)
	}

	// 滤镜链包含 scale 和 overlay
	if !strings.Contains(rs.Filter, "scale=240:135") {
		t.Errorf("滤镜链缺少 logo scale: %s", rs.Filter)
	}
	if !strings.Contains(rs.Filter, "overlay=20:20") {
		t.Errorf("滤镜链缺少 overlay: %s", rs.Filter)
	}
	if rs.VideoOut == "" {
		t.Error("缺少视频输出标签")
	}
}

func TestBuildWithAudioMix(t *testing.T) {
	sc := &model.Scene{
		ID: "s2", Name: "含音频", CanvasW: 1280, CanvasH: 720, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1280, Height: 720, Opacity: 1.0, ZIndex: 0, Visible: true},
		},
	}

	rs, err := Build(sc, testSources())
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 麦克风（音频）输入应存在
	foundMic := false
	for _, in := range rs.Inputs {
		if in.Kind == InputALSA {
			foundMic = true
			// 音量 0.5 应有 volume 滤镜
		}
	}
	if !foundMic {
		t.Error("缺少麦克风音频输入")
	}

	// 音量 0.5 → volume=0.50
	if !strings.Contains(rs.Filter, "volume=0.50") {
		t.Errorf("滤镜链缺少 volume 滤镜: %s", rs.Filter)
	}
	if rs.AudioOut == "" {
		t.Error("缺少音频输出标签")
	}
}

func TestBuildHiddenAndDisabled(t *testing.T) {
	sc := &model.Scene{
		ID: "s3", Name: "隐藏项", CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1.0, ZIndex: 0, Visible: true},
			{SourceID: "logo", X: 10, Y: 10, Width: 240, Height: 135, Opacity: 1.0, ZIndex: 1, Visible: false},
		},
	}

	rs, err := Build(sc, testSources())
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	// 隐藏的 logo 不应出现
	if strings.Contains(rs.Filter, "logo") || strings.Contains(rs.Filter, "scale=240:135") {
		t.Errorf("隐藏项不应出现在滤镜链: %s", rs.Filter)
	}
	// 采集卡 + 麦克风（全局混音）+ 基底
	if len(rs.Inputs) != 3 {
		t.Errorf("期望 3 个输入，实际 %d", len(rs.Inputs))
	}
}

func TestBuildDenoise(t *testing.T) {
	sc := &model.Scene{
		ID: "s-dn", Name: "降噪", CanvasW: 1280, CanvasH: 720, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1280, Height: 720, Opacity: 1.0, ZIndex: 0, Visible: true},
		},
	}
	srcs := testSources()
	mic := srcs["mic"]
	mic.Denoise = true
	mic.Highpass = 100
	mic.NoiseLevel = -25
	rs, err := Build(sc, srcs)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	if !strings.Contains(rs.Filter, "highpass=f=100") {
		t.Errorf("滤镜链缺少 highpass: %s", rs.Filter)
	}
	for _, f := range []string{"bandreject=f=50", "bandreject=f=100", "bandreject=f=150",
		"bandreject=f=200", "bandreject=f=250", "bandreject=f=300"} {
		if !strings.Contains(rs.Filter, f) {
			t.Errorf("滤镜链缺少电源谐波陷波 %s: %s", f, rs.Filter)
		}
	}
	if !strings.Contains(rs.Filter, "afftdn=nf=-25") || !strings.Contains(rs.Filter, "afftdn=nf=-15") {
		t.Errorf("滤镜链缺少双段 afftdn: %s", rs.Filter)
	}
}

func TestBuildOpacity(t *testing.T) {
	sc := &model.Scene{
		ID: "s4", Name: "透明度", CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1.0, ZIndex: 0, Visible: true},
			{SourceID: "logo", X: 10, Y: 10, Width: 240, Height: 135, Opacity: 0.5, ZIndex: 1, Visible: true},
		},
	}

	rs, err := Build(sc, testSources())
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}

	if !strings.Contains(rs.Filter, "colorchannelmixer=aa=0.50") {
		t.Errorf("透明度滤镜缺失: %s", rs.Filter)
	}
}

func TestEmptyScene(t *testing.T) {
	sc := &model.Scene{
		ID: "s5", Name: "空场景", CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Items: []model.SceneItem{},
	}

	// 空场景也应生成纯色基底输出（不报错）
	rs, err := Build(sc, testSources())
	if err != nil {
		t.Fatalf("空场景 Build 失败: %v", err)
	}
	if rs.VideoOut == "" {
		t.Error("空场景也应有视频输出")
	}
}

func TestBuildSkipMissingDevice(t *testing.T) {
	// 采集卡设备不存在 → 跳过该源，不拖垮整个构建
	srcs := map[string]*model.Source{
		"cam": {
			ID: "cam", Name: "采集卡", Type: model.SourceVideoDevice,
			Enabled: true, DevicePath: "/dev/video999-not-exist",
		},
		"color": {
			ID: "color", Name: "纯色", Type: model.SourceColor,
			Enabled: true, Color: "black",
		},
	}
	sc := &model.Scene{
		ID: "s6", Name: "跳过测试", CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "cam", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1, ZIndex: 0, Visible: true},
			{SourceID: "color", X: 0, Y: 0, Width: 100, Height: 100, Opacity: 1, ZIndex: 1, Visible: true},
		},
	}

	rs, err := Build(sc, srcs)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	// 采集卡应被记录为跳过
	if len(rs.Skipped) != 1 {
		t.Errorf("期望 1 个跳过源，实际 %d: %v", len(rs.Skipped), rs.Skipped)
	}
	// 输入：仅 color + 基底
	if len(rs.Inputs) != 2 {
		t.Errorf("期望 2 个输入（color+基底），实际 %d", len(rs.Inputs))
	}
	// 滤镜链正常（color 层 + overlay）
	if !strings.Contains(rs.Filter, "overlay=0:0") {
		t.Errorf("滤镜链异常: %s", rs.Filter)
	}
}

func TestBuildSkipMissingUSB(t *testing.T) {
	// 无 USB 声卡时（FindUSBAudioDevice 返回空），音频源应跳过而非 fallback hw:0
	srcs := map[string]*model.Source{
		"color": {
			ID: "color", Name: "纯色", Type: model.SourceColor,
			Enabled: true, Color: "blue",
		},
		"mic": {
			ID: "mic", Name: "USB 声卡", Type: model.SourceAudioDevice,
			Enabled: true, AudioDevice: "usb",
		},
	}
	sc := &model.Scene{
		ID: "s7", Name: "跳过音频", CanvasW: 1280, CanvasH: 720, FPS: 30,
		Items: []model.SceneItem{
			{SourceID: "color", X: 0, Y: 0, Width: 1280, Height: 720, Opacity: 1, ZIndex: 0, Visible: true},
		},
	}

	rs, err := Build(sc, srcs)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	// 无 ALSA 输入（usb 探测失败被跳过）；在 Windows 上 runArecordL 返回空 → FindUSBAudioDevice 空 → 跳过
	foundALSA := false
	for _, in := range rs.Inputs {
		if in.Kind == InputALSA {
			foundALSA = true
		}
	}
	if foundALSA {
		t.Error("无 USB 声卡时不应包含 ALSA 输入")
	}
	if rs.AudioOut != "" {
		t.Errorf("无音频时 AudioOut 应为空，实际 %s", rs.AudioOut)
	}
	if len(rs.Skipped) == 0 {
		t.Error("应记录跳过的音频源")
	}
}
