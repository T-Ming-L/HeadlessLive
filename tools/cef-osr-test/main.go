// CEF OSR 离屏渲染验证程序（Linux，需 Xvfb + ENERGY_HOME/libenergy）
//
// 目的：在 N100 上验证 energye/cef 的 OSR 模式能否做到——
//   1. 真透明：OnPaint 原始 buffer 中 alpha 通道是否保留（透明网页）
//   2. 帧率：SetWindowlessFrameRate 下 OnPaint 实际回调帧率
//   3. CPU：本进程 + chromium 子进程的总 CPU 占用
//
// 用法（Linux 服务器）：
//   ENERGY_HOME=/opt/cef DISPLAY=:99 ./cef-osr-test -fps 30 -duration 20 -log test.log
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/energye/cef/base"
	"github.com/energye/cef/cef"
	cefTypes "github.com/energye/cef/cef/types"
	"github.com/energye/lcl/api"
	"github.com/energye/lcl/api/libname"
	"github.com/energye/lcl/lcl"
	"github.com/energye/lcl/types"
)

// ---- 命令行参数 ----
var (
	flagURL      = flag.String("url", "", "要加载的网页 URL（默认生成本地透明测试页）")
	flagWidth    = flag.Int("width", 1280, "渲染宽")
	flagHeight   = flag.Int("height", 720, "渲染高")
	flagFPS      = flag.Int("fps", 30, "目标帧率（SetWindowlessFrameRate，1-60）")
	flagDuration = flag.Int("duration", 15, "测试秒数")
	flagLog      = flag.String("log", "", "日志文件路径（不填只输出 stdout）")
	flagVerbose  = flag.Bool("verbose", false, "输出 DEBUG 日志")
	flagLib      = flag.String("lib", "", "libenergy 动态库路径（默认 $ENERGY_HOME/libenergy-gtk3-147.so）")
)

// ---- 帧统计（OnPaint 回调线程）----
var (
	frameCount atomic.Int64 // 总帧数
	alphaMu    sync.Mutex
	alphaSum   [3]int64 // 透明 / 半透明 / 不透明 抽样累计
	alphaN     int64    // 抽样次数
	lastAlpha  [3]int64 // 最近一次抽样
)

var chromium cef.IChromium

