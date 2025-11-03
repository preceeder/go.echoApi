package echoApi

import (
	"github.com/labstack/echo/v4"
)

type EchoConfig struct {
	Name                       string `json:"name"`
	Addr                       string `json:"addr"`
	HideServerMiddleLog        bool   `json:"hideServerMiddleLog"`        // 是否隐藏内置中间件的 http 日志
	HideServerMiddleLogHeaders bool   `json:"hideServerMiddleLogHeaders"` // 是否隐藏内置中间件 http 日志 中的 headers   这个配置生效的前提是  hideServerMiddleLog=false
}

func NewEcho(config EchoConfig, corsConfig CorsConfig, middlewares ...echo.MiddlewareFunc) *echo.Echo {
	r := echo.New()
	var baseMiddleWares = []echo.MiddlewareFunc{
		BaseErrorMiddleware(),
		CorsMiddleware(corsConfig),
		EchoLogger(config.HideServerMiddleLog, config.HideServerMiddleLogHeaders),
	}
	if len(middlewares) > 0 {
		baseMiddleWares = append(baseMiddleWares, middlewares...)
	}
	r.Use(baseMiddleWares...)
	MountRoutes(r)
	return r
}
