package echoApi

import (
	"bytes"
	"github.com/labstack/echo/v4"
	"github.com/preceeder/go.base"
	"io"
	"net/url"
)

type HandlerFunc func(c *GContext)

//func Handle(h HandlerFunc) echo.HandlerFunc {
//	return func(c echo.Context) error {
//		ctx := &GContext{
//			Context:   c,
//			RequestId: c.Get("requestId"),
//			UContext: base.Context{
//				RequestId: c.Get("requestId"),
//			},
//		}
//		h(ctx)
//	}
//}

type GContext struct {
	echo.Context
	UContext base.BaseContext
}

type ParamsData struct {
	Body  any
	Query url.Values
	Url   string
	Path  map[string]string // Echo 的 path 参数是 map 结构
}

// GetRequestParamsEcho 提取 Echo 请求参数
func GetRequestParamsEcho(c echo.Context) ParamsData {
	var body []byte
	bo, err := io.ReadAll(c.Request().Body)
	if err != nil {
		body = []byte("")
	} else {
		body = bo
		c.Request().Body = io.NopCloser(bytes.NewBuffer(body))
	}

	// 解析 query 和 path 参数
	query := c.QueryParams()
	urlp := c.Request().RequestURI

	// path 参数：Echo 没有 gin.Params，要转成 map[string]string
	pathParams := make(map[string]string)
	for _, name := range c.ParamNames() {
		pathParams[name] = c.Param(name)
	}

	return ParamsData{
		Body:  string(body),
		Query: query,
		Url:   urlp,
		Path:  pathParams,
	}
}