func main() {
	flag.Parse()

	// 日志
	if err := InitLogger(*flagLog, *flagVerbose); err != nil {
		fmt.Println("日志初始化失败:", err)
		os.Exit(1)
	}
	defer Close()

	Infof("==== CEF OSR 验证程序 ====")
	Infof("url=%q width=%d height=%d fps=%d duration=%d", *flagURL, *flagWidth, *flagHeight, *flagFPS, *flagDuration)

	// libenergy 动态库
	lib := *flagLib
	if lib == "" {
		home := os.Getenv("ENERGY_HOME")
		if home == "" {
			Errorf("未设置 ENERGY_HOME，也未用 -lib 指定 libenergy 路径")
			os.Exit(1)
		}
		lib = filepath.Join(home, "libenergy-gtk3-147.so")
	}
	if _, err := os.Stat(lib); err != nil {
		Errorf("libenergy 不存在: %s（请检查 ENERGY_HOME / -lib）", lib)
		os.Exit(1)
	}
	libname.LibName = lib
	Infof("使用 libenergy: %s", lib)

	// DISPLAY 检查（CEF 在 Linux 上需要 X 环境，即使 OSR）
	if os.Getenv("DISPLAY") == "" {
		Warnf("未设置 DISPLAY，请在 Xvfb 下运行（如 DISPLAY=:99）")
	}

	lcl.Init()
	base.Init()
	Infof("CEF 版本: %s", cef.CEFVersion)

	app := cef.NewApplication()
	app.SetMultiThreadedMessageLoop(false)
	app.SetExternalMessagePump(false)
	app.SetWindowlessRenderingEnabled(true)
	app.SetNoSandbox(true) // root / 无沙箱环境必须

	// 页面 URL：默认生成一个"透明背景 + 红色方块 + 绿色文字"的本地测试页
	url := *flagURL
	if url == "" {
		url = writeTestPage()
		Infof("使用本地透明测试页: %s", url)
	}

	app.SetOnContextInitialized(func() {
		Infof("[CEF] OnContextInitialized")
		var handle cefTypes.TCefWindowHandle
		cef.MiscFunc.InitializeWindowHandle(&handle)

		chromium = cef.NewChromium(nil)
		chromium.SetDefaultUrl(url)
		chromium.Options().SetWindowlessFrameRate(int32(*flagFPS))
		// alpha=0 全透明背景 → 启用透明绘制（OSR 真透明的关键）
		chromium.Options().SetBackgroundColor(0)

		// 视口尺寸
		chromium.SetOnGetViewRect(func(_ lcl.IObject, _ cef.ICefBrowser, r *cef.TCefRect) {
			r.X, r.Y = 0, 0
			r.Width, r.Height = int32(*flagWidth), int32(*flagHeight)
		})
		chromium.SetOnGetScreenInfo(func(_ lcl.IObject, _ cef.ICefBrowser, i *cef.TCefScreenInfo, ok *bool) {
			i.DeviceScaleFactor = 1.0
			i.Depth = 24
			i.Rect = cef.TCefRect{Width: int32(*flagWidth), Height: int32(*flagHeight)}
			i.AvailableRect = i.Rect
			*ok = true
		})

		// OnPaint：原始像素 buffer（BGRA，每像素 4 字节，offset3 = alpha）
		chromium.SetOnPaint(func(_ lcl.IObject, _ cef.ICefBrowser, _ cefTypes.TCefPaintElementType,
			_ cefTypes.NativeUInt, _ cef.ICefRectArray, buf uintptr, w, h int32) {
			frameCount.Add(1)
			// 每 10 帧抽样一次 alpha
			if frameCount.Load()%10 == 0 {
				sampleAlpha(buf, w, h)
			}
		})

		chromium.SetOnLoadEnd(func(_ lcl.IObject, _ cef.ICefBrowser, _ cef.ICefFrame, _ int32) {
			Infof("[CEF] 页面加载完成")
		})
		chromium.SetOnLoadError(func(_ lcl.IObject, _ cef.ICefBrowser, _ cef.ICefFrame, errCode cefTypes.TCefErrorCode, errText, failedURL string) {
			Errorf("[CEF] 加载错误 %d %s (%s)", errCode, errText, failedURL)
		})

		// 创建无窗口 OSR 浏览器（forceAsPopup=true 与 tiny 示例一致）
		rect := types.TRect{}
		if !chromium.CreateBrowserWithWHandleRectStrRContextDValueBool(handle, rect, "osr-test", nil, nil, true) {
			Errorf("[CEF] 创建浏览器失败")
		} else {
			Infof("[CEF] 浏览器创建指令已发送")
		}
	})

	app.SetOnGetDefaultClient(func(client *cef.IEngClient) {
		if chromium != nil {
			*client = chromium.CefClient()
		}
	})

	start := time.Now()
	if !app.StartMainProcess() {
		Errorf("CEF 主进程启动失败（检查 libenergy 路径 / DISPLAY / ENERGY_HOME）")
		os.Exit(1)
	}
	Infof("CEF 主进程已启动，开始 %d 秒测试（目标 %d fps @ %dx%d）", *flagDuration, *flagFPS, *flagWidth, *flagHeight)

	// ---- 主循环：泵 CEF 消息 + 每秒统计 ----
	pump := time.NewTicker(8 * time.Millisecond)
	stat := time.NewTicker(1 * time.Second)
	done := time.After(time.Duration(*flagDuration) * time.Second)
	prevFrames := frameCount.Load()
	prevCPU := procTreeCPU(os.Getpid())
	wallPrev := time.Now()

loop:
	for {
		select {
		case <-pump.C:
			app.DoMessageLoopWork()
		case <-stat.C:
			now := time.Now()
			f := frameCount.Load()
			dt := now.Sub(wallPrev).Seconds()
			fps := float64(f-prevFrames) / dt
			wallPrev, prevFrames = now, f

			cpuNow := procTreeCPU(os.Getpid())
			cpuPct := float64(cpuNow-prevCPU) / (float64(clockTicks()) * dt) * 100.0
			prevCPU = cpuNow

			a0, a1, a2 := alphaSnapshot()
			Infof("fps=%.1f  cpu=%.1f%%  帧=%d  透明%%=%.2f 半透明%%=%.2f 不透明%%=%.2f",
				fps, cpuPct, f, a0, a1, a2)
		case <-done:
			break loop
		}
	}

	// ---- 汇总 ----
	elapsed := time.Since(start).Seconds()
	f := frameCount.Load()
	avgFPS := float64(f) / elapsed
	cpuTotal := procTreeCPU(os.Getpid())
	cpuAvg := float64(cpuTotal) / (float64(clockTicks()) * elapsed) * 100.0
	a0, a1, a2 := alphaSnapshot()
	Infof("==== 汇总 ====")
	Infof("总帧数: %d  平均 fps: %.1f（目标 %d）  测试时长: %.1fs", f, avgFPS, *flagFPS, elapsed)
	Infof("平均 CPU（含 chromium 子进程）: %.1f%%", cpuAvg)
	Infof("alpha 统计: 透明 %.2f%% / 半透明 %.2f%% / 不透明 %.2f%%", a0, a1, a2)
	if a0 > 50 {
		Infof("结论: 透明像素占比 %.2f%% > 50%% → OSR 真透明验证通过 ✅", a0)
	} else {
		Infof("结论: 透明像素占比偏低 → 可能未启用透明绘制（检查 SetBackgroundColor(0)）")
	}
	Infof("帧率达标: %v（avgFPS >= 目标*0.9）", avgFPS >= float64(*flagFPS)*0.9)

	app.QuitMessageLoop()
	Infof("测试结束")
}

