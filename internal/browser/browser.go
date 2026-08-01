// Package browser 自动管理浏览器源所需环境：Xvfb 虚拟显示器 + Chromium/Chrome 进程。
//
// 浏览器源通过 x11grab 抓取虚拟显示器，本包在首次使用浏览器源时自动拉起
// Xvfb 与浏览器（按显示器分别管理），退出时统一清理，无需用户手动启动。
// 注意：snap 版 Chromium 因沙箱限制无法连接 Xvfb，请使用 deb 版 chromium / google-chrome。
package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 浏览器可执行文件候选（按顺序探测）。
// Google Chrome（deb 版）最稳定；snap 版 chromium 实际也能连接 Xvfb 渲染，
// 仅在缺少 Chrome 时作为后备（会提示可能存在的兼容问题）。
var browserCandidates = []string{
	"google-chrome-stable",
	"google-chrome",
	"chromium",
	"chromium-browser",
	"chrome",
}

// 常见安装路径（PATH 异常时的后备探测）
var browserPaths = []string{
	"/usr/bin/google-chrome-stable",
	"/usr/bin/google-chrome",
	"/opt/google/chrome/chrome",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/snap/bin/chromium",
}

// displayProc 一个虚拟显示器上的进程组
// （记录当前打开的 URL/尺寸，变化时自动重启浏览器以支持切换网站）
type displayProc struct {
	xvfb   *exec.Cmd
	chrome *exec.Cmd
	url    string
	w, h   int
}

// Manager 浏览器环境管理器（按 DISPLAY 区分，支持多个虚拟显示器）
type Manager struct {
	mu    sync.Mutex
	procs map[string]*displayProc
	logf  func(string, ...interface{})
}

// NewManager 创建浏览器管理器
func NewManager() *Manager {
	return &Manager{procs: map[string]*displayProc{}, logf: func(string, ...interface{}) {}}
}

// SetLogger 设置日志回调（转发到 WebSocket / 日志文件）
func (m *Manager) SetLogger(f func(format string, args ...interface{})) {
	if f != nil {
		m.logf = f
	}
}

