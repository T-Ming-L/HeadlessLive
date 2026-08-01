package api

import (
	"net/http"

	"github.com/T-Ming-L/HeadlessLive/internal/model"

	"github.com/gin-gonic/gin"
)

// --- 输出 CRUD ---

// ListOutputs 列出所有输出
func (h *Handler) ListOutputs(c *gin.Context) {
	c.JSON(http.StatusOK, h.store.Data().Outputs)
}

// CreateOutput 创建输出
func (h *Handler) CreateOutput(c *gin.Context) {
	var o model.Output
	if err := c.ShouldBindJSON(&o); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if o.ID == "" {
		o.ID = genID("out")
	}
	if err := h.store.AddOutput(&o); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}

// UpdateOutput 更新输出
func (h *Handler) UpdateOutput(c *gin.Context) {
	id := c.Param("id")
	var o model.Output
	if err := c.ShouldBindJSON(&o); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	o.ID = id
	if err := h.store.UpdateOutput(&o); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}

// DeleteOutput 删除输出
func (h *Handler) DeleteOutput(c *gin.Context) {
	id := c.Param("id")
	// 运行中先停止
	h.manager.StopOutput(id)
	if err := h.store.DeleteOutput(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// StartOutput 启动输出（推流/录制）
func (h *Handler) StartOutput(c *gin.Context) {
	id := c.Param("id")
	o := h.store.Data().FindOutput(id)
	if o == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "输出不存在"})
		return
	}

	sc := h.resolveScene(o.SceneID)
	if sc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有可用的场景，请先创建场景"})
		return
	}

	rs, err := h.buildSpec(sc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "场景渲染规格构建失败: " + err.Error()})
		return
	}
	h.logSkipped(rs)

	// 同一视频设备只允许一个采集进程（两个 FFmpeg 同时读 v4l2 会占满 USB 带宽导致系统卡死），
	// 推流前先自动停止预览 + 其它运行中的输出
	if h.preview.IsRunning() {
		h.preview.Stop()
		h.hub.BroadcastLog("[api] 推流启动，已自动停止预览（避免采集设备并发占用）")
	}
	h.manager.StopAll()

	if err := h.manager.StartOutput(rs, o); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.hub.BroadcastEvent("output_started", gin.H{"output_id": o.ID, "name": o.Name})
	c.JSON(http.StatusOK, gin.H{"message": "输出已启动", "output_id": o.ID})
}

// StopOutput 停止输出
func (h *Handler) StopOutput(c *gin.Context) {
	id := c.Param("id")
	h.manager.StopOutput(id)
	h.hub.BroadcastEvent("output_stopped", gin.H{"output_id": id})
	c.JSON(http.StatusOK, gin.H{"message": "输出已停止"})
}

// --- 预览 ---

// StartPreviewRequest 预览启动参数
type StartPreviewRequest struct {
	SceneID string `json:"scene_id,omitempty"`
	MaxW    int    `json:"max_w,omitempty"` // 预览最大宽度，0=不缩放
}

// StartPreview 启动 MJPEG 预览
func (h *Handler) StartPreview(c *gin.Context) {
	var req StartPreviewRequest
	_ = c.ShouldBindJSON(&req) // 参数可选

	sc := h.resolveScene(req.SceneID)
	if sc == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有可用的场景"})
		return
	}
	rs, err := h.buildSpec(sc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "场景渲染规格构建失败: " + err.Error()})
		return
	}
	h.logSkipped(rs)
	// 同一视频设备只允许一个采集进程（避免并发读 v4l2 占满带宽导致卡死），
	// 启动预览前先停掉运行中的输出
	if len(h.manager.GetAllStatus()) > 0 {
		h.manager.StopAll()
		h.hub.BroadcastLog("[api] 启动预览，已停止运行中的输出")
	}
	if err := h.preview.Start(rs, req.MaxW); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "预览已启动"})
}

// StopPreview 停止预览
func (h *Handler) StopPreview(c *gin.Context) {
	h.preview.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "预览已停止"})
}

// Preview 提供 MJPEG 预览流（GET /preview）
func (h *Handler) Preview(c *gin.Context) {
	h.preview.Handler(c.Writer, c.Request)
}

// PreviewFrame 提供最新一帧 JPEG（GET /preview/frame，单帧轮询用）
func (h *Handler) PreviewFrame(c *gin.Context) {
	h.preview.FrameHandler(c.Writer, c.Request)
}

// --- 状态 ---

// GetStatus 系统全量状态（源/场景/输出 + 运行状态）
func (h *Handler) GetStatus(c *gin.Context) {
	data := h.store.Data()

	statusMap := make(map[string]interface{})
	for _, o := range data.Outputs {
		statusMap[o.ID] = h.manager.GetStatus(o.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"sources":        data.Sources,
		"scenes":         data.Scenes,
		"outputs":        data.Outputs,
		"current_scene":  data.CurrentScene,
		"output_status":  statusMap,
		"preview_running": h.preview.IsRunning(),
	})
}
