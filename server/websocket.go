package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// WebSocket 消息类型
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub 管理所有 WebSocket 连接
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan WSMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Client 表示一个 WebSocket 连接
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan WSMessage
}

// 升级器：将 HTTP 连接升级为 WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境应限制）
	},
}

// NewHub 创建新的 Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan WSMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 启动 Hub 的事件循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("📡 WebSocket 客户端连接，当前连接数: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("📡 WebSocket 客户端断开，当前连接数: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 发送失败，关闭连接
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(msgType string, data interface{}) {
	msg := WSMessage{Type: msgType, Data: data}
	select {
	case h.broadcast <- msg:
	default:
		// 非阻塞发送
	}
}

// ClientCount 返回当前连接的客户端数量
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// handleWebSocket 处理 WebSocket 连接请求
func (s *Server) handleWebSocket(c echo.Context) error {
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("❌ WebSocket 升级失败: %v", err)
		return err
	}

	client := &Client{
		hub:  s.Hub,
		conn: conn,
		send: make(chan WSMessage, 256),
	}

	s.Hub.register <- client

	// 启动读写协程
	go client.writePump()
	go client.readPump()

	return nil
}

// readPump 从 WebSocket 读取消息（主要用于检测断开）
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("⚠️  WebSocket 读取错误: %v", err)
			}
			break
		}
		// 忽略客户端发来的消息（纯推送模式）
	}
}

// writePump 向 WebSocket 写入消息
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		if err := c.conn.WriteJSON(message); err != nil {
			log.Printf("⚠️  WebSocket 写入错误: %v", err)
			return
		}
	}
}
