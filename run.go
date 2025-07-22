//go:build darwin

package echoApi

import (
	"context"
	"github.com/labstack/echo/v4"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(r *echo.Echo, srvName string, addr string, stop func()) {

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	//保证下面的优雅启停
	go func() {
		slog.Info("server running in ", "serverName", srvName, "addr", "http://"+srv.Addr, "swag", "http://"+srv.Addr+"/swagger/index.html")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(err.Error())
			os.Exit(2)
		} else {
			slog.Info("启动uri", "uri", addr)
		}
	}()
	quit := make(chan os.Signal)
	//SIGINT 用户发送INTR字符(Ctrl+C)触发
	//SIGTERM 结束程序(可以被捕获、阻塞或忽略)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting Down project ...", "server-name", addr)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	if stop != nil {
		stop()
	}
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("stop error ", "svrName", srvName, "err", err.Error())
	}

	select {
	case <-ctx.Done():
		slog.Info("stop success ", "svrName", srvName)
	}

}

// 结合	"github.com/preceeder/graceful/fetcher" 使用
func RunLisenter(r *echo.Echo, srvName string, lisenter net.Listener, stop func()) {
	srv := &http.Server{Handler: r}
	//保证下面的优雅启停
	go func() {
		slog.Info("server running in ", "serverName", srvName, "addr", "http://"+lisenter.Addr().String(), "swag", "http://"+lisenter.Addr().String()+"/swagger/index.html")
		if err := srv.Serve(lisenter); err != nil && err != http.ErrServerClosed {
			slog.Error("tcp链接关闭", "pid", os.Getpid(), "message", err.Error())
		} else {
			slog.Info("启动uri", "uri", lisenter)
		}
	}()
	quit := make(chan os.Signal)
	//SIGINT 用户发送INTR字符(Ctrl+C)触发
	//SIGTERM 结束程序(可以被捕获、阻塞或忽略)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting Down project ...", "server-name", lisenter.Addr().String())
	if stop != nil {
		stop()
	}
	slog.Info("stop success ", "svrName", srvName)
}
