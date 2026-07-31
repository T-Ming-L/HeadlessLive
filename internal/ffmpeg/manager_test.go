package ffmpeg

import (
	"strings"
	"testing"

	"github.com/T-Ming-L/HeadlessLive/internal/model"
	"github.com/T-Ming-L/HeadlessLive/internal/scene"
)

// 构造最小渲染规格：基底 color + 一个 color 视频层 + 音频
func testRenderSpec() *scene.RenderSpec {
	rs := &scene.RenderSpec{
		CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Filter: "[0:v]scale=1920:1080[v0];[2:v][v0] overlay=0:0[vo0]",
		VideoOut: "[vo0]",
		AudioOut: "[1:a]",
	}
	// 输入：camera + alsa + base
	rs.Inputs = []*scene.Input{
		{Index: 0, Kind: scene.InputV4L2, HasVideo: true},
		{Index: 1, Kind: scene.InputALSA, HasAudio: true},
		{Index: 2, Kind: scene.InputColor, HasVideo: true},
	}
	return rs
}

func TestBuildOutputArgsVAAPI(t *testing.T) {
	rs := testRenderSpec()
	o := &model.Output{ID: "o1", Name: "t", Type: model.OutputRTMP,
		URL: "rtmp://x/live", StreamKey: "key", Encoder: "h264_vaapi", Bitrate: 5000, KeyFrame: 60}

	args, err := BuildOutputArgs(rs, o, true)
	if err != nil {
		t.Fatalf("BuildOutputArgs: %v", err)
	}
	cmd := strings.Join(args, " ")

	// -vaapi_device 必须前置（在 -filter_complex 之前）
	if i := strings.Index(cmd, "-vaapi_device /dev/dri/renderD128"); i < 0 {
		t.Errorf("缺少 -vaapi_device: %s", cmd)
	}
	// 不得再用 -vf（与 -filter_complex 冲突）
	if strings.Contains(cmd, " -vf ") {
		t.Errorf("不应出现 -vf: %s", cmd)
	}
	// filter_complex 需包含 hwupload 转换
	if !strings.Contains(cmd, "format=nv12,hwupload") {
		t.Errorf("filter_complex 缺少 format=nv12,hwupload: %s", cmd)
	}
	// map 应指向转换后的 [vout]
	if !strings.Contains(cmd, "-map [vout]") {
		t.Errorf("应 map [vout]: %s", cmd)
	}
	// 音频输入流标签应转为数字形式（存在 filter_complex 时 [1:a] 无效）
	if !strings.Contains(cmd, "-map 1:a") {
		t.Errorf("音频应 map 1:a（输入流数字形式）: %s", cmd)
	}
}

func TestBuildOutputArgsSoftware(t *testing.T) {
	rs := testRenderSpec()
	o := &model.Output{ID: "o1", Name: "t", Type: model.OutputRTMP,
		URL: "rtmp://x/live", StreamKey: "key", Encoder: "libx264", Bitrate: 5000, KeyFrame: 60}

	args, err := BuildOutputArgs(rs, o, false)
	if err != nil {
		t.Fatalf("BuildOutputArgs: %v", err)
	}
	cmd := strings.Join(args, " ")

	// 软件编码不应有 -vf / hwupload
	if strings.Contains(cmd, " -vf ") {
		t.Errorf("软件编码不应有 -vf: %s", cmd)
	}
	if strings.Contains(cmd, "hwupload") {
		t.Errorf("软件编码不应有 hwupload: %s", cmd)
	}
	// 保持原 filter_complex 与 VideoOut
	if !strings.Contains(cmd, "[vo0]") {
		t.Errorf("软件编码应 map 原 [vo0]: %s", cmd)
	}
	// 无 -vaapi_device
	if strings.Contains(cmd, "vaapi") {
		t.Errorf("软件编码不应有 vaapi: %s", cmd)
	}
}

func TestAppendOutputFilter(t *testing.T) {
	// Filter 非空
	rs := testRenderSpec()
	f, label := appendOutputFilter(rs, "format=nv12,hwupload")
	if label != "[vout]" {
		t.Errorf("标签错误: %s", label)
	}
	if !strings.HasSuffix(f, "[vo0]format=nv12,hwupload[vout]") {
		t.Errorf("追加错误: %s", f)
	}

	// Filter 为空（直接输出输入流）
	rs2 := &scene.RenderSpec{Filter: "", VideoOut: "[0:v]"}
	f2, _ := appendOutputFilter(rs2, "scale=960:-2")
	if f2 != "[0:v]scale=960:-2[vout]" {
		t.Errorf("空 filter 追加错误: %s", f2)
	}
}

