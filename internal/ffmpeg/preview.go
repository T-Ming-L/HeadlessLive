package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/scene"
)

// Preview MJPEG 预览流。
// 独立 FFmpeg 进程，输出 mpjpeg（multipart/x-mixed-replace）到 stdout，
// 再广播给所有 HTTP 客户端。stderr 会通过日志回调输出，方便排查失败原因。
//
// 注意：预览与推流/录制各启动一个 FFmpeg 进程，同时打开同一采集设备
// 可能受驱动限制（UVC 一般允许多 open），若冲突需后续改为单一采集进程 + 内存分发。
type Preview struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	clients     map[chan []byte]bool
	latestFrame []byte // 最新一帧 JPEG（单帧轮询用，兼容所有浏览器）
	logf        func(format string, args ...interface{})
	onExit      func() // 进程退出回调（用于通知前端预览已停止）
	Debug       bool   // 调试模式：true 输出详细 FFmpeg 日志
}

// NewPreview 创建预览器
func NewPreview() *Preview {
	return &Preview{
		clients: make(map[chan []byte]bool),
	}
}

// SetLogger 设置日志回调
func (p *Preview) SetLogger(logf func(format string, args ...interface{})) {
	p.logf = logf
}

// SetDebug 设置调试模式
func (p *Preview) SetDebug(debug bool) {
	p.Debug = debug
}

// SetOnExit 设置进程退出回调
func (p *Preview) SetOnExit(fn func()) {
	p.onExit = fn
}

func (p *Preview) logf2(format string, args ...interface{}) {
	if p.logf != nil {
		p.logf(format, args...)
	}
}

// IsRunning 预览是否在运行
func (p *Preview) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}

// Start 启动预览进程（指定场景渲染规格 + 预览最大宽度）
func (p *Preview) Start(rs *scene.RenderSpec, maxW int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return fmt.Errorf("预览已在运行")
	}

	loglevel := "warning"
	if p.Debug {
		loglevel = "info"
	}
	args := []string{"-hide_banner", "-loglevel", loglevel}
	for _, in := range rs.Inputs {
		args = append(args, in.FFmpegArgs()...)
	}
	// 预览只输出视频：必须剔除音频滤镜（否则未连接的 volume/amix 输出导致 ffmpeg 报错退出）
	vFilter := rs.VideoOnlyFilter()
	filter := vFilter
	videoMap := rs.VideoOut
	if maxW > 0 {
		filter, videoMap = appendOutputFilterBase(vFilter, rs.VideoOut, fmt.Sprintf("scale=%d:-2", maxW))
	}
	if filter != "" {
		args = append(args, "-filter_complex", filter)
	}
	args = append(args, "-map", mapArg(videoMap))
	args = append(args, "-f", "mpjpeg", "-q:v", "3", "pipe:1")

	p.logf2("[preview] 启动预览: ffmpeg %v", args)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("创建预览 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("创建预览 stderr 管道失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("启动预览 FFmpeg 失败: %w", err)
	}

	p.cmd = cmd
	p.cancel = cancel

	// stderr → 日志（预览失败原因可见）
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			p.logf2("[preview] %s", sc.Text())
		}
	}()

	go p.pump(stdout)
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		var exit func()
		if p.cmd == cmd {
			p.cmd = nil
			p.cancel = nil
			// 通知客户端结束
			for ch := range p.clients {
				close(ch)
			}
			p.clients = make(map[chan []byte]bool)
			exit = p.onExit
		}
		p.mu.Unlock()
		// 锁外调用，避免死锁
		if exit != nil {
			exit()
		}
	}()

	return nil
}

// Stop 停止预览
func (p *Preview) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		p.cancel()
		_ = p.cmd.Process.Kill()
		// 进程卡死时 Wait 会永久阻塞，超时返回避免清理卡死
		waitWithTimeout(p.cmd, 3*time.Second)
		p.cmd = nil
		p.cancel = nil
	}
	for ch := range p.clients {
		close(ch)
	}
	p.clients = make(map[chan []byte]bool)
}

// pump 从 stdout 读取并广播到所有客户端（丢帧策略：客户端消费慢则丢弃）。
// 同时解析出完整 JPEG 帧保存到 latestFrame，供 /preview/frame 单帧轮询使用。
func (p *Preview) pump(stdout io.ReadCloser) {
	defer stdout.Close()
	buf := make([]byte, 64*1024)
	pending := make([]byte, 0, 512*1024) // 待解析缓冲
	inFrame := false
	var frame []byte

	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			data := append([]byte{}, buf[:n]...)
			p.broadcast(data) // multipart 流广播（保留 /preview）

			// 解析 JPEG 帧（按 SOI FFD8 / EOI FFD9 切帧）
			pending = append(pending, data...)
			for {
				if !inFrame {
					idx := findJPEGSOI(pending)
					if idx < 0 {
						// 丢弃帧间垃圾（multipart 头），保留尾部以防 SOI 跨 chunk
						if len(pending) > 1 {
							pending = pending[len(pending)-1:]
						}
						break
					}
					pending = pending[idx:]
					inFrame = true
					frame = frame[:0]
				}
				eoi := findJPEGEOI(pending)
				if eoi < 0 {
					// 帧未完整，保留全部继续等
					frame = append(frame, pending...)
					pending = pending[:0]
					break
				}
				frame = append(frame, pending[:eoi+2]...)
				pending = pending[eoi+2:]
				inFrame = false
				p.mu.Lock()
				p.latestFrame = append([]byte{}, frame...)
				p.mu.Unlock()
				frame = frame[:0]
			}
		}
		if err != nil {
			return
		}
	}
}

// broadcast 向所有客户端广播数据
func (p *Preview) broadcast(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for ch := range p.clients {
		select {
		case ch <- data:
		default: // 客户端消费慢，丢弃
		}
	}
}

// findJPEGSOI 查找 JPEG 起始标记 FF D8
func findJPEGSOI(b []byte) int {
	for i := 0; i < len(b)-1; i++ {
		if b[i] == 0xFF && b[i+1] == 0xD8 {
			return i
		}
	}
	return -1
}

// findJPEGEOI 查找 JPEG 结束标记 FF D9
func findJPEGEOI(b []byte) int {
	for i := 0; i < len(b)-1; i++ {
		if b[i] == 0xFF && b[i+1] == 0xD9 {
			return i
		}
	}
	return -1
}

// FrameHandler 返回最新一帧 JPEG（单帧模式，兼容所有浏览器）
func (p *Preview) FrameHandler(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	running := p.cmd != nil
	f := append([]byte{}, p.latestFrame...)
	p.mu.Unlock()
	if !running || len(f) == 0 {
		http.Error(w, "无预览帧", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(f)
}

// Handler 提供 multipart/x-mixed-replace MJPEG 流
func (p *Preview) Handler(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	if p.cmd == nil {
		p.mu.Unlock()
		http.Error(w, "预览未运行", http.StatusServiceUnavailable)
		return
	}
	ch := make(chan []byte, 32)
	p.clients[ch] = true
	p.mu.Unlock()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=ffmpegboundary")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		p.mu.Lock()
		delete(p.clients, ch)
		p.mu.Unlock()
		http.Error(w, "流式响应不支持", http.StatusInternalServerError)
		return
	}

	defer func() {
		p.mu.Lock()
		delete(p.clients, ch)
		p.mu.Unlock()
	}()

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return // 预览已停止
			}
			if _, err := w.Write(data); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
