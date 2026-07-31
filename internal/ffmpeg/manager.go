// Package ffmpeg 管理 FFmpeg 子进程：多输出（RTMP/录制）+ 预览流。
// 每个输出独立启动一个 FFmpeg 进程，共用场景渲染规格（RenderSpec）的输入与滤镜链。
package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/model"
	"github.com/T-Ming-L/HeadlessLive/internal/scene"
)

// State 输出状态
type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StateError   State = "error"
)

// OutputStatus 输出运行状态
type OutputStatus struct {
	OutputID  string     `json:"output_id"`
	State     State      `json:"state"`
	FPS       float64    `json:"fps"`
	Bitrate   string     `json:"bitrate"`
	Uptime    string     `json:"uptime"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	ErrorMsg  string     `json:"error_msg,omitempty"`
}

// LogCallback 日志回调（outputID 为空表示系统级日志）
type LogCallback func(outputID, line string)

// StatusCallback 状态回调
type StatusCallback func(status *OutputStatus)

// outputProc 单个输出进程
type outputProc struct {
	outputID  string
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	status    *OutputStatus
	startTime time.Time
}

// Manager FFmpeg 多输出管理器
type Manager struct {
	mu             sync.Mutex
	procs          map[string]*outputProc
	logCallback    LogCallback
	statusCallback StatusCallback
	Debug          bool // 调试模式：true 输出详细 FFmpeg 日志
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		procs: make(map[string]*outputProc),
	}
}

// SetDebug 设置调试模式
func (m *Manager) SetDebug(debug bool) {
	m.Debug = debug
}

// SetCallbacks 设置日志与状态回调
func (m *Manager) SetCallbacks(logCb LogCallback, statusCb StatusCallback) {
	m.logCallback = logCb
	m.statusCallback = statusCb
}

// logf 记录日志（内部调用）
func (m *Manager) logf(outputID, format string, args ...interface{}) {
	if m.logCallback != nil {
		m.logCallback(outputID, fmt.Sprintf(format, args...))
	}
}

func (m *Manager) notifyStatus(p *outputProc) {
	if m.statusCallback != nil {
		m.statusCallback(p.status)
	}
}

// GetStatus 获取指定输出状态
func (m *Manager) GetStatus(outputID string) *OutputStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[outputID]; ok {
		return p.status
	}
	return &OutputStatus{OutputID: outputID, State: StateIdle}
}

// GetAllStatus 获取所有输出状态
func (m *Manager) GetAllStatus() []*OutputStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*OutputStatus, 0, len(m.procs))
	for _, p := range m.procs {
		s := *p.status
		if p.status.State == StateRunning && !p.startTime.IsZero() {
			s.Uptime = time.Since(p.startTime).Round(time.Second).String()
		}
		out = append(out, &s)
	}
	return out
}

// StartOutput 启动一个输出（推流/录制），基于场景渲染规格
func (m *Manager) StartOutput(rs *scene.RenderSpec, o *model.Output) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.procs[o.ID]; ok {
		return fmt.Errorf("输出 %s 已在运行", o.Name)
	}

	args, err := BuildOutputArgs(rs, o, m.Debug)
	if err != nil {
		return err
	}

	m.logf(o.ID, "[ffmpeg] 启动输出 %s: %v", o.Name, args)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("启动 FFmpeg 失败: %w", err)
	}

	now := time.Now()
	p := &outputProc{
		outputID:  o.ID,
		cmd:       cmd,
		cancel:    cancel,
		startTime: now,
		status: &OutputStatus{
			OutputID:  o.ID,
			State:     StateRunning,
			StartedAt: &now,
		},
	}
	m.procs[o.ID] = p

	// 异步读取 stderr
	go m.readStderr(p, stderr)

	// 异步等待退出
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		defer m.mu.Unlock()

		if p2, ok := m.procs[o.ID]; !ok || p2 != p {
			return // 已被替换或停止
		}

		if err != nil {
			if ctx.Err() != nil {
				m.logf(o.ID, "[ffmpeg] 输出 %s 被主动停止", o.Name)
			} else {
				m.logf(o.ID, "[ffmpeg] 输出 %s 异常退出: %v", o.Name, err)
				p.status.State = StateError
				p.status.ErrorMsg = err.Error()
				m.notifyStatus(p)
			}
		} else {
			m.logf(o.ID, "[ffmpeg] 输出 %s 正常结束", o.Name)
		}

		delete(m.procs, o.ID)
		p.status.State = StateIdle
		m.notifyStatus(p)
	}()

	m.notifyStatus(p)
	return nil
}

// StopOutput 停止指定输出
func (m *Manager) StopOutput(outputID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[outputID]; ok {
		m.stopProc(p)
	}
}

// StopAll 停止全部输出
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.procs {
		m.stopProc(p)
	}
}

// stopProc 停止单个进程（需持有锁）
func (m *Manager) stopProc(p *outputProc) {
	if p.cancel != nil {
		p.cancel()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		m.logf(p.outputID, "[ffmpeg] 终止进程...")
		_ = p.cmd.Process.Kill()
		// 进程卡死（如 v4l2 读取阻塞进入 D 状态）时 SIGKILL 无效、Wait 永久阻塞，
		// 必须超时返回，否则 ctrl+C 清理会卡死
		waitWithTimeout(p.cmd, 3*time.Second)
	}
	delete(m.procs, p.outputID)
	p.status.State = StateIdle
	m.notifyStatus(p)
}

// waitWithTimeout 等待进程结束，超时后放弃。
// 防止 ffmpeg 卡在不可中断状态（D 状态，如 v4l2 ioctl 阻塞）时 Kill 无效、Wait 永久阻塞。
func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// readStderr 读取 FFmpeg stderr，推送日志并解析 fps/bitrate
func (m *Manager) readStderr(p *outputProc, pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	fpsRe := regexp.MustCompile(`fps=\s*([\d.]+)`)
	bitrateRe := regexp.MustCompile(`bitrate=\s*([\d.]+\s*\w+/s)`)

	for scanner.Scan() {
		line := scanner.Text()
		m.logf(p.outputID, "%s", line)

		if matches := fpsRe.FindStringSubmatch(line); matches != nil {
			var fps float64
			fmt.Sscanf(matches[1], "%f", &fps)
			m.mu.Lock()
			p.status.FPS = fps
			m.mu.Unlock()
		}
		if matches := bitrateRe.FindStringSubmatch(line); matches != nil {
			m.mu.Lock()
			p.status.Bitrate = matches[1]
			m.mu.Unlock()
		}
	}
}

// BuildOutputArgs 根据渲染规格 + 输出配置构建 FFmpeg 命令行
func BuildOutputArgs(rs *scene.RenderSpec, o *model.Output, debug bool) ([]string, error) {
	loglevel := "warning"
	if debug {
		loglevel = "info"
	}
	args := []string{"-hide_banner", "-loglevel", loglevel, "-y"}

	// 编码参数
	encoder := o.Encoder
	if encoder == "" {
		encoder = PickBestEncoder("")
	}

	// FLV 容器（RTMP 推流 / FLV 录制）不支持 HEVC，自动回退 H.264，
	// 避免 ffmpeg 报 "Video codec hevc not compatible with flv"
	if o.Type == model.OutputRTMP ||
		(o.Type == model.OutputRecord && strings.EqualFold(o.Format, "flv")) {
		if encoder == "hevc_vaapi" {
			encoder = "h264_vaapi"
		}
	}

	// VAAPI 设备全局选项前置（hwupload 需要）
	if encoder == "h264_vaapi" || encoder == "hevc_vaapi" {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	}

	// 输入
	for _, in := range rs.Inputs {
		args = append(args, in.FFmpegArgs()...)
	}

	// 滤镜链：编码器需要的格式转换（nv12/hwupload）合并进 filter_complex，
	// 避免 -vf 与 -filter_complex 冲突（ffmpeg 不允许两者作用于同一流）
	filter := rs.Filter
	videoMap := rs.VideoOut
	switch encoder {
	case "h264_vaapi", "hevc_vaapi":
		filter, videoMap = appendOutputFilter(rs, "format=nv12,hwupload")
	case "h264_qsv":
		filter, videoMap = appendOutputFilter(rs, "format=nv12")
	}
	if filter != "" {
		args = append(args, "-filter_complex", filter)
	}

	// 映射（输入流标签需转成 "N:a" 形式：存在 -filter_complex 时 [1:a] 会被当作 filter 输出标签）
	args = append(args, "-map", mapArg(videoMap))
	if rs.AudioOut != "" {
		args = append(args, "-map", mapArg(rs.AudioOut))
	}

	bitrate := o.Bitrate
	if bitrate <= 0 {
		bitrate = 5000
	}
	keyframe := o.KeyFrame
	if keyframe <= 0 {
		keyframe = rs.FPS * 2
	}

	switch o.Type {
	case model.OutputRTMP:
		url := strings.TrimRight(o.URL, "/")
		if url == "" {
			return nil, fmt.Errorf("输出 %s 缺少 RTMP 地址", o.Name)
		}
		if o.StreamKey != "" {
			url += "/" + o.StreamKey
		}
		args = append(args, encodeVideoArgs(encoder, bitrate, keyframe, rs.FPS)...)
		args = append(args, "-c:a", "aac", "-b:a", "128k", "-ar", "44100")
		args = append(args, "-f", "flv", url)

	case model.OutputRecord:
		path := o.FilePath
		if path == "" {
			path = fmt.Sprintf("recordings/record-%s.mp4", time.Now().Format("20060102-150405"))
		}
		if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
			return nil, fmt.Errorf("创建录制目录失败: %w", err)
		}
		format := o.Format
		if format == "" {
			format = "mp4"
		}
		args = append(args, encodeVideoArgs(encoder, bitrate, keyframe, rs.FPS)...)
		args = append(args, "-c:a", "aac", "-b:a", "128k")
		args = append(args, "-f", containerFor(format), path)

	case model.OutputSRT:
		url := o.URL
		if url == "" {
			return nil, fmt.Errorf("输出 %s 缺少 SRT 地址", o.Name)
		}
		args = append(args, encodeVideoArgs(encoder, bitrate, keyframe, rs.FPS)...)
		args = append(args, "-c:a", "aac", "-b:a", "128k")
		args = append(args, "-f", "mpegts", url)

	default:
		return nil, fmt.Errorf("不支持的输出类型: %s", o.Type)
	}

	return args, nil
}

// mapArg 将内部流标签转换为 ffmpeg -map 参数。
// filter 输出标签（如 [aout]）原样保留；输入流标签（如 [1:a]）转为 "1:a"，
// 因为存在 -filter_complex 时，[1:a] 会被当作 filter 图输出标签而报错。
func mapArg(label string) string {
	if len(label) >= 4 && label[0] == '[' {
		if end := strings.Index(label, "]"); end > 0 {
			inner := label[1:end]
			if i := strings.IndexByte(inner, ':'); i > 0 && isDigits(inner[:i]) {
				return inner // 数字:流 → 输入引用
			}
		}
	}
	return label
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// appendOutputFilter 将输出级滤镜（编码前转换）追加到 filter_complex 末尾，
// 返回新的 filter_complex 与输出标签。避免 -vf 与 -filter_complex 冲突。
func appendOutputFilter(rs *scene.RenderSpec, chain string) (filter, outLabel string) {
	return appendOutputFilterBase(rs.Filter, rs.VideoOut, chain)
}

// appendOutputFilterBase 基于自定义基础滤镜链追加输出级滤镜
func appendOutputFilterBase(base, videoOut, chain string) (filter, outLabel string) {
	outLabel = "[vout]"
	if base == "" {
		filter = videoOut + chain + outLabel
	} else {
		filter = base + ";" + videoOut + chain + outLabel
	}
	return filter, outLabel
}

// encodeVideoArgs 视频编码参数（格式转换已在 filter_complex 中处理）
func encodeVideoArgs(encoder string, bitrate, keyframe, fps int) []string {
	var args []string
	switch encoder {
	case "h264_vaapi", "hevc_vaapi":
		codec := "h264_vaapi"
		if encoder == "hevc_vaapi" {
			codec = "hevc_vaapi"
		}
		args = append(args, "-c:v", codec,
			"-b:v", fmt.Sprintf("%dk", bitrate),
			"-maxrate", fmt.Sprintf("%dk", bitrate),
			"-bufsize", fmt.Sprintf("%dk", bitrate*2),
			"-g", fmt.Sprintf("%d", keyframe))
	case "h264_qsv":
		args = append(args, "-c:v", "h264_qsv",
			"-b:v", fmt.Sprintf("%dk", bitrate),
			"-maxrate", fmt.Sprintf("%dk", bitrate),
			"-bufsize", fmt.Sprintf("%dk", bitrate*2),
			"-g", fmt.Sprintf("%d", keyframe))
	default: // libx264 等软件编码
		args = append(args, "-c:v", encoder,
			"-preset", "ultrafast",
			"-b:v", fmt.Sprintf("%dk", bitrate),
			"-maxrate", fmt.Sprintf("%dk", bitrate),
			"-bufsize", fmt.Sprintf("%dk", bitrate*2),
			"-pix_fmt", "yuv420p",
			"-g", fmt.Sprintf("%d", keyframe))
	}
	return args
}

// PickBestEncoder 自动选择最佳编码器
func PickBestEncoder(preferred string) string {
	if preferred != "" && preferred != "libx264" && preferred != "h264_qsv" {
		return preferred
	}
	if isVAAPIAvailable() {
		return "h264_vaapi"
	}
	if isQSVAvailable() {
		return "h264_qsv"
	}
	return "libx264"
}

func isVAAPIAvailable() bool {
	if _, err := os.Stat("/dev/dri/renderD128"); os.IsNotExist(err) {
		return false
	}
	cmd := exec.Command("vainfo")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, "H.264") || strings.Contains(s, "H264") ||
		strings.Contains(s, "VAProfileH264")
}

func isQSVAvailable() bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "h264_qsv")
}

func containerFor(format string) string {
	switch strings.ToLower(format) {
	case "mkv":
		return "matroska"
	case "flv":
		return "flv"
	case "ts", "mpegts":
		return "mpegts"
	default:
		return "mp4"
	}
}

func dirOf(path string) string {
	idx := strings.LastIndexAny(path, "/\\")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}