func TestPreviewFilterMerge(t *testing.T) {
	// 模拟预览逻辑：scale 合并进 filter_complex
	rs := testRenderSpec()
	filter, label := appendOutputFilter(rs, "scale=960:-2")
	if !strings.Contains(filter, "[vo0]scale=960:-2[vout]") {
		t.Errorf("预览 scale 未合并: %s", filter)
	}
	if label != "[vout]" {
		t.Errorf("预览标签错误: %s", label)
	}
}

func TestMapArg(t *testing.T) {
	// 输入流标签 → 数字:流
	if got := mapArg("[1:a]"); got != "1:a" {
		t.Errorf("[1:a] 应转为 1:a，实际 %s", got)
	}
	if got := mapArg("[0:v]"); got != "0:v" {
		t.Errorf("[0:v] 应转为 0:v，实际 %s", got)
	}
	// filter 输出标签保留
	if got := mapArg("[aout]"); got != "[aout]" {
		t.Errorf("[aout] 应保留，实际 %s", got)
	}
	if got := mapArg("[vout]"); got != "[vout]" {
		t.Errorf("[vout] 应保留，实际 %s", got)
	}
	if got := mapArg("[vo0]"); got != "[vo0]" {
		t.Errorf("[vo0] 应保留，实际 %s", got)
	}
}

func TestBuildOutputArgsNoFilterAudioMap(t *testing.T) {
	// 无滤镜（Filter 空，VideoOut 为输入标签）时，map 也需转数字形式
	rs := &scene.RenderSpec{CanvasW: 1920, CanvasH: 1080, FPS: 30,
		Filter: "", VideoOut: "[2:v]", AudioOut: "[1:a]"}
	o := &model.Output{ID: "o1", Name: "t", Type: model.OutputRTMP,
		URL: "rtmp://x/live", StreamKey: "key", Encoder: "libx264"}

	args, err := BuildOutputArgs(rs, o, false)
	if err != nil {
		t.Fatalf("BuildOutputArgs: %v", err)
	}
	cmd := strings.Join(args, " ")
	// 无 filter_complex
	if strings.Contains(cmd, "-filter_complex") {
		t.Errorf("Filter 空时不应有 filter_complex: %s", cmd)
	}
	// map 用数字形式
	if !strings.Contains(cmd, "-map 2:v") || !strings.Contains(cmd, "-map 1:a") {
		t.Errorf("map 应为数字形式: %s", cmd)
	}
}

func TestBuildOutputArgsHevcFallbackForFLV(t *testing.T) {
	// RTMP(FLV) 容器不支持 HEVC，hevc_vaapi 应自动回退 h264_vaapi
	rs := testRenderSpec()
	o := &model.Output{ID: "o1", Name: "t", Type: model.OutputRTMP,
		URL: "rtmp://x/live", StreamKey: "key", Encoder: "hevc_vaapi", Bitrate: 5000, KeyFrame: 60}

	args, err := BuildOutputArgs(rs, o, false)
	if err != nil {
		t.Fatalf("BuildOutputArgs: %v", err)
	}
	cmd := strings.Join(args, " ")
	if strings.Contains(cmd, "hevc") {
		t.Errorf("FLV 输出不应使用 HEVC: %s", cmd)
	}
	if !strings.Contains(cmd, "-c:v h264_vaapi") {
		t.Errorf("应回退到 h264_vaapi: %s", cmd)
	}
	// 仍需 -vaapi_device 与 hwupload
	if !strings.Contains(cmd, "-vaapi_device /dev/dri/renderD128") ||
		!strings.Contains(cmd, "format=nv12,hwupload") {
		t.Errorf("VAAPI 回退后参数不完整: %s", cmd)
	}

	// MP4 录制保持 HEVC 可用
	o2 := &model.Output{ID: "o2", Name: "r", Type: model.OutputRecord,
		FilePath: "rec.mp4", Format: "mp4", Encoder: "hevc_vaapi", Bitrate: 5000}
	args2, err := BuildOutputArgs(rs, o2, false)
	if err != nil {
		t.Fatalf("BuildOutputArgs record: %v", err)
	}
	if !strings.Contains(strings.Join(args2, " "), "hevc_vaapi") {
		t.Errorf("MP4 录制应保留 HEVC: %s", strings.Join(args2, " "))
	}
}
