package auth

import (
	"fmt"
	"github.com/preceeder/go/echoApi"
	"time"
)

type Dte struct {
	Name string `query:"name"`
	Age  int    `query:"age" default:"18"`
}

func (a *Auth) GetToken(c echoApi.GContext, Dtes Dte) echoApi.HttpResponse {
	fmt.Println("data:", Dtes)
	c.JSON(200, map[string]any{"data": "hello world"})
	return nil
}

func (a *Auth) GetTime(c echoApi.GContext) echoApi.HttpResponse {
	return echoApi.BaseHttpResponse{
		Data: map[string]any{"time": time.Now().Format("2006-01-02 15:04:05")},
	}
}
