package chat

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"log/slog"
	"time"
)

// HandleWebSocket 基础 WebSocket 处理示例
// 路由: WS /api/ws
// 功能：接收客户端消息并返回响应
func (c *Chat) HandleWebSocket(gc echo.Context, conn *websocket.Conn) error {
	requestId := gc.Get("requestId")
	slog.Info("WebSocket 连接已建立", "requestId", requestId, "remoteAddr", conn.RemoteAddr().String())

	// 发送欢迎消息
	welcomeMsg := map[string]interface{}{
		"type":    "welcome",
		"message": "WebSocket 连接成功",
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		slog.Error("发送欢迎消息失败", "error", err, "requestId", requestId)
		return err
	}
	slog.Info("已发送欢迎消息", "requestId", requestId)

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 启动心跳
	go func() {
		ticker := time.NewTicker(54 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// 消息循环
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket 读取错误", "error", err, "requestId", requestId)
			}
			break
		}

		slog.Info("收到 WebSocket 消息", "requestId", requestId, "type", messageType, "message", string(message))

		// 解析 JSON 消息（如果是 JSON）
		var msgData map[string]interface{}
		if err := json.Unmarshal(message, &msgData); err != nil {
			// 如果不是 JSON，直接回显
			response := map[string]interface{}{
				"type":    "echo",
				"message": string(message),
				"time":    time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := conn.WriteJSON(response); err != nil {
				slog.Error("发送响应失败", "error", err)
				break
			}
			continue
		}

		// 处理不同类型的消息
		switch msgType := msgData["type"].(string); msgType {
		case "ping":
			// 响应 ping
			response := map[string]interface{}{
				"type":      "pong",
				"time":      time.Now().Format("2006-01-02 15:04:05"),
				"requestId": requestId,
			}
			if err := conn.WriteJSON(response); err != nil {
				slog.Error("发送 pong 失败", "error", err)
				break
			}

		case "message":
			// 处理普通消息
			content := msgData["content"]
			response := map[string]interface{}{
				"type":     "response",
				"message":  fmt.Sprintf("收到消息: %v", content),
				"time":     time.Now().Format("2006-01-02 15:04:05"),
				"original": msgData,
			}
			if err := conn.WriteJSON(response); err != nil {
				slog.Error("发送响应失败", "error", err)
				break
			}

		case "close":
			// 客户端请求关闭连接
			response := map[string]interface{}{
				"type":    "close",
				"message": "连接即将关闭",
			}
			conn.WriteJSON(response)
			return nil

		default:
			// 未知消息类型，返回错误
			response := map[string]interface{}{
				"type":    "error",
				"message": "未知的消息类型: " + msgType,
			}
			if err := conn.WriteJSON(response); err != nil {
				slog.Error("发送错误响应失败", "error", err)
				break
			}
		}

		// 重置读取超时
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}

	slog.Info("WebSocket 连接已关闭", "requestId", requestId)
	return nil
}

// HandleEcho 回显 WebSocket 处理示例
// 路由: WS /api/ws/echo
// 功能：简单回显所有收到的消息
func (c *Chat) HandleEcho(gc echo.Context, conn *websocket.Conn) error {
	requestId := gc.Get("requestId")
	slog.Info("Echo WebSocket 连接已建立", "requestId", requestId)

	defer func() {
		slog.Info("Echo WebSocket 连接已关闭", "requestId", requestId)
	}()

	// 消息循环：回显所有收到的消息
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Echo WebSocket 读取错误", "error", err)
			}
			break
		}

		// 回显消息
		if err := conn.WriteMessage(messageType, message); err != nil {
			slog.Error("Echo WebSocket 回显失败", "error", err)
			break
		}
	}

	return nil
}
