# API 压力测试

本目录包含所有接口的压力测试案例。

## 测试接口列表

1. **GET /api/time** - 获取当前时间接口
2. **GET /api/token** - 获取 Token 接口（带 query 参数）
3. **POST /api/create** - 创建数据接口（带 query 和 body 参数）
4. **WS /api/ws** - WebSocket 完整处理接口
5. **WS /api/ws/echo** - WebSocket 回显接口

## 运行测试

### 前置条件

1. 确保服务器已启动：
```bash
cd echoTest
go run main.go
```

服务器默认运行在 `http://localhost:8080`

### 运行基准测试

```bash
# 运行所有基准测试
go test -bench=. -benchmem ./test/api_test

# 运行特定接口的基准测试
go test -bench=BenchmarkGetTime ./test/api_test
go test -bench=BenchmarkGetToken ./test/api_test
go test -bench=BenchmarkPostCreate ./test/api_test
```

### 运行压力测试

```bash
# 运行所有压力测试（会跳过 short 模式的测试）
go test -v ./test/api_test

# 运行所有压力测试（包括 short 模式）
go test -v -short=false ./test/api_test

# 运行特定接口的压力测试
go test -v -run TestGetTimeLoadTest ./test/api_test
go test -v -run TestGetTokenLoadTest ./test/api_test
go test -v -run TestPostCreateLoadTest ./test/api_test
go test -v -run TestWebSocketLoadTest ./test/api_test
go test -v -run TestWebSocketEchoLoadTest ./test/api_test

# 运行综合压力测试
go test -v -run TestAllAPIsLoadTest ./test/api_test
```

### 压测参数配置

可以在 `api_benchmark_test.go` 中修改以下常量：

```go
const (
    baseURL      = "http://localhost:8080"  // 服务器地址
    wsBaseURL    = "ws://localhost:8080"    // WebSocket 地址
    testDuration = 10 * time.Second         // 压测持续时间
    concurrency  = 100                      // 并发数
)
```

## 测试输出说明

### 基准测试输出

```
BenchmarkGetTime-8    10000    120.5 ns/op    32 B/op    2 allocs/op
```

- `10000` - 执行次数
- `120.5 ns/op` - 每次操作平均耗时
- `32 B/op` - 每次操作分配的内存
- `2 allocs/op` - 每次操作的内存分配次数

### 压力测试输出

```
GET /api/time 压测结果:
  总请求数: 12500
  成功数: 12498
  失败数: 2
  QPS: 1250.00
  平均延迟: 80ms
  持续时间: 10s
```

- `总请求数` - 总共发送的请求数
- `成功数` - 成功的请求数
- `失败数` - 失败的请求数
- `QPS` - 每秒查询数（Queries Per Second）
- `平均延迟` - 平均响应时间
- `持续时间` - 测试持续时间

## 注意事项

1. **服务器资源**: 压力测试会消耗大量服务器资源，确保服务器有足够的 CPU 和内存
2. **并发数**: 默认并发数为 100，可以根据实际情况调整
3. **测试时长**: 默认测试时长为 10 秒，可以根据需要调整
4. **网络延迟**: 测试结果会受到网络延迟影响，建议在本地测试
5. **WebSocket 测试**: WebSocket 测试需要确保服务器支持 WebSocket 升级

## 常见问题

### 1. 连接被拒绝

确保服务器已启动：
```bash
cd echoTest && go run main.go
```

### 2. WebSocket 连接失败

确保服务器支持 WebSocket，并且路由已正确配置。

### 3. 测试超时

如果测试超时，可以：
- 减少并发数
- 缩短测试时长
- 检查服务器性能

## 性能优化建议

根据压测结果，可以考虑以下优化：

1. **数据库连接池**: 如果涉及数据库操作，优化连接池大小
2. **缓存**: 对热点数据进行缓存
3. **异步处理**: 对耗时操作进行异步处理
4. **负载均衡**: 使用负载均衡分散请求
5. **限流**: 实现限流保护服务器
