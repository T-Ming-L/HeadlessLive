// Package bilibili 提供 B 站直播助手能力：扫码登录、获取直播间信息、开播获取推流地址/密钥、停播。
//
// 接口协议参考 BiliLive-Utility（AGPL-3.0）所调用的 B 站公开接口，本包为独立 Go 重新实现。
// APPKEY/APPSEC 为 B 站 Election 客户端公开的固定凭据，用于接口签名。
package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// 固定凭据与接口地址（B 站 Election 客户端）
const (
	AppKey = "aa1e74ee4874176e"
	AppSec = "54e6a9a31b911cd5fc0daa66ebf94bc4"
	// 开播使用的 PC 直播端构建号（BiliLive-Utility 同样写死此值）
	pcLinkBuild = "1001017006"

	urlQRGenerate  = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
	urlQRPoll      = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll"
	urlNav         = "https://api.bilibili.com/x/web-interface/nav"
	urlRoomInfoOld = "https://api.live.bilibili.com/room/v1/Room/getRoomInfoOld"
	urlRoomInfo    = "https://api.live.bilibili.com/room/v1/Room/get_info"
	urlAreas       = "https://api.live.bilibili.com/room/v1/Area/getList"
	urlStartLive   = "https://api.live.bilibili.com/room/v1/Room/startLive"
	urlStopLive    = "https://api.live.bilibili.com/room/v1/Room/stopLive"
	urlLogout      = "https://passport.bilibili.com/login/exit/v2"
)

var defaultHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36 Edg/144.0.0.0",
	"Accept":     "application/json, text/plain, */*",
	"Origin":     "https://link.bilibili.com",
	"Referer":    "https://link.bilibili.com/p/center/index",
}

// Client B 站 API 客户端（手动管理 Cookie，跨子域一致；持久化到会话文件）
type Client struct {
	mu      sync.Mutex
	hc      *http.Client
	cookies map[string]string
	session string // 会话持久化文件路径
	roomID  string // room_id 缓存
}

// NewClient 创建客户端并尝试加载已保存的会话
func NewClient(sessionPath string) *Client {
	c := &Client{
		hc:      &http.Client{Timeout: 25 * time.Second},
		cookies: map[string]string{},
		session: sessionPath,
	}
	if data, err := os.ReadFile(sessionPath); err == nil {
		var m map[string]string
		if json.Unmarshal(data, &m) == nil && len(m) > 0 {
			c.cookies = m
		}
	}
	return c
}

// ---- 内部 HTTP 封装 ----

// do 发送请求：附加 Cookie 头，合并响应 Set-Cookie 并持久化
func (c *Client) do(method, u string, params url.Values, form url.Values) (*http.Response, error) {
	if params != nil {
		u += "?" + params.Encode()
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	req.Header.Set("Cookie", c.cookieStringLocked())
	c.mu.Unlock()
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	// 合并 Set-Cookie（扫码登录、跳转等都会种 Cookie）
	if cks := resp.Cookies(); len(cks) > 0 {
		c.mu.Lock()
		changed := false
		for _, ck := range cks {
			if ck.Value != "" && c.cookies[ck.Name] != ck.Value {
				c.cookies[ck.Name] = ck.Value
				changed = true
			}
		}
		if changed {
			c.saveLocked()
		}
		c.mu.Unlock()
	}
	return resp, nil
}

func (c *Client) get(u string, params map[string]string) (*http.Response, error) {
	p := url.Values{}
	for k, v := range params {
		p.Set(k, v)
	}
	return c.do(http.MethodGet, u, p, nil)
}

func (c *Client) postForm(u string, form map[string]string) (*http.Response, error) {
	f := url.Values{}
	for k, v := range form {
		f.Set(k, v)
	}
	return c.do(http.MethodPost, u, nil, f)
}

// readJSON 读取响应并解析为标准 B 站返回 {code, message, data}
func readJSON(resp *http.Response, out interface{}) (int, string, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, "", err
	}
	var box struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &box); err != nil {
		return -1, "", fmt.Errorf("解析响应失败: %v", err)
	}
	if out != nil && len(box.Data) > 0 {
		if err := json.Unmarshal(box.Data, out); err != nil {
			return box.Code, box.Message, fmt.Errorf("解析 data 失败: %v", err)
		}
	}
	return box.Code, box.Message, nil
}

// ---- Cookie 管理 ----

func (c *Client) cookieStringLocked() string {
	keys := make([]string, 0, len(c.cookies))
	for k := range c.cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+c.cookies[k])
	}
	return strings.Join(parts, "; ")
}

