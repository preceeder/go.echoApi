package main

import (
	_ "base-utils/echoTest/api/auth"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/preceeder/go.base"
	"github.com/preceeder/go.echoApi"
	"github.com/preceeder/go.logs"
	"github.com/preceeder/go/echoApi"
	"log/slog"
	"slices"
)

func main() {
	config := echoApi.EchoConfig{
		Name: "test",
		Addr: ":8080",
	}
	r := echoApi.NewEcho(config,
		echoApi.InterceptMiddleware(func(c echo.Context, resp *echoApi.ResponseInterceptor) []byte {
			req := c.Request()
			if c.Get("PrintResponse") == "true" {
				// 是否打印响应日志
				slog.InfoContext(base.Context{RequestId: c.Get("requestId"), UserId: c.Get("userId")}, "Method", req.Method, "url", req.URL.String(), "响应数据", "body", logs.LogStr(resp.Body.String()))
			} else if slices.Contains([]string{"POST", "PUT"}, req.Method) {
				slog.InfoContext(base.Context{RequestId: c.Get("requestId"), UserId: c.Get("userId")}, "Method", req.Method, "url", req.URL.String(), "响应数据", "body", logs.LogStr(resp.Body.String()))
			}
			if c.Get("ResponseSecret") == "false" {
				return resp.Body.Bytes()
			} else {
				header := c.Response().Header()
				header.Set(echo.HeaderContentType, echo.MIMETextPlain)
				header.Set("x-auth-timestamp", "sdsds")
				header.Set("x-auth-announce", "sdwwew")
				res := "加密后的参数Data"
				//res, _ := aes.EncryptCBCBase64([]byte(resp.Body.String()), []byte(aesKey), []byte(aesIv))
				return []byte(res)
			}
			return resp.Body.Bytes()
		}),
		echoApi.EchoResponseAndRecoveryHandler(func(c echo.Context, res echoApi.HttpResponse) echoApi.HttpResponse { return res },
			func(c echo.Context, res echoApi.HttpError) echoApi.HttpError { return res }),
	)
	echoApi.Run(r, config.Name, config.Addr, func() {
		fmt.Println("停止")
	})
}
