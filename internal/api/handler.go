package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/bilibili"
	"github.com/T-Ming-L/HeadlessLive/internal/browser"
	"github.com/T-Ming-L/HeadlessLive/internal/capture"
	"github.com/T-Ming-L/HeadlessLive/internal/ffmpeg"
	"github.com/T-Ming-L/HeadlessLive/internal/model"
	"github.com/T-Ming-L/HeadlessLive/internal/scene"
	"github.com/T-Ming-L/HeadlessLive/internal/store"
	"github.com/T-Ming-L/HeadlessLive/internal/websocket"

	"github.com/gin-gonic/gin"
)

// Handler API 处理器
type Handler struct {
	store     *store.Store
	manager   *ffmpeg.Manager
	preview   *ffmpeg.Preview
	hub       *websocket.Hub
	uploadDir string
	bili      *bilibili.Client
	browser   *browser.Manager
}

// NewHandler 创建处理器
func NewHandler(st *store.Store, manager *ffmpeg.Manager, preview *ffmpeg.Preview,
	hub *websocket.Hub, uploadDir string, bili *bilibili.Client, browser *browser.Manager) *Handler {
	os.MkdirAll(uploadDir, 0755)
	return &Handler{
		store:     st,
		manager:   manager,
		preview:   preview,
		hub:       hub,
		uploadDir: uploadDir,
		bili:      bili,
		browser:   browser,
	}
}

// ensureBrowsers 渲染规格含浏览器源时，自动启动 Xvfb + Chromium（无需手动拉起）。
// 返回第一个失败原因（调用方可据此决定是否剔除浏览器源）。
func (h *Handler) ensureBrowsers(rs *scene.RenderSpec) error {
	if h.browser == nil || rs == nil {
		return nil
	}
	var firstErr error
	for _, in := range rs.Inputs {
		if in.Kind != scene.InputX11 || in.Source == nil || in.Source.Type != model.SourceBrowser {
			continue
		}
		disp := in.Source.Display
		if disp == "" {
			disp = ":99"
		}
		if err := h.browser.Ensure(disp, in.Source.URL, in.Source.BrowserW, in.Source.BrowserH); err != nil {
			h.hub.BroadcastLog("[browser] " + err.Error())
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// buildSpec 构建渲染规格。浏览器源无法启动时自动剔除（其余源照常渲染），
// 避免 x11grab 连不上 :99 导致整个预览/推流失败，并返回剔除原因供前端提示。
func (h *Handler) buildSpec(sc *model.Scene) (*scene.RenderSpec, []string, error) {
	rs, err := h.renderSpec(sc)
	if err != nil {
		return nil, nil, err
	}
	if h.browser == nil {
		return rs, nil, nil
	}
	if err := h.ensureBrowsers(rs); err != nil {
		msg := "浏览器源已剔除：" + err.Error()
		h.hub.BroadcastLog("[browser] ⚠️ " + msg)
		rs2, err2 := h.renderSpecNoBrowser(sc)
		if err2 != nil {
			return nil, nil, err2
		}
		return rs2, []string{msg}, nil
	}
	return rs, nil, nil
}

// renderSpecNoBrowser 构建不含浏览器源的渲染规格
func (h *Handler) renderSpecNoBrowser(sc *model.Scene) (*scene.RenderSpec, error) {
	srcs := make(map[string]*model.Source)
	for _, s := range h.store.Data().Sources {
		if s.Type == model.SourceBrowser {
			continue
		}
		srcs[s.ID] = s
	}
	return scene.Build(sc, srcs)
}

// logSkipped 将构建时被跳过的源广播到日志（帮助定位"为什么没显示"）
func (h *Handler) logSkipped(rs *scene.RenderSpec) {
	if rs == nil {
		return
	}
	for _, s := range rs.Skipped {
		h.hub.BroadcastLog("[api] ⚠️ 已跳过不可用的源: " + s)
	}
}

// genID 生成带前缀的短 ID
func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%100000000)
}

// renderSpec 将场景编译为渲染规格
func (h *Handler) renderSpec(sc *model.Scene) (*scene.RenderSpec, error) {
	srcs := make(map[string]*model.Source)
	for _, s := range h.store.Data().Sources {
		srcs[s.ID] = s
	}
	return scene.Build(sc, srcs)
}

// resolveScene 按 ID 或当前活动场景解析
func (h *Handler) resolveScene(id string) *model.Scene {
	if id != "" {
		if sc := h.store.Data().FindScene(id); sc != nil {
			return sc
		}
	}
	return h.store.Data().GetCurrentScene()
}

// --- 源 CRUD ---

// ListSources 列出所有源
func (h *Handler) ListSources(c *gin.Context) {
	c.JSON(http.StatusOK, h.store.Data().Sources)
}

// CreateSource 创建源
func (h *Handler) CreateSource(c *gin.Context) {
	var src model.Source
	if err := c.ShouldBindJSON(&src); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if src.Name == "" {
		src.Name = string(src.Type)
	}
	if src.ID == "" {
		src.ID = genID("src")
	}
	if err := h.store.AddSource(&src); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, src)
}

// UpdateSource 更新源
func (h *Handler) UpdateSource(c *gin.Context) {
	id := c.Param("id")
	var src model.Source
	if err := c.ShouldBindJSON(&src); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	src.ID = id
	if err := h.store.UpdateSource(&src); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, src)
}

// DeleteSource 删除源
func (h *Handler) DeleteSource(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteSource(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ProbeSource 探测源属性（采集卡分辨率/格式，媒体文件信息）
func (h *Handler) ProbeSource(c *gin.Context) {
	id := c.Param("id")
	src := h.store.Data().FindSource(id)
	if src == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "源不存在"})
		return
	}

	switch src.Type {
	case model.SourceVideoDevice:
		info := capture.Probe(src.DevicePath, 1920, 1080, 30)
		c.JSON(http.StatusOK, info)
	case model.SourceMediaFile:
		info := probeFile(src.FilePath)
		c.JSON(http.StatusOK, info)
	default:
		c.JSON(http.StatusOK, gin.H{"type": src.Type, "message": "该源类型无需探测"})
	}
}

// --- 场景 CRUD ---

// ListScenes 列出所有场景
func (h *Handler) ListScenes(c *gin.Context) {
	c.JSON(http.StatusOK, h.store.Data().Scenes)
}

// CreateScene 创建场景
func (h *Handler) CreateScene(c *gin.Context) {
	var sc model.Scene
	if err := c.ShouldBindJSON(&sc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if sc.CanvasW <= 0 {
		sc.CanvasW = 1920
	}
	if sc.CanvasH <= 0 {
		sc.CanvasH = 1080
	}
	if sc.FPS <= 0 {
		sc.FPS = 30
	}
	if sc.Items == nil {
		sc.Items = []model.SceneItem{}
	}
	if sc.ID == "" {
		sc.ID = genID("scene")
	}
	if err := h.store.AddScene(&sc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sc)
}

// UpdateScene 更新场景（含场景项）
func (h *Handler) UpdateScene(c *gin.Context) {
	id := c.Param("id")
	var sc model.Scene
	if err := c.ShouldBindJSON(&sc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	sc.ID = id
	if err := h.store.UpdateScene(&sc); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sc)
}

// DeleteScene 删除场景
func (h *Handler) DeleteScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.DeleteScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ActivateScene 切换当前活动场景
func (h *Handler) ActivateScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.store.SetCurrentScene(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	h.hub.BroadcastEvent("scene_changed", gin.H{"scene_id": id})
	c.JSON(http.StatusOK, gin.H{"message": "已切换", "scene_id": id})
}