// CookieString 返回完整 Cookie 字符串（给前端展示/调试用）
func (c *Client) CookieString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cookieStringLocked()
}

// Cookie 读取单个 Cookie 值
func (c *Client) Cookie(name string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cookies[name]
}

func (c *Client) saveLocked() {
	if c.session == "" {
		return
	}
	data, _ := json.MarshalIndent(c.cookies, "", "  ")
	_ = os.WriteFile(c.session, data, 0600)
}

func (c *Client) clearLocked() {
	c.cookies = map[string]string{}
	c.roomID = ""
	if c.session != "" {
		_ = os.Remove(c.session)
	}
}

// ---- 签名 ----

// sign 生成 APP 签名：参数按 key 排序后 urlencode，追加 appsec 后 MD5
func sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if b.Len() > 0 {
			b.WriteString("&")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(url.QueryEscape(params[k]))
	}
	h := md5.Sum([]byte(b.String() + AppSec))
	return hex.EncodeToString(h[:])
}

// ---- 扫码登录 ----

// QRCode 登录二维码数据
type QRCode struct {
	URL       string `json:"url"`
	QrcodeKey string `json:"qrcode_key"`
}

// GenerateQR 生成登录二维码
func (c *Client) GenerateQR() (*QRCode, error) {
	resp, err := c.get(urlQRGenerate, nil)
	if err != nil {
		return nil, err
	}
	var d struct {
		URL       string `json:"url"`
		QrcodeKey string `json:"qrcode_key"`
	}
	code, msg, err := readJSON(resp, &d)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("生成二维码失败: %s", msg)
	}
	return &QRCode{URL: d.URL, QrcodeKey: d.QrcodeKey}, nil
}

// PollResult 二维码轮询结果
type PollResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	RoomID  string `json:"room_id,omitempty"`
}

// 二维码轮询状态码
const (
	QRNotScanned   = 86101 // 未扫码
	QRScanned      = 86090 // 已扫码未确认
	QRExpired      = 86038 // 二维码过期
	QRLoginSuccess = 0     // 登录成功
)

// PollQR 轮询二维码扫描状态；登录成功后自动保存 Cookie 并获取房间号
func (c *Client) PollQR(key string) (*PollResult, error) {
	resp, err := c.get(urlQRPoll, map[string]string{"qrcode_key": key})
	if err != nil {
		return nil, err
	}
	var d struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_, _, err = readJSON(resp, &d)
	if err != nil {
		return nil, err
	}
	res := &PollResult{Code: d.Code, Message: d.Message}
	if d.Code == QRLoginSuccess {
		roomID, err := c.GetRoomID()
		if err != nil {
			return nil, err
		}
		res.RoomID = roomID
	}
	return res, nil
}

// IsLoggedIn 通过 nav 接口检查登录状态
func (c *Client) IsLoggedIn() bool {
	resp, err := c.get(urlNav, nil)
	if err != nil {
		return false
	}
	var d struct {
		IsLogin bool `json:"isLogin"`
	}
	_, _, err = readJSON(resp, &d)
	return err == nil && d.IsLogin
}

// GetRoomID 获取当前账号直播间 ID（从 Cookie 中的 DedeUserID 查询）
func (c *Client) GetRoomID() (string, error) {
	c.mu.Lock()
	if c.roomID != "" {
		c.mu.Unlock()
		return c.roomID, nil
	}
	mid := c.cookies["DedeUserID"]
	c.mu.Unlock()
	if mid == "" {
		return "", fmt.Errorf("未登录（缺少 DedeUserID）")
	}
	resp, err := c.get(urlRoomInfoOld, map[string]string{"mid": mid})
	if err != nil {
		return "", err
	}
	var d struct {
		RoomID json.Number `json:"roomid"`
	}
	code, msg, err := readJSON(resp, &d)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("获取直播间 ID 失败: %s", msg)
	}
	if d.RoomID.String() == "" || d.RoomID.String() == "0" {
		return "", fmt.Errorf("该账号没有直播间")
	}
	c.mu.Lock()
	c.roomID = d.RoomID.String()
	c.mu.Unlock()
	return c.roomID, nil
}

// RoomInfo 直播间信息
type RoomInfo struct {
	RoomID    json.Number `json:"room_id"`
	Title     string      `json:"title"`
	LiveStatus int        `json:"live_status"` // 0 未开播 1 直播中
	ParentAreaID json.Number `json:"parent_area_id"`
	AreaID    json.Number `json:"area_id"`
	Tags      string      `json:"tags"`
}

