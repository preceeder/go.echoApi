package auth

import (
	"fmt"
	"github.com/preceeder/go.echoApi"
	"net/http"
	"time"
)

// Dte 请求参数结构体示例
// 支持从 query 参数绑定，并支持默认值
type Dte struct {
	Name string `query:"name" json:"name"`            // query 参数绑定
	Age  int    `query:"age" json:"age" default:"18"` // 带默认值的参数
}

// GetToken 获取 Token 的接口
// 路由: GET /api/token
// 参数会自动从 query string 绑定到 Dte 结构体，Age 字段如果未提供则默认为 18
func (a *Auth) GetToken(c echoApi.GContext, req *Dte) echoApi.HttpResponse {
	fmt.Println("接收到的参数:", req)

	// 方式1: 直接使用 c.JSON 返回（但返回 nil 让中间件处理统一响应格式）
	c.JSON(http.StatusOK, map[string]any{"data": "hello world", "params": req})
	return nil

	// 方式2: 返回 HttpResponse 接口（推荐，统一响应格式）
	// return echoApi.BaseHttpResponse{
	// 	Data: map[string]any{"data": "hello world", "params": req},
	// }
}

// GetTime 获取当前时间的接口
// 路由: GET /api/time
// 不需要额外参数，只需要 GContext
func (a *Auth) GetTime(c echoApi.GContext) echoApi.HttpResponse {
	return echoApi.BaseHttpResponse{
		Data: map[string]any{
			"time":      time.Now().Format("2006-01-02 15:04:05"),
			"timestamp": time.Now().Unix(),
		},
	}
}

// GetUser 示例：带路径参数的接口
// 如果需要路径参数，可以在 RouteBuilder 中配置 Path: "/user/:id"
// func (a *Auth) GetUser(c echoApi.GContext, id string) echoApi.HttpResponse {
// 	return echoApi.BaseHttpResponse{
// 		Data: map[string]any{"id": id},
// 	}
// }

// CreateUser 示例：POST 请求，带请求体绑定
// type CreateUserReq struct {
// 	Username string `json:"username" binding:"required"`
// 	Email    string `json:"email" binding:"required,email"`
// 	Age      int    `json:"age" default:"0"`
// }
//
// func (a *Auth) CreateUser(c echoApi.GContext, req CreateUserReq) echoApi.HttpResponse {
// 	// 处理业务逻辑
// 	return echoApi.BaseHttpResponse{
// 		Data: map[string]any{"user": req},
// 	}
// }

// CreateData POST 接口示例：整合 query 和 body 参数
// 路由: POST /api/create
// 支持从 query string 和 request body 同时接收参数
type CreateDataQuery struct {
	Name     string `query:"name" json:"name"`
	Category string `query:"category" json:"category"`           // query 参数
	Source   string `query:"source" json:"source" default:"web"` // query 参数，带默认值
}

type CreateDataBody struct {
	Name        string   `json:"name"`                // body 参数
	Description string   `json:"description"`         // body 参数
	Price       float64  `json:"price" default:"0.0"` // body 参数，带默认值
	Tags        []string `json:"tags"`                // body 参数，数组类型
}

func (a *Auth) CreateData(c echoApi.GContext, query *CreateDataQuery, body *CreateDataBody) echoApi.HttpResponse {
	// 整合 query 和 body 数据
	result := map[string]any{
		"query": map[string]any{
			"category": query.Category,
			"source":   query.Source,
		},
		"body": map[string]any{
			"name":        body.Name,
			"description": body.Description,
			"price":       body.Price,
			"tags":        body.Tags,
		},
		"combined": map[string]any{
			"category":    query.Category,
			"source":      query.Source,
			"name":        body.Name,
			"description": body.Description,
			"price":       body.Price,
			"tags":        body.Tags,
		},
	}

	return echoApi.BaseHttpResponse{
		Data: result,
	}
}

// UpdateUser 示例：PUT 请求，返回错误示例
// func (a *Auth) UpdateUser(c echoApi.GContext, req UpdateUserReq) echoApi.HttpResponse {
// 	if req.ID == "" {
// 		panic(echoApi.BaseHttpError{
// 			StatusCode: http.StatusBadRequest,
// 			Code:       "INVALID_PARAM",
// 			Message:    "ID 不能为空",
// 		})
// 	}
// 	return echoApi.BaseHttpResponse{
// 		Data: map[string]any{"updated": true},
// 	}
// }
