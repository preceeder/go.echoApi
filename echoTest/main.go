package main

import (
	_ "base-utils/echoTest/api/auth" // 导入控制器包，触发 init 注册
	_ "base-utils/echoTest/api/chat" // 导入 WebSocket 控制器包，触发 init 注册
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/preceeder/go.base"
	"github.com/preceeder/go.echoApi"
	"github.com/preceeder/go.logs"
	"log/slog"
	"slices"
)

func main() {
	// Echo 服务器配置
	config := echoApi.EchoConfig{
		Name: "test",
		Addr: ":8080",
	}

	// CORS 配置（使用默认配置，开发环境）
	corsConfig := echoApi.DefaultCorsConfig()
	// 如果需要自定义 CORS 配置，可以这样：
	// corsConfig := echoApi.CorsConfig{
	// 	AllowOrigins:     []string{"https://example.com"},
	// 	AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
	// 	AllowHeaders:     []string{"Content-Type", "Authorization"},
	// 	ExposeHeaders:    []string{"X-Username"},
	// 	AllowCredentials: true,
	// 	MaxAge:           86400,
	// }

	// 创建 Echo 实例并应用中间件
	r := echoApi.NewEcho(
		config,
		corsConfig,
		// 响应拦截中间件（用于日志记录、加密等）
		echoApi.InterceptMiddleware(func(c echo.Context, resp *echoApi.ResponseInterceptor) []byte {
			req := c.Request()
			requestId, _ := c.Get("requestId").(string)
			userId, _ := c.Get("userId").(string)

			// 根据配置决定是否打印响应日志
			if c.Get("PrintResponse") == "true" {
				slog.InfoContext(
					base.Context{RequestId: requestId, UserId: userId},
					"Method", req.Method,
					"url", req.URL.String(),
					"响应数据", "body", logs.LogStr(resp.Body.String()),
				)
			} else if slices.Contains([]string{"POST", "PUT"}, req.Method) {
				// POST 和 PUT 请求默认打印响应日志
				slog.InfoContext(
					base.Context{RequestId: requestId, UserId: userId},
					"Method", req.Method,
					"url", req.URL.String(),
					"响应数据", "body", logs.LogStr(resp.Body.String()),
				)
			}

			// 响应加密处理
			if c.Get("ResponseSecret") == "false" {
				// 不加密，直接返回原始响应
				return resp.Body.Bytes()
			} else {
				// 加密响应（示例代码）
				header := c.Response().Header()
				header.Set(echo.HeaderContentType, echo.MIMETextPlain)
				header.Set("x-auth-timestamp", "sdsds")
				header.Set("x-auth-announce", "sdwwew")
				res := "加密后的参数Data"
				// 实际使用中，可以这样加密：
				// res, _ := aes.EncryptCBCBase64([]byte(resp.Body.String()), []byte(aesKey), []byte(aesIv))
				return []byte(res)
			}
		}),
		// 响应和错误处理中间件
		echoApi.EchoResponseAndRecoveryHandler(
			// 正常响应处理器
			func(c echo.Context, res echoApi.HttpResponse) echoApi.HttpResponse {
				return res
			},
			// 错误响应处理器
			func(c echo.Context, res echoApi.HttpError) echoApi.HttpError {
				return res
			},
		),
	)

	// 启动服务器
	echoApi.Run(r, config.Name, config.Addr, func() {
		fmt.Println("服务器已停止")
	})
}