// GetRoomInfo 获取直播间信息
func (c *Client) GetRoomInfo() (*RoomInfo, error) {
	roomID, err := c.GetRoomID()
	if err != nil {
		return nil, err
	}
	resp, err := c.get(urlRoomInfo, map[string]string{"room_id": roomID})
	if err != nil {
		return nil, err
	}
	var d RoomInfo
	code, msg, err := readJSON(resp, &d)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("获取直播间信息失败: %s", msg)
	}
	return &d, nil
}

// Area 直播分区（父分区含子分区列表）
type Area struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
	List []Area      `json:"list"`
}

// GetAreas 获取直播分区列表
func (c *Client) GetAreas() ([]Area, error) {
	resp, err := c.get(urlAreas, nil)
	if err != nil {
		return nil, err
	}
	var d []Area
	code, msg, err := readJSON(resp, &d)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("获取分区列表失败: %s", msg)
	}
	return d, nil
}

// StartLiveResult 开播结果
type StartLiveResult struct {
	RTMPAddr string `json:"rtmp_addr"` // 推流地址（如 rtmp://live-push.bilivideo.com/live-bvc/）
	RTMPCode string `json:"rtmp_code"` // 推流密钥（?streamname=...&key=...）
	QR       string `json:"qr,omitempty"` // 需要人脸验证时的二维码链接
	Message  string `json:"message,omitempty"`
}

// StartLive 开播，返回推流地址与密钥
func (c *Client) StartLive(areaID string) (*StartLiveResult, error) {
	roomID, err := c.GetRoomID()
	if err != nil {
		return nil, err
	}
	csrf := c.Cookie("bili_jct")
	if csrf == "" {
		return nil, fmt.Errorf("Cookie 缺少 bili_jct，请重新登录")
	}
	data := map[string]string{
		"room_id":  roomID,
		"platform": "web_electron_link",
		"area_v2":  areaID,
		"csrf":     csrf,
		"ts":       fmt.Sprintf("%d", time.Now().Unix()),
		"build":    pcLinkBuild,
		"appkey":   AppKey,
	}
	data["sign"] = sign(data)

	resp, err := c.postForm(urlStartLive, data)
	if err != nil {
		return nil, err
	}
	var d struct {
		RTMP struct {
			Addr string `json:"addr"`
			Code string `json:"code"`
		} `json:"rtmp"`
		QR string `json:"qr"`
	}
	code, msg, err := readJSON(resp, &d)
	if err != nil {
		return nil, err
	}
	if code == 0 {
		if d.RTMP.Addr == "" || d.RTMP.Code == "" {
			return nil, fmt.Errorf("开播成功但未返回推流信息")
		}
		return &StartLiveResult{RTMPAddr: d.RTMP.Addr, RTMPCode: d.RTMP.Code}, nil
	}
	// 60024/60043 = 需要人脸验证，返回二维码
	if code == 60024 || code == 60043 {
		qr := d.QR
		if qr == "" {
			mid := c.Cookie("DedeUserID")
			qr = fmt.Sprintf("https://www.bilibili.com/blackboard/live/face-auth-middle.html?source_event=400&mid=%s", mid)
		}
		return &StartLiveResult{QR: qr, Message: msg}, nil
	}
	return nil, fmt.Errorf("开播失败: %s", msg)
}

// StopLive 停播
func (c *Client) StopLive() error {
	roomID, err := c.GetRoomID()
	if err != nil {
		return err
	}
	csrf := c.Cookie("bili_jct")
	if csrf == "" {
		return fmt.Errorf("Cookie 缺少 bili_jct，请重新登录")
	}
	resp, err := c.postForm(urlStopLive, map[string]string{
		"room_id":    roomID,
		"platform":   "web_electron_link",
		"csrf_token": csrf,
		"csrf":       csrf,
		"visit_id":   "",
	})
	if err != nil {
		return err
	}
	code, msg, err := readJSON(resp, nil)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("停播失败: %s", msg)
	}
	return nil
}

// Logout 退出登录并清除本地会话
func (c *Client) Logout() error {
	csrf := c.Cookie("bili_jct")
	if csrf != "" {
		resp, err := c.postForm(urlLogout, map[string]string{
			"biliCSRF": csrf,
			"gourl":    "https://www.bilibili.com/",
		})
		if err == nil {
			_, _, _ = readJSON(resp, nil)
		}
	}
	c.mu.Lock()
	c.clearLocked()
	c.mu.Unlock()
	return nil
}
