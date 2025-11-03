# WebSocket API 使用示例

## 功能说明

本示例展示了如何使用框架的 WebSocket 功能，包含两个 WebSocket 端点：

1. `/api/ws` - 完整的 WebSocket 处理示例，支持心跳、消息处理等
2. `/api/ws/echo` - 简单的回显示例

## WebSocket Handler 签名

WebSocket 处理函数必须遵循以下签名：

```go
func (c *Chat) HandleWebSocket(gc echoApi.GContext, conn *websocket.Conn) error {
    // gc: 框架的 GContext，包含请求上下文信息
    // conn: WebSocket 连接对象
    // 返回 error 如果连接处理出错
}
```

## 路由配置

在 `RouteConfig` 中使用 `WS` 字段配置 WebSocket 路由：

```go
func (c *Chat) RouteConfig() echoApi.RouteConfig {
    return echoApi.RouteConfig{
        WS: []echoApi.RouteBuilder{
            {
                Path:     "/ws",
                FuncName: c.HandleWebSocket,
            },
        },
    }
}
```

## 客户端连接示例

### JavaScript (浏览器)

```javascript
// 连接到 /api/ws
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onopen = function() {
    console.log('WebSocket 连接已建立');
    
    // 发送欢迎消息
    ws.send(JSON.stringify({
        type: 'message',
        content: 'Hello from client'
    }));
};

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log('收到消息:', data);
};

ws.onerror = function(error) {
    console.error('WebSocket 错误:', error);
};

ws.onclose = function() {
    console.log('WebSocket 连接已关闭');
};
```

### cURL (测试 WebSocket 升级)

```bash
# 注意：cURL 需要特殊参数才能测试 WebSocket
curl --include \
     --no-buffer \
     --header "Connection: Upgrade" \
     --header "Upgrade: websocket" \
     --header "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
     --header "Sec-WebSocket-Version: 13" \
     http://localhost:8080/api/ws
```

### wscat (推荐用于测试)

```bash
# 安装 wscat
npm install -g wscat

# 连接到 WebSocket
wscat -c ws://localhost:8080/api/ws

# 发送消息
{"type":"message","content":"Hello"}
{"type":"ping"}
{"type":"close"}
```

## 消息格式示例

### 发送消息到服务器

```json
{
    "type": "message",
    "content": "这是消息内容"
}
```

### 服务器响应格式

```json
{
    "type": "response",
    "message": "收到消息: 这是消息内容",
    "time": "2024-01-01 12:00:00",
    "original": {
        "type": "message",
        "content": "这是消息内容"
    }
}
```

## 特性说明

1. **自动心跳**: 服务器每 54 秒发送一次 ping，客户端应响应 pong
2. **超时处理**: 60 秒无活动自动断开连接
3. **JSON 消息**: 支持 JSON 格式的结构化消息
4. **错误处理**: 自动处理连接关闭和异常情况

## 注意事项

1. WebSocket 路由使用 GET 方法注册，但会在 handler 中检测升级
2. 生产环境应该配置 `CheckOrigin` 函数来验证请求来源
3. WebSocket 连接会绕过 HTTP 响应拦截中间件（因为升级后不再是 HTTP）
4. 可以正常使用其他中间件（认证、日志等）

