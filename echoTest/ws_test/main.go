package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketClient WebSocket 客户端示例
type WebSocketClient struct {
	conn  *websocket.Conn
	url   string
	done  chan struct{}
	errCh chan error
	msgCh chan []byte
}

// NewWebSocketClient 创建新的 WebSocket 客户端
func NewWebSocketClient(url string) *WebSocketClient {
	return &WebSocketClient{
		url:   url,
		done:  make(chan struct{}),
		errCh: make(chan error, 1),
		msgCh: make(chan []byte, 256),
	}
}

// Connect 连接到 WebSocket 服务器
func (c *WebSocketClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	c.conn = conn
	log.Printf("✅ 成功连接到: %s", c.url)

	// 启动读写 goroutine
	go c.readPump()
	go c.writePump()

	return nil
}

// readPump 读取消息的 goroutine
func (c *WebSocketClient) readPump() {
	defer func() {
		c.conn.Close()
		close(c.msgCh)
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		log.Println("📨 收到 Pong")
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.errCh <- fmt.Errorf("读取错误: %w", err)
			}
			break
		}

		switch messageType {
		case websocket.TextMessage:
			log.Printf("📥 收到文本消息: %s", string(message))

			// 尝试解析为 JSON
			var msgData map[string]interface{}
			if err := json.Unmarshal(message, &msgData); err == nil {
				// 格式化输出 JSON
				prettyJSON, _ := json.MarshalIndent(msgData, "", "  ")
				log.Printf("📦 JSON 消息:\n%s", string(prettyJSON))
			}

		case websocket.BinaryMessage:
			log.Printf("📥 收到二进制消息: %d bytes", len(message))

		case websocket.PingMessage:
			log.Println("📨 收到 Ping")
			c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		case websocket.PongMessage:
			log.Println("📨 收到 Pong")
			c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		case websocket.CloseMessage:
			log.Println("🔌 收到关闭消息")
			return
		}

		c.msgCh <- message
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

// writePump 发送心跳的 goroutine
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.errCh <- fmt.Errorf("发送 Ping 失败: %w", err)
				return
			}
			log.Println("📤 发送 Ping")

		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return

		case err := <-c.errCh:
			log.Printf("❌ 错误: %v", err)
			return
		}
	}
}

// SendMessage 发送文本消息
func (c *WebSocketClient) SendMessage(message string) error {
	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteMessage(websocket.TextMessage, []byte(message))
}

// SendJSON 发送 JSON 消息
func (c *WebSocketClient) SendJSON(data interface{}) error {
	message, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}

	c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err = c.conn.WriteMessage(websocket.TextMessage, message)
	if err == nil {
		log.Printf("📤 发送 JSON 消息: %s", string(message))
	}
	return err
}

// Close 关闭连接
func (c *WebSocketClient) Close() {
	close(c.done)
	c.conn.Close()
	log.Println("🔌 连接已关闭")
}

// RunWebSocketExample 运行示例
func RunWebSocketExample() {
	// 连接到 WebSocket 服务器
	client := NewWebSocketClient("ws://localhost:8080/api/ws")

	if err := client.Connect(); err != nil {
		log.Fatalf("连接失败: %v", err)
		return
	}

	// 等待连接建立
	time.Sleep(1 * time.Second)

	// 发送不同类型的消息
	log.Println("\n=== 发送测试消息 ===")

	// 1. 发送普通文本消息
	time.Sleep(1 * time.Second)
	log.Println("📤 发送: Hello WebSocket")
	if err := client.SendMessage("Hello WebSocket"); err != nil {
		log.Printf("发送消息失败: %v", err)
	}

	// 2. 发送 JSON 消息（ping）
	time.Sleep(2 * time.Second)
	log.Println("📤 发送: ping 消息")
	if err := client.SendJSON(map[string]interface{}{
		"type": "ping",
	}); err != nil {
		log.Printf("发送 ping 失败: %v", err)
	}

	// 3. 发送 JSON 消息（message）
	time.Sleep(2 * time.Second)
	log.Println("📤 发送: message 消息")
	if err := client.SendJSON(map[string]interface{}{
		"type":    "message",
		"content": "这是一条测试消息",
	}); err != nil {
		log.Printf("发送 message 失败: %v", err)
	}

	// 4. 发送多条消息
	time.Sleep(2 * time.Second)
	for i := 1; i <= 3; i++ {
		if err := client.SendJSON(map[string]interface{}{
			"type":    "message",
			"content": fmt.Sprintf("消息 #%d", i),
		}); err != nil {
			log.Printf("发送消息 #%d 失败: %v", i, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 等待一段时间接收消息
	log.Println("\n=== 等待接收消息 ===")
	time.Sleep(5 * time.Second)

	// 发送关闭消息
	log.Println("📤 发送: close 消息")
	if err := client.SendJSON(map[string]interface{}{
		"type": "close",
	}); err != nil {
		log.Printf("发送 close 失败: %v", err)
	}

	// 等待接收关闭响应
	time.Sleep(2 * time.Second)

	// 关闭连接
	client.Close()
}

// RunEchoExample 运行回显示例
func RunEchoExample() {
	client := NewWebSocketClient("ws://localhost:8080/api/ws/echo")

	if err := client.Connect(); err != nil {
		log.Fatalf("连接失败: %v", err)
		return
	}

	// 等待连接建立
	time.Sleep(1 * time.Second)

	log.Println("\n=== Echo 测试 ===")

	// 发送多条消息，服务器会回显
	messages := []string{
		"消息 1",
		"消息 2",
		"消息 3",
		"测试回显功能",
	}

	for i, msg := range messages {
		time.Sleep(1 * time.Second)
		log.Printf("📤 发送 [%d]: %s", i+1, msg)
		if err := client.SendMessage(msg); err != nil {
			log.Printf("发送失败: %v", err)
			break
		}
	}

	// 等待接收回显
	time.Sleep(3 * time.Second)
	client.Close()
}

// 独立运行的客户端主函数
func main() {
	log.Println("=== WebSocket 客户端测试 ===")

	// 测试完整的 WebSocket 处理
	log.Println("\n[测试 1] 完整 WebSocket 处理")
	RunWebSocketExample()

	time.Sleep(3 * time.Second)

	// 测试回显功能
	log.Println("\n[测试 2] Echo 回显功能")
	RunEchoExample()

	log.Println("\n=== 测试完成 ===")
}
