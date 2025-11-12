//go:build windows

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

func Run(r *echo.Echo, srvName string, addr string, stop func(), opts ...RunOption) {

	options := &runOptions{}
	if err := options.apply(opts); err != nil {
		slog.Error("apply run options failed", "serverName", srvName, "err", err.Error())
		return
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	if options.tlsConfig != nil {
		srv.TLSConfig = options.tlsConfig
	}

	//保证下面的优雅启停
	go func() {
		scheme := "http"
		if options.useTLS() {
			scheme = "https"
		}
		slog.Info("server running in ", "serverName", srvName, "addr", scheme+"://"+srv.Addr, "swag", scheme+"://"+srv.Addr+"/swagger/index.html")

		var err error
		if options.useTLS() {
			err = srv.ListenAndServeTLS(options.tlsCertFile, options.tlsKeyFile)
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			slog.Error(err.Error())
		} else {
			slog.Info("启动uri", "uri", addr)
		}
	}()
	quit := make(chan os.Signal, 1)
	//SIGINT 用户发送INTR字符(Ctrl+C)触发
	//SIGTERM 结束程序(可以被捕获、阻塞或忽略)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting Down project ...", "server-name", addr)
	
	// 先执行 stop 回调
	if stop != nil {
		stop()
	}
	
	// 优雅关闭服务器，最多等待30秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("stop error ", "svrName", srvName, "err", err.Error())
	} else {
		slog.Info("stop success ", "svrName", srvName)
	}

}

func RunLisenter(r *echo.Echo, srvName string, lisenter net.Listener, stop func()) {
	srv := &http.Server{Handler: r}
	//保证下面的优雅启停
	go func() {
		slog.Info("server running in ", "serverName", srvName, "swag", "http://127.0.0.1:port/swagger/index.html")
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
	slog.Info("Shutting Down project ...", "server-name", srvName)
	//ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	//defer cancel()
	if stop != nil {
		stop()
	}
	slog.Info("stop success ", "svrName", srvName)

	//slog.Info("<------------------hahaaahahahahahahha----------->")
	//if err := srv.Shutdown(ctx); err != nil {
	//	slog.Error("stop error ", "svrName", srvName, "err", err.Error())
	//}
	//
	//select {
	//case <-ctx.Done():
	//	slog.Info("stop success ", "svrName", srvName)
	//}
}
