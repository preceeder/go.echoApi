package echoApi

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

type Auth struct {
	ApiRouteConfig ApiRouteConfig
}

func TestRegister(t *testing.T) {

	a := Auth{}

	a.ApiRouteConfig = ApiRouteConfig{
		ALL: &ApiRouterSubConfig{
			NoUseModel: true,
			CtxParams: map[string]string{
				"RequestUnSecret": "true",
			},
		},
		GET: []ApiRouterSubConfig{
			{
				Path:     "/time",
				FuncName: a.GetTime,
			},
			{
				Path:     "/token",
				FuncName: a.GetToken,
			},
		},
	}
	Register(&a)
	en := echo.New()
	MountRoutes(en)
	// 打印所有注册的路由
	for _, route := range en.Routes() {
		fmt.Printf("Method: %s, Path: %s, Name: %s\n", route.Method, route.Path, route.Name)
	}
	en.Start("0.0.0.0:2333")
	quit := make(chan os.Signal)
	//SIGINT 用户发送INTR字符(Ctrl+C)触发
	//SIGTERM 结束程序(可以被捕获、阻塞或忽略)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting Down project ...", "server-name", "")

	//Run(en, "echo", "0.0.0.0:2333", func() {
	//	fmt.Println("关机。。。。。。")
	//})
}

type Dte struct {
	Name string `query:"name"`
	Age  int    `query:"age" default:"18"`
}

func (a *Auth) GetToken(c echo.Context, Dtes Dte) error {
	fmt.Println("data:", Dtes)
	c.JSON(200, map[string]any{"data": "hello world"})
	return nil
}

func (a *Auth) GetTime(c echo.Context) error {
	c.JSON(200, map[string]any{"time": time.Now().Format("2006-01-02 15:04:05")})
	return nil
}
