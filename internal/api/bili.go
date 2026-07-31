package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ---- B 站直播助手 ----

// BiliQRCode 生成登录二维码
func (h *Handler) BiliQRCode(c *gin.Context) {
	if h.bili == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "B 站助手未启用"})
		return
	}
	qr, err := h.bili.GenerateQR()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": qr.URL, "qrcode_key": qr.QrcodeKey})
}

// BiliPoll 轮询二维码扫描状态
func (h *Handler) BiliPoll(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 key"})
		return
	}
	res, err := h.bili.PollQR(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// BiliStatus 登录状态 + 直播间信息
func (h *Handler) BiliStatus(c *gin.Context) {
	status := gin.H{"logged_in": h.bili.IsLoggedIn()}
	if status["logged_in"].(bool) {
		info, err := h.bili.GetRoomInfo()
		if err != nil {
			// 能登录但拿不到房间信息也不致命
			status["error"] = err.Error()
		} else {
			status["room_id"] = info.RoomID.String()
			status["title"] = info.Title
			status["is_live"] = info.LiveStatus == 1
			status["area_id"] = info.AreaID.String()
			status["parent_area_id"] = info.ParentAreaID.String()
		}
	}
	c.JSON(http.StatusOK, status)
}

// BiliAreas 直播分区列表
func (h *Handler) BiliAreas(c *gin.Context) {
	areas, err := h.bili.GetAreas()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"areas": areas})
}

type biliStartReq struct {
	Area string `json:"area"` // 子分区 ID
}

// BiliStart 开播，返回推流地址与密钥
func (h *Handler) BiliStart(c *gin.Context) {
	var req biliStartReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择直播分区"})
		return
	}
	res, err := h.bili.StartLive(req.Area)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// BiliStop 停播
func (h *Handler) BiliStop(c *gin.Context) {
	if err := h.bili.StopLive(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// BiliLogout 退出登录
func (h *Handler) BiliLogout(c *gin.Context) {
	if err := h.bili.Logout(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
