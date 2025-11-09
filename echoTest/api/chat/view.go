package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"log/slog"
	"time"
)

// HandleWebSocket 基础 WebSocket 处理示例
// 路由: WS /api/ws
// 功能：接收客户端消息并返回响应
func (c *Chat) HandleWebSocket(gc echo.Context, conn *websocket.Conn) error {
	ctx := gc.Get("context").(context.Context)
	requestId := ctx.Value("requestId")
	slog.Info("WebSocket 连接已建立", "requestId", requestId, "remoteAddr", ctx.Value("remote_ip"))

	baseCtx := gc.Request().Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	// 发送欢迎消息
	welcomeMsg := map[string]interface{}{
		"type":    "welcome",
		"message": "WebSocket 连接成功",
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}

	sendCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
	if err := writeJSON(sendCtx, conn, welcomeMsg); err != nil {
		cancel()
		slog.Error("发送欢迎消息失败", "error", err, "requestId", requestId)
		return err
	}
	cancel()
	slog.Info("已发送欢迎消息", "requestId", requestId)

	heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
	defer heartbeatCancel()

	// 启动心跳
	go func() {
		ticker := time.NewTicker(54 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := conn.Ping(pingCtx); err != nil {
					slog.Error("发送 Ping 失败", "error", err, "requestId", requestId)
					cancel()
					return
				}
				cancel()

			case <-heartbeatCtx.Done():
				return
			}
		}
	}()

	// 消息循环
	for {
		readCtx, cancel := context.WithTimeout(baseCtx, 60*time.Second)
		messageType, message, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			status := websocket.CloseStatus(err)
			switch status {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway:
				slog.Info("WebSocket 连接正常关闭", "requestId", requestId, "status", status)
			case websocket.StatusAbnormalClosure:
				slog.Warn("WebSocket 连接异常关闭", "requestId", requestId, "error", err)
			default:
				if status == -1 {
					slog.Error("WebSocket 读取错误", "error", err, "requestId", requestId)
				} else {
					slog.Warn("WebSocket 连接关闭", "requestId", requestId, "status", status, "error", err)
				}
			}
			break
		}

		switch messageType {
		case websocket.MessageText:
			slog.Info("收到 WebSocket 文本消息", "requestId", requestId, "message", string(message))

			// 解析 JSON 消息（如果是 JSON）
			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err != nil {
				// 如果不是 JSON，直接回显
				response := map[string]interface{}{
					"type":    "echo",
					"message": string(message),
					"time":    time.Now().Format("2006-01-02 15:04:05"),
				}
				writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
				if err := writeJSON(writeCtx, conn, response); err != nil {
					cancel()
					slog.Error("发送响应失败", "error", err, "requestId", requestId)
					return err
				}
				cancel()
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
				writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
				if err := writeJSON(writeCtx, conn, response); err != nil {
					cancel()
					slog.Error("发送 pong 失败", "error", err, "requestId", requestId)
					return err
				}
				cancel()

			case "message":
				// 处理普通消息
				content := msgData["content"]
				response := map[string]interface{}{
					"type":     "response",
					"message":  fmt.Sprintf("收到消息: %v", content),
					"time":     time.Now().Format("2006-01-02 15:04:05"),
					"original": msgData,
				}
				writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
				if err := writeJSON(writeCtx, conn, response); err != nil {
					cancel()
					slog.Error("发送响应失败", "error", err, "requestId", requestId)
					return err
				}
				cancel()

			case "close":
				// 客户端请求关闭连接
				response := map[string]interface{}{
					"type":    "close",
					"message": "连接即将关闭",
				}
				writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
				_ = writeJSON(writeCtx, conn, response)
				cancel()
				conn.Close(websocket.StatusNormalClosure, "client requested close")
				return nil

			default:
				// 未知消息类型，返回错误
				response := map[string]interface{}{
					"type":    "error",
					"message": "未知的消息类型: " + msgType,
				}
				writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
				if err := writeJSON(writeCtx, conn, response); err != nil {
					cancel()
					slog.Error("发送错误响应失败", "error", err, "requestId", requestId)
					return err
				}
				cancel()
			}

		case websocket.MessageBinary:
			slog.Info("收到 WebSocket 二进制消息", "requestId", requestId, "size", len(message))
			writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
			if err := conn.Write(writeCtx, websocket.MessageBinary, message); err != nil {
				cancel()
				slog.Error("回显二进制消息失败", "error", err, "requestId", requestId)
				return err
			}
			cancel()

		default:
			slog.Debug("收到未知类型的 WebSocket 消息", "requestId", requestId, "type", messageType)
			continue
		}
	}

	heartbeatCancel()
	conn.Close(websocket.StatusNormalClosure, "handler completed")
	slog.Info("WebSocket 连接已关闭", "requestId", requestId)
	return nil
}

// HandleEcho 回显 WebSocket 处理示例
// 路由: WS /api/ws/echo
// 功能：简单回显所有收到的消息
func (c *Chat) HandleEcho(gc echo.Context, conn *websocket.Conn) error {
	requestId := gc.Get("requestId")
	slog.Info("Echo WebSocket 连接已建立", "requestId", requestId)

	baseCtx := gc.Request().Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	defer func() {
		slog.Info("Echo WebSocket 连接已关闭", "requestId", requestId)
	}()

	// 消息循环：回显所有收到的消息
	for {
		readCtx, cancel := context.WithTimeout(baseCtx, 60*time.Second)
		messageType, message, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == websocket.StatusGoingAway || status == websocket.StatusNormalClosure {
				slog.Info("Echo WebSocket 正常关闭", "requestId", requestId, "status", status)
			} else if status == websocket.StatusAbnormalClosure {
				slog.Warn("Echo WebSocket 非正常关闭", "requestId", requestId, "error", err)
			} else if status == -1 {
				slog.Error("Echo WebSocket 读取错误", "error", err, "requestId", requestId)
			} else {
				slog.Warn("Echo WebSocket 关闭", "requestId", requestId, "status", status, "error", err)
			}
			break
		}

		switch messageType {
		case websocket.MessageText, websocket.MessageBinary:
			writeCtx, cancel := context.WithTimeout(baseCtx, 5*time.Second)
			if err := conn.Write(writeCtx, messageType, message); err != nil {
				cancel()
				slog.Error("Echo WebSocket 回显失败", "error", err, "requestId", requestId)
				return err
			}
			cancel()
		default:
			slog.Debug("Echo WebSocket 收到未知类型消息", "requestId", requestId, "type", messageType)
		}
	}

	conn.Close(websocket.StatusNormalClosure, "echo handler completed")
	return nil
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
