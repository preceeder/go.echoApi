package echoApi

import (
	"bytes"
	"github.com/labstack/echo/v4"
	"io"
	"net/url"
)

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
