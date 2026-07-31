package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/capture"

	"github.com/gin-gonic/gin"
)

// --- 设备 ---

// ListVideoDevices 扫描 v4l2 视频设备
func (h *Handler) ListVideoDevices(c *gin.Context) {
	c.JSON(http.StatusOK, capture.ListVideoDevices())
}

// ListAudioDevices 扫描音频设备
func (h *Handler) ListAudioDevices(c *gin.Context) {
	c.JSON(http.StatusOK, capture.ListAudioDevices())
}

// GetDeviceControls 获取设备控制项（亮度/对比度等）
func (h *Handler) GetDeviceControls(c *gin.Context) {
	dev := c.Param("dev")
	if !strings.HasPrefix(dev, "/") {
		dev = "/dev/" + dev
	}
	ctrls, err := capture.GetControls(dev)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"controls": []capture.DeviceControl{}, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"controls": ctrls, "device": dev})
}

// SetDeviceControl 设置设备控制项
func (h *Handler) SetDeviceControl(c *gin.Context) {
	dev := c.Param("dev")
	if !strings.HasPrefix(dev, "/") {
		dev = "/dev/" + dev
	}
	var req struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if err := capture.SetControl(dev, req.Name, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// --- 上传 ---

var allowedImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true,
}
var allowedMediaExt = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".flv": true, ".webm": true,
	".mp3": true, ".aac": true, ".wav": true, ".flac": true, ".ogg": true,
}

// UploadImage 上传图片
func (h *Handler) UploadImage(c *gin.Context) {
	h.handleUpload(c, "images", allowedImageExt)
}

// UploadMedia 上传媒体文件
func (h *Handler) UploadMedia(c *gin.Context) {
	h.handleUpload(c, "media", allowedMediaExt)
}

func (h *Handler) handleUpload(c *gin.Context, subDir string, allowed map[string]bool) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件字段 file"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型: " + ext})
		return
	}

	dir := filepath.Join(h.uploadDir, subDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	name := fmt.Sprintf("%d%s", time.Now().UnixMilli(), ext)
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": dst, "filename": file.Filename})
}

// --- WebSocket ---

// WebSocket 处理 WebSocket 连接
func (h *Handler) WebSocket(c *gin.Context) {
	h.hub.HandleConnection(c.Writer, c.Request)
}

// --- 探测辅助 ---

// FileProbe 媒体文件探测结果
type FileProbe struct {
	Path       string  `json:"path"`
	Exists     bool    `json:"exists"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
	Duration   float64 `json:"duration"`
	HasVideo   bool    `json:"has_video"`
	HasAudio   bool    `json:"has_audio"`
	Error      string  `json:"error,omitempty"`
}

// probeFile 用 ffprobe 探测媒体文件
func probeFile(path string) *FileProbe {
	p := &FileProbe{Path: path}
	if _, err := os.Stat(path); err != nil {
		p.Error = "文件不存在"
		return p
	}
	p.Exists = true

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json",
		"-show_streams", "-show_format", path)
	out, err := cmd.Output()
	if err != nil {
		p.Error = fmt.Sprintf("ffprobe 失败: %v", err)
		return p
	}
	parseProbeJSON(out, p)
	return p
}

func parseProbeJSON(data []byte, p *FileProbe) {
	// 简易解析：只提取需要的字段，避免引入额外依赖
	text := string(data)
	// duration
	if d := jsonField(text, `"duration": "`); d != "" {
		fmt.Sscanf(d, "%f", &p.Duration)
	}
	// 遍历 streams
	for _, stream := range strings.Split(text, `"codec_type":`) {
		if strings.HasPrefix(stream, `"video"`) {
			p.HasVideo = true
			if v := jsonField(stream, `"width": `); v != "" {
				fmt.Sscanf(v, "%d", &p.Width)
			}
			if v := jsonField(stream, `"height": `); v != "" {
				fmt.Sscanf(v, "%d", &p.Height)
			}
			if v := jsonField(stream, `"avg_frame_rate": "`); v != "" {
				var num, den float64
				if _, err := fmt.Sscanf(v, "%f/%f", &num, &den); err == nil && den > 0 {
					p.FPS = num / den
				}
			}
		} else if strings.HasPrefix(stream, `"audio"`) {
			p.HasAudio = true
		}
	}
}

// jsonField 简易 JSON 字段提取（仅用于 ffprobe 输出）
func jsonField(text, key string) string {
	idx := strings.Index(text, key)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(key):]
	end := strings.IndexAny(rest, "\",\n}")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}