// Ensure 确保指定显示器上的 Xvfb + 浏览器已启动。
// 若浏览器已打开但 URL 或尺寸与目标不一致，会自动重启浏览器（Xvfb 保留）。
func (m *Manager) Ensure(disp, url string, w, h int) error {
	if disp == "" {
		disp = ":99"
	}
	if w <= 0 {
		w = 1280
	}
	if h <= 0 {
		h = 720
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 已在运行：目标一致则复用，URL/尺寸变化则重启浏览器
	if p, ok := m.procs[disp]; ok && p.xvfb != nil && p.chrome != nil {
		if p.url == url && p.w == w && p.h == h {
			return nil
		}
		m.logf("[browser] 目标变化（%s %dx%d），重启浏览器...", url, w, h)
		m.killChromeLocked(disp)
		if err := m.startChromeLocked(disp, url, w, h); err != nil {
			m.stopLocked(disp)
			return err
		}
		return nil
	}

	m.stopLocked(disp)

	m.logf("[browser] 启动虚拟显示器 %s (%dx%d)...", disp, w, h)
	xvfb, err := m.startXvfbLocked(disp, w, h)
	if err != nil {
		m.logf("[browser] ❌ Xvfb 启动失败: %v", err)
		return err
	}
	m.procs[disp] = &displayProc{xvfb: xvfb}

	if err := m.startChromeLocked(disp, url, w, h); err != nil {
		m.stopLocked(disp)
		return err
	}
	return nil
}

// startXvfbLocked 启动 Xvfb 并等待 X socket 就绪。
// 启动失败时捕获 stderr 给出真实原因，并清理残留锁/socket 后重试一次。
func (m *Manager) startXvfbLocked(disp string, w, h int) (*exec.Cmd, error) {
	if _, err := exec.LookPath("Xvfb"); err != nil {
		return nil, fmt.Errorf("未找到 Xvfb，请先安装（sudo apt install xvfb）")
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		var buf syncBuffer
		xvfb := exec.Command("Xvfb", disp, "-screen", "0", fmt.Sprintf("%dx%dx24", w, h),
			"-ac", "+extension", "GLX", "+render", "-noreset")
		xvfb.Stdout = &buf
		xvfb.Stderr = &buf
		if err := xvfb.Start(); err != nil {
			return nil, fmt.Errorf("启动 Xvfb 失败: %w", err)
		}
		died := make(chan struct{})
		go func() {
			_ = xvfb.Wait()
			close(died)
		}()

		// 等待 X socket 就绪（Xvfb 启动后 /tmp/.X11-unix/X<num> 才出现）
		sock := x11Socket(disp)
		deadline := time.Now().Add(5 * time.Second)
		ready := false
		for time.Now().Before(deadline) {
			if _, err := os.Stat(sock); err == nil {
				ready = true
				break
			}
			select {
			case <-died:
				goto failed // Xvfb 提前退出
			default:
			}
			time.Sleep(200 * time.Millisecond)
		}
		if ready {
			return xvfb, nil
		}

	failed:
		lastErr = fmt.Errorf("Xvfb 未就绪（第 %d 次）: %s", attempt+1, strings.TrimSpace(buf.String()))
		// 清理残留锁 / socket（上次异常退出会导致 Xvfb 拒绝启动）
		cleanupDisplayFiles(disp)
		time.Sleep(300 * time.Millisecond)
	}
	return nil, lastErr
}

// cleanupDisplayFiles 删除显示器的残留锁文件与 socket（异常退出后 Xvfb 无法再启动同一编号）
func cleanupDisplayFiles(disp string) {
	num := strings.TrimPrefix(disp, ":")
	_ = os.Remove(filepath.Join(os.TempDir(), ".X"+num+"-lock"))
	_ = os.Remove(x11Socket(disp))
}

// syncBuffer 线程安全的 stderr 缓冲（子进程写、主线程读）
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// startChromeLocked 探测浏览器并启动（--kiosk 全屏无边框，只显示网页内容）
func (m *Manager) startChromeLocked(disp, url string, w, h int) error {
	// 探测浏览器可执行文件（Google Chrome 优先；snap 版可用但提示兼容问题）
	bin := ""
	for _, c := range browserCandidates {
		if p, err := exec.LookPath(c); err == nil {
			bin = p
			break
		}
	}
	// PATH 里没有时，直接探测常见安装路径（兼容 PATH 异常 / 服务环境）
	if bin == "" {
		for _, p := range browserPaths {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				bin = p
				break
			}
		}
	}
	if bin == "" {
		return fmt.Errorf("未找到 Chrome/Chromium，请安装（推荐 Google Chrome deb 版）后重试")
	}
	if strings.HasPrefix(bin, "/snap/") {
		m.logf("[browser] 使用 snap 版 %s（可运行，若异常建议安装 Google Chrome deb 版）", bin)
	}

	m.logf("[browser] 打开 %s (%dx%d) -> %s", bin, w, h, url)
	chrome := exec.Command(bin,
		"--no-sandbox", "--disable-gpu", "--hide-scrollbars",
		"--kiosk", // 全屏无边框，x11grab 只捕获网页内容
		"--window-position=0,0",
		"--window-size="+fmt.Sprintf("%dx%d", w, h),
		"--autoplay-policy=no-user-gesture-required",
		"--no-first-run", "--no-default-browser-check",
		"--disable-session-crashed-bubble", "--disable-infobars",
		"--disable-notifications",
		url)
	chrome.Env = append(os.Environ(), "DISPLAY="+disp)
	chrome.Stderr = logWriter{m.logf}
	if err := chrome.Start(); err != nil {
		return fmt.Errorf("启动浏览器失败: %w", err)
	}
	p := m.procs[disp]
	p.chrome = chrome
	p.url = url
	p.w, p.h = w, h
	return nil
}

func (m *Manager) killChromeLocked(disp string) {
	p, ok := m.procs[disp]
	if !ok || p.chrome == nil {
		return
	}
	_ = p.chrome.Process.Kill()
	_, _ = p.chrome.Process.Wait()
	p.chrome = nil
}

// StopAll 停止所有虚拟显示器与浏览器
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for disp := range m.procs {
		m.stopLocked(disp)
	}
}

func (m *Manager) stopLocked(disp string) {
	p, ok := m.procs[disp]
	if !ok {
		return
	}
	if p.chrome != nil {
		_ = p.chrome.Process.Kill()
		_, _ = p.chrome.Process.Wait()
	}
	if p.xvfb != nil {
		_ = p.xvfb.Process.Kill()
		_, _ = p.xvfb.Process.Wait()
	}
	delete(m.procs, disp)
	cleanupDisplayFiles(disp)
}

// x11Socket 返回显示器的 X socket 路径，如 ":99" -> "/tmp/.X11-unix/X99"
func x11Socket(disp string) string {
	num := strings.TrimPrefix(disp, ":")
	return filepath.Join(os.TempDir(), ".X11-unix", "X"+num)
}

// logWriter 把子进程 stderr 逐行转发到日志（过滤 Chromium 常见噪音）
type logWriter struct {
	f func(string, ...interface{})
}

// 常见噪音：snap 的 AppArmor DBus 报错、GPU 相关警告等（不影响渲染）
var browserNoise = []string{
	"org.freedesktop.DBus",
	"gpu_blocklist",
	"maxDynamicUniformBuffers",
	"maxDynamicStorageBuffers",
	"Unable to get gpu adapter",
	"dbus/object_proxy",
}

func isBrowserNoise(line string) bool {
	for _, n := range browserNoise {
		if strings.Contains(line, n) {
			return true
		}
	}
	return false
}

func (w logWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		if t := strings.TrimSpace(line); t != "" && !isBrowserNoise(t) {
			w.f("[browser] %s", t)
		}
	}
	return len(p), nil
}