// ---- alpha 抽样：统计 buffer 中透明/半透明/不透明像素占比 ----
// buffer 为 BGRA，每像素 4 字节，offset 3 = alpha
func sampleAlpha(buf uintptr, w, h int32) {
	stride := int(w) * 4
	var t, p, o int
	total := 0
	for y := 0; y < int(h); y += 4 {
		row := buf + uintptr(y*stride)
		for x := 0; x < int(w); x += 4 {
			a := *(*byte)(unsafe.Pointer(row + uintptr(x*4+3)))
			switch {
			case a == 0:
				t++
			case a < 255:
				p++
			default:
				o++
			}
			total++
		}
	}
	alphaMu.Lock()
	alphaSum[0] += int64(t)
	alphaSum[1] += int64(p)
	alphaSum[2] += int64(o)
	alphaN++
	lastAlpha[0], lastAlpha[1], lastAlpha[2] = int64(t), int64(p), int64(o)
	alphaMu.Unlock()
}

func alphaSnapshot() (t0, t1, t2 float64) {
	alphaMu.Lock()
	defer alphaMu.Unlock()
	total := alphaSum[0] + alphaSum[1] + alphaSum[2]
	if total == 0 {
		return 0, 0, 100
	}
	return float64(alphaSum[0]) / float64(total) * 100,
		float64(alphaSum[1]) / float64(total) * 100,
		float64(alphaSum[2]) / float64(total) * 100
}

// ---- CPU 统计：本进程 + 所有子进程（chromium 进程树）的 utime+stime ----
func clockTicks() int64 {
	// Linux HZ，通常是 100
	if v := os.Getenv("CLK_TCK"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 100
}

// readProcCPU 读 /proc/<pid>/stat 的 utime(14) + stime(15)
func readProcCPU(pid int) int64 {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	s := string(data)
	// comm 可能含括号和空格，从最后一个 ')' 之后取字段
	i := strings.LastIndexByte(s, ')')
	if i < 0 || i+2 >= len(s) {
		return 0
	}
	fields := strings.Fields(s[i+2:])
	// 从 comm 后的第一个字段(state)算起：utime=字段14→偏移11，stime=字段15→偏移12
	if len(fields) < 13 {
		return 0
	}
	ut, _ := strconv.ParseInt(fields[11], 10, 64) // utime
	st, _ := strconv.ParseInt(fields[12], 10, 64) // stime
	return ut + st
}

// procTreeCPU 递归统计 pid 及其直接子进程（一层即可覆盖 chromium 主渲染进程）
func procTreeCPU(pid int) int64 {
	total := readProcCPU(pid)
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cp, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		// 读 /proc/<cp>/stat 的 ppid（第 4 字段 → fields[1]）
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		s := string(data)
		i := strings.LastIndexByte(s, ')')
		if i < 0 || i+2 >= len(s) {
			continue
		}
		fields := strings.Fields(s[i+2:])
		if len(fields) < 2 {
			continue
		}
		ppid, _ := strconv.ParseInt(fields[1], 10, 64)
		if ppid == int64(pid) {
			total += readProcCPU(cp)
		}
	}
	return total
}

// ---- 本地透明测试页 ----
func writeTestPage() string {
	html := `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><style>
  html, body { background: transparent; margin: 0; }
  .box { width: 160px; height: 160px; background: #ff0000; margin: 120px auto 0; }
  .txt { color: #00ff00; font: bold 48px sans-serif; text-align: center; margin-top: 30px; }
</style></head>
<body>
  <div class="box"></div>
  <div class="txt">HELLO OSR</div>
</body>
</html>`
	path := filepath.Join(os.TempDir(), "cef-osr-test-page.html")
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		Errorf("写测试页失败: %v", err)
		return "about:blank"
	}
	return "file://" + path
}
