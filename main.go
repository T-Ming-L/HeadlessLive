package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/api"
	"github.com/T-Ming-L/HeadlessLive/internal/bilibili"
	"github.com/T-Ming-L/HeadlessLive/internal/browser"
	"github.com/T-Ming-L/HeadlessLive/internal/config"
	"github.com/T-Ming-L/HeadlessLive/internal/ffmpeg"
	"github.com/T-Ming-L/HeadlessLive/internal/logging"
	"github.com/T-Ming-L/HeadlessLive/internal/store"
	"github.com/T-Ming-L/HeadlessLive/internal/websocket"

	"github.com/gin-gonic/gin"
)

//go:embed all:static
var staticFiles embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "服务器配置文件路径")
	scenesPath := flag.String("scenes", "scenes.yaml", "场景/源/输出配置路径")
	portFlag := flag.Int("port", 0, "HTTP 端口（覆盖 config.yaml，0 = 用配置）")
	flag.Parse()

	fmt.Println("=== HeadlessLive — 无头直播演播室（Headless OBS，Web 控制）===")

	// 服务器配置（端口 + debug/log 开关）
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 场景配置（源/场景/输出，持久化为 YAML）
	st, err := store.Load(*scenesPath)
	if err != nil {
		log.Fatalf("加载场景配置失败: %v", err)
	}

	// FFmpeg 多输出管理器 + MJPEG 预览
	manager := ffmpeg.NewManager()
	manager.SetDebug(cfg.Server.Debug)
	preview := ffmpeg.NewPreview()
	preview.SetDebug(cfg.Server.Debug)

	// 文件日志（logs/ 目录，config 开关控制）
	var logFile *logging.FileLog
	if cfg.Server.Log {
		logFile, err = logging.Open("logs")
		if err != nil {
			log.Fatalf("创建日志文件失败: %v", err)
		}
		defer logFile.Close()
	}

	// WebSocket 中心
	hub := websocket.NewHub()

	// B 站直播助手（扫码登录 + 开播取推流密钥，会话持久化到本地文件）
	bili := bilibili.NewClient("bilibili_session.json")

	// 浏览器源管理器（自动启动 Xvfb + Chromium）
	bm := browser.NewManager()

	// 路由
	router := api.SetupRouter(st, manager, preview, hub, "uploads", logFile, bili, bm)

	// 嵌入的静态文件（Vite 构建产物：static/index.html + static/assets/*）
	// 放在 NoRoute，仅兜底非 API 路径
	router.NoRoute(staticHandler())

	// 退出信号清理（带整体超时：ffmpeg 卡死时也能强制退出）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		fmt.Println("\n[main] 收到退出信号，正在清理...")
		done := make(chan struct{})
		go func() {
			manager.StopAll()
			preview.Stop()
			bm.StopAll()
			close(done)
		}()
		select {
		case <-done:
			fmt.Println("[main] 清理完成")
		case <-time.After(5 * time.Second):
			fmt.Println("[main] 清理超时，强制退出")
		}
		os.Exit(0)
	}()

	port := cfg.Server.Port
	if *portFlag > 0 {
		port = *portFlag
	}
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("[main] 服务启动于 http://0.0.0.0%s\n", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// staticHandler 提供 embed 的静态文件（/ 与 /assets/*）
func staticHandler() gin.HandlerFunc {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("读取静态资源失败: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return func(c *gin.Context) {
		name := strings.TrimPrefix(c.Request.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		// 文件存在则提供，否则 404
		if _, err := fs.Stat(sub, name); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
