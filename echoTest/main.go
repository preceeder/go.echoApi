package main

import (
	_ "base-utils/echoTest/api/auth" // 导入控制器包，触发 init 注册
	_ "base-utils/echoTest/api/chat" // 导入 WebSocket 控制器包，触发 init 注册
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/preceeder/echoApi"
	"github.com/preceeder/logs"
	"log/slog"
	"slices"
)

func main() {
	// Echo 服务器配置
	config := echoApi.EchoConfig{
		Name: "test",
		Addr: ":8080",
	}

	// 创建 Echo 实例并应用中间件
	r := echoApi.NewEcho(
		echoApi.BaseErrorMiddleware(),
		echoApi.CorsMiddleware(echoApi.DefaultCorsConfig()),
		echoApi.EchoLogger(config.HideServerMiddleLog, config.HideServerMiddleLogHeaders),
		// 响应拦截中间件（用于日志记录、加密等）
		echoApi.InterceptMiddleware(func(c echo.Context, resp *echoApi.ResponseInterceptor) []byte {
			req := c.Request()

			// 根据配置决定是否打印响应日志
			if c.Get("PrintResponse") == "true" {
				slog.InfoContext(
					c.Get("context").(context.Context),
					"Method", req.Method,
					"url", req.URL.String(),
					"响应数据", "body", resp.Body.String(),
				)
			} else if slices.Contains([]string{"POST", "PUT"}, req.Method) {
				// POST 和 PUT 请求默认打印响应日志
				slog.InfoContext(
					c.Get("context").(context.Context),
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

	var runOptions []echoApi.RunOption
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		runOptions = append(runOptions, echoApi.WithTLSCertificates(config.TLSCertFile, config.TLSKeyFile))
	}

	// 启动服务器
	echoApi.Run(r, config.Name, config.Addr, func() {
		fmt.Println("服务器已停止")
	}, runOptions...)
}
