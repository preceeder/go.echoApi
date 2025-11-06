package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// 测试服务器地址（需要先启动服务器）
const (
	baseURL      = "http://localhost:8080"
	wsBaseURL    = "ws://localhost:8080"
	testDuration = 10 * time.Second // 压测持续时间
	concurrency  = 100               // 并发数
)

// checkServerHealth 检查服务器是否运行
func checkServerHealth() error {
	resp, err := http.Get(baseURL + "/api/time")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器状态码错误: %d", resp.StatusCode)
	}
	return nil
}

// TestMain 测试前置检查
func TestMain(m *testing.M) {
	// 检查服务器是否运行
	if err := checkServerHealth(); err != nil {
		fmt.Printf("警告: 无法连接到服务器 (%v)，请先启动服务器\n", err)
		fmt.Printf("服务器地址: %s\n", baseURL)
		fmt.Printf("运行: cd echoTest && go run main.go\n")
	}

	// 运行测试
	code := m.Run()
	os.Exit(code)
}

// ==================== GET /api/time 压测 ====================

// BenchmarkGetTime 基准测试
func BenchmarkGetTime(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}
	reqURL := baseURL + "/api/time"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(reqURL)
		if err != nil {
			b.Fatalf("请求失败: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("状态码错误: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestGetTimeLoadTest 并发压力测试
func TestGetTimeLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	reqURL := baseURL + "/api/time"

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, errorCount int64
	var totalLatency time.Duration

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(start) < testDuration {
				reqStart := time.Now()
				resp, err := client.Get(reqURL)
				latency := time.Since(reqStart)

				mu.Lock()
				if err != nil || resp.StatusCode != http.StatusOK {
					errorCount++
				} else {
					successCount++
					totalLatency += latency
				}
				mu.Unlock()

				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalRequests := successCount + errorCount
	avgLatency := time.Duration(0)
	if successCount > 0 {
		avgLatency = totalLatency / time.Duration(successCount)
	}
	qps := float64(totalRequests) / duration.Seconds()
	mu.Unlock()

	t.Logf("GET /api/time 压测结果:")
	t.Logf("  总请求数: %d", totalRequests)
	t.Logf("  成功数: %d", successCount)
	t.Logf("  失败数: %d", errorCount)
	t.Logf("  QPS: %.2f", qps)
	t.Logf("  平均延迟: %v", avgLatency)
	t.Logf("  持续时间: %v", duration)
}

// ==================== GET /api/token 压测 ====================

// BenchmarkGetToken 基准测试
func BenchmarkGetToken(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqURL := fmt.Sprintf("%s/api/token?name=test%d&age=%d", baseURL, i, 20+i%10)
		resp, err := client.Get(reqURL)
		if err != nil {
			b.Fatalf("请求失败: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("状态码错误: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestGetTokenLoadTest 并发压力测试
func TestGetTokenLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, errorCount int64
	var totalLatency time.Duration

	start := time.Now()
	requestID := int64(0)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(start) < testDuration {
				mu.Lock()
				id := requestID
				requestID++
				mu.Unlock()

				reqURL := fmt.Sprintf("%s/api/token?name=test%d&age=%d", baseURL, id, 20+int(id)%10)
				reqStart := time.Now()
				resp, err := client.Get(reqURL)
				latency := time.Since(reqStart)

				mu.Lock()
				if err != nil || resp.StatusCode != http.StatusOK {
					errorCount++
				} else {
					successCount++
					totalLatency += latency
				}
				mu.Unlock()

				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalRequests := successCount + errorCount
	avgLatency := time.Duration(0)
	if successCount > 0 {
		avgLatency = totalLatency / time.Duration(successCount)
	}
	qps := float64(totalRequests) / duration.Seconds()
	mu.Unlock()

	t.Logf("GET /api/token 压测结果:")
	t.Logf("  总请求数: %d", totalRequests)
	t.Logf("  成功数: %d", successCount)
	t.Logf("  失败数: %d", errorCount)
	t.Logf("  QPS: %.2f", qps)
	t.Logf("  平均延迟: %v", avgLatency)
	t.Logf("  持续时间: %v", duration)
}

// ==================== POST /api/create 压测 ====================

// BenchmarkPostCreate 基准测试
func BenchmarkPostCreate(b *testing.B) {
	client := &http.Client{Timeout: 10 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqURL := fmt.Sprintf("%s/api/create?category=tech%d&source=web", baseURL, i%5)
		body := map[string]interface{}{
			"name":        fmt.Sprintf("产品%d", i),
			"description": fmt.Sprintf("这是产品%d的描述", i),
			"price":       99.99 + float64(i%10),
			"tags":        []string{"tag1", "tag2", fmt.Sprintf("tag%d", i%3)},
		}
		bodyJSON, _ := json.Marshal(body)

		resp, err := client.Post(reqURL, "application/json", bytes.NewBuffer(bodyJSON))
		if err != nil {
			b.Fatalf("请求失败: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("状态码错误: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestPostCreateLoadTest 并发压力测试
func TestPostCreateLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, errorCount int64
	var totalLatency time.Duration

	start := time.Now()
	requestID := int64(0)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(start) < testDuration {
				mu.Lock()
				id := requestID
				requestID++
				mu.Unlock()

				reqURL := fmt.Sprintf("%s/api/create?category=tech%d&source=web", baseURL, id%5)
				body := map[string]interface{}{
					"name":        fmt.Sprintf("产品%d", id),
					"description": fmt.Sprintf("这是产品%d的描述", id),
					"price":       99.99 + float64(id%10),
					"tags":        []string{"tag1", "tag2", fmt.Sprintf("tag%d", id%3)},
				}
				bodyJSON, _ := json.Marshal(body)

				reqStart := time.Now()
				resp, err := client.Post(reqURL, "application/json", bytes.NewBuffer(bodyJSON))
				latency := time.Since(reqStart)

				mu.Lock()
				if err != nil || resp.StatusCode != http.StatusOK {
					errorCount++
				} else {
					successCount++
					totalLatency += latency
				}
				mu.Unlock()

				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalRequests := successCount + errorCount
	avgLatency := time.Duration(0)
	if successCount > 0 {
		avgLatency = totalLatency / time.Duration(successCount)
	}
	qps := float64(totalRequests) / duration.Seconds()
	mu.Unlock()

	t.Logf("POST /api/create 压测结果:")
	t.Logf("  总请求数: %d", totalRequests)
	t.Logf("  成功数: %d", successCount)
	t.Logf("  失败数: %d", errorCount)
	t.Logf("  QPS: %.2f", qps)
	t.Logf("  平均延迟: %v", avgLatency)
	t.Logf("  持续时间: %v", duration)
}

// ==================== WebSocket /api/ws 压测 ====================

// TestWebSocketLoadTest WebSocket 连接压力测试
func TestWebSocketLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, errorCount int64

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			// 建立 WebSocket 连接
			u, _ := url.Parse(wsBaseURL + "/api/ws")
			conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			defer conn.Close()

			mu.Lock()
			successCount++
			mu.Unlock()

			// 发送一些消息
			messageCount := 0
			for time.Since(start) < testDuration && messageCount < 100 {
				msg := map[string]interface{}{
					"type":    "message",
					"content": fmt.Sprintf("消息%d", messageCount),
					"connID":  connID,
				}
				if err := conn.WriteJSON(msg); err != nil {
					break
				}

				// 尝试读取响应（带超时）
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				var response map[string]interface{}
				if err := conn.ReadJSON(&response); err != nil {
					// 超时或错误，继续发送
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						break
					}
				}
				messageCount++
			}

			// 发送关闭消息
			closeMsg := map[string]interface{}{"type": "close"}
			conn.WriteJSON(closeMsg)
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalConnections := successCount + errorCount
	connectionRate := float64(totalConnections) / duration.Seconds()
	mu.Unlock()

	t.Logf("WebSocket /api/ws 压测结果:")
	t.Logf("  总连接数: %d", totalConnections)
	t.Logf("  成功连接数: %d", successCount)
	t.Logf("  失败连接数: %d", errorCount)
	t.Logf("  连接速率: %.2f conn/s", connectionRate)
	t.Logf("  持续时间: %v", duration)
}

// ==================== WebSocket /api/ws/echo 压测 ====================

// TestWebSocketEchoLoadTest WebSocket Echo 压力测试
func TestWebSocketEchoLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount, errorCount int64
	var totalMessages int64

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			// 建立 WebSocket 连接
			u, _ := url.Parse(wsBaseURL + "/api/ws/echo")
			conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			defer conn.Close()

			mu.Lock()
			successCount++
			mu.Unlock()

			// 发送消息并接收回显
			messageCount := 0
			for time.Since(start) < testDuration && messageCount < 200 {
				msg := fmt.Sprintf("echo消息%d_conn%d", messageCount, connID)
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					break
				}

				// 读取回显
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				_, _, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						break
					}
					continue
				}

				mu.Lock()
				totalMessages++
				mu.Unlock()
				messageCount++
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	mu.Lock()
	totalConnections := successCount + errorCount
	connectionRate := float64(totalConnections) / duration.Seconds()
	messageRate := float64(totalMessages) / duration.Seconds()
	mu.Unlock()

	t.Logf("WebSocket /api/ws/echo 压测结果:")
	t.Logf("  总连接数: %d", totalConnections)
	t.Logf("  成功连接数: %d", successCount)
	t.Logf("  失败连接数: %d", errorCount)
	t.Logf("  总消息数: %d", totalMessages)
	t.Logf("  连接速率: %.2f conn/s", connectionRate)
	t.Logf("  消息速率: %.2f msg/s", messageRate)
	t.Logf("  持续时间: %v", duration)
}

// ==================== 综合压测 ====================

// TestAllAPIsLoadTest 所有接口综合压测
func TestAllAPIsLoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试")
	}

	t.Log("开始综合压测...")
	t.Run("GET /api/time", TestGetTimeLoadTest)
	t.Run("GET /api/token", TestGetTokenLoadTest)
	t.Run("POST /api/create", TestPostCreateLoadTest)
	t.Run("WS /api/ws", TestWebSocketLoadTest)
	t.Run("WS /api/ws/echo", TestWebSocketEchoLoadTest)
	t.Log("综合压测完成")
}

