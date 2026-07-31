package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// Client WebSocket 客户端连接
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub WebSocket 连接中心
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

// NewHub 创建 WebSocket 中心
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

// HandleConnection 处理 WebSocket 升级请求
func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("[ws] 升级连接失败: %v\n", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 64),
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	fmt.Printf("[ws] 客户端已连接，当前连接数: %d\n", len(h.clients))

	// 启动写入协程
	go client.writePump(h)
	// 启动读取协程（保持连接活跃，处理 ping/pong）
	go client.readPump(h)
}

// BroadcastLog 广播日志消息
func (h *Hub) BroadcastLog(msg string) {
	h.broadcast([]byte(`{"type":"log","data":"` + escapeJSON(msg) + `"}`))
}

// BroadcastStatus 广播状态更新
func (h *Hub) BroadcastStatus(statusJSON string) {
	h.broadcast([]byte(`{"type":"status","data":` + statusJSON + `}`))
}

// BroadcastEvent 广播结构化事件（如日志/输出状态/场景切换）
func (h *Hub) BroadcastEvent(eventType string, data interface{}) {
	payload, err := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	if err != nil {
		return
	}
	h.broadcast(payload)
}

// broadcast 向所有客户端广播消息
func (h *Hub) broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			// 客户端发送缓冲区满，跳过
		}
	}
}

// removeClient 移除客户端
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
		fmt.Printf("[ws] 客户端已断开，当前连接数: %d\n", len(h.clients))
	}
}

// clientCount 获取客户端数量
func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// writePump 向客户端写入消息
func (c *Client) writePump(hub *Hub) {
	defer c.conn.Close()

	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			fmt.Printf("[ws] 写入消息失败: %v\n", err)
			break
		}
	}
}

// readPump 读取客户端消息（保持连接活跃）
func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				fmt.Printf("[ws] 读取异常: %v\n", err)
			}
			break
		}
		// 不处理客户端消息（仅服务端推送）
	}
}

// escapeJSON 转义 JSON 字符串中的特殊字符
func escapeJSON(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			if c < 0x20 {
				result += fmt.Sprintf("\\u%04x", c)
			} else {
				result += string(c)
			}
		}
	}
	return result
}
