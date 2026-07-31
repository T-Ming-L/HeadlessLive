package api

import (
	"fmt"

	"github.com/T-Ming-L/HeadlessLive/internal/bilibili"
	"github.com/T-Ming-L/HeadlessLive/internal/browser"
	"github.com/T-Ming-L/HeadlessLive/internal/ffmpeg"
	"github.com/T-Ming-L/HeadlessLive/internal/logging"
	"github.com/T-Ming-L/HeadlessLive/internal/store"
	"github.com/T-Ming-L/HeadlessLive/internal/websocket"

	"github.com/gin-gonic/gin"
)

// SetupRouter 配置全部路由
func SetupRouter(st *store.Store, manager *ffmpeg.Manager, preview *ffmpeg.Preview,
	hub *websocket.Hub, uploadDir string, logFile *logging.FileLog, bili *bilibili.Client,
	browser *browser.Manager) *gin.Engine {
	r := gin.Default()

	handler := NewHandler(st, manager, preview, hub, uploadDir, bili, browser)

	// 浏览器管理器日志 → WebSocket + 日志文件
	if browser != nil {
		browser.SetLogger(func(format string, args ...interface{}) {
			emitLog := func(line string) {
				hub.BroadcastLog(line)
				if logFile != nil {
					logFile.WriteLine(line)
				}
			}
			emitLog(fmt.Sprintf(format, args...))
		})
	}

	// 日志广播到 WebSocket + 写入日志文件
	emitLog := func(line string) {
		hub.BroadcastLog(line)
		if logFile != nil {
			logFile.WriteLine(line)
		}
	}

	// FFmpeg 日志/状态 → WebSocket 推送
	manager.SetCallbacks(
		func(outputID, line string) {
			if outputID == "" {
				emitLog(line)
			} else {
				hub.BroadcastEvent("output_log", gin.H{"output_id": outputID, "line": line})
				if logFile != nil {
					logFile.WriteLine("[" + outputID + "] " + line)
				}
			}
		},
		func(status *ffmpeg.OutputStatus) {
			hub.BroadcastEvent("output_status", status)
		},
	)

	// 预览日志 → WebSocket + 文件
	preview.SetLogger(func(format string, args ...interface{}) {
		emitLog(fmt.Sprintf(format, args...))
	})

	// 预览进程退出 → 通知前端
	preview.SetOnExit(func() {
		hub.BroadcastEvent("preview_stopped", gin.H{})
	})

	api := r.Group("/api")
	{
		// 源
		api.GET("/sources", handler.ListSources)
		api.POST("/sources", handler.CreateSource)
		api.PUT("/sources/:id", handler.UpdateSource)
		api.DELETE("/sources/:id", handler.DeleteSource)
		api.POST("/sources/:id/probe", handler.ProbeSource)

		// 场景
		api.GET("/scenes", handler.ListScenes)
		api.POST("/scenes", handler.CreateScene)
		api.PUT("/scenes/:id", handler.UpdateScene)
		api.DELETE("/scenes/:id", handler.DeleteScene)
		api.POST("/scenes/:id/activate", handler.ActivateScene)

		// 输出
		api.GET("/outputs", handler.ListOutputs)
		api.POST("/outputs", handler.CreateOutput)
		api.PUT("/outputs/:id", handler.UpdateOutput)
		api.DELETE("/outputs/:id", handler.DeleteOutput)
		api.POST("/outputs/:id/start", handler.StartOutput)
		api.POST("/outputs/:id/stop", handler.StopOutput)

		// 状态
		api.GET("/status", handler.GetStatus)

		// 预览
		api.POST("/preview/start", handler.StartPreview)
		api.POST("/preview/stop", handler.StopPreview)

		// 上传
		api.POST("/upload/image", handler.UploadImage)
		api.POST("/upload/media", handler.UploadMedia)

		// 设备
		api.GET("/devices/video", handler.ListVideoDevices)
		api.GET("/devices/audio", handler.ListAudioDevices)
		api.GET("/devices/:dev/controls", handler.GetDeviceControls)
		api.PUT("/devices/:dev/control", handler.SetDeviceControl)

		// B 站直播助手（扫码登录 + 开播获取推流密钥）
		if bili != nil {
			api.GET("/bili/qrcode", handler.BiliQRCode)
			api.GET("/bili/poll", handler.BiliPoll)
			api.GET("/bili/status", handler.BiliStatus)
			api.GET("/bili/areas", handler.BiliAreas)
			api.POST("/bili/start", handler.BiliStart)
			api.POST("/bili/stop", handler.BiliStop)
			api.POST("/bili/logout", handler.BiliLogout)
		}
	}

	// MJPEG 预览流
	r.GET("/preview", handler.Preview)
	r.GET("/preview/frame", handler.PreviewFrame)

	// WebSocket（日志 + 状态推送）
	r.GET("/ws", handler.WebSocket)

	return r
}
