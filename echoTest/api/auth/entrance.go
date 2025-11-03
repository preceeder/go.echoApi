package auth

import (
	"github.com/preceeder/go.echoApi"
)

type Auth struct{}

// RouteConfig 实现 Controller 接口，定义路由配置
func (a *Auth) RouteConfig() echoApi.RouteConfig {
	return echoApi.RouteConfig{
		// 全局配置
		Global: &echoApi.RouteBuilder{
			UseModel:            false, // 不使用模型名作为路径前缀
			NoUseBasePrefixPath: false,
			CtxParams: map[string]string{
				"RequestUnSecret": "true",
			},
		},
		// GET 方法的路由
		GET: []echoApi.RouteBuilder{
			{
				Path:     "/time",
				FuncName: a.GetTime, // 支持函数引用形式
				CtxParams: map[string]string{
					"PrintResponse": "true",
					"ResponseSecret": "false",
				},
			},
			{
				Path:     "/token",
				FuncName: a.GetToken, // 支持函数引用形式
				CtxParams: map[string]string{
					"PrintResponse": "true",
					"ResponseSecret": "false",
				},
			},
		},
		// POST 方法的路由
		POST: []echoApi.RouteBuilder{
			{
				Path:     "/create",
				FuncName: a.CreateData, // POST 接口，整合 query 和 body 参数
				CtxParams: map[string]string{
					"PrintResponse": "true",
					"ResponseSecret": "false",
				},
			},
		},
	}
}

func init() {
	// 注册控制器
	if err := echoApi.Register(&Auth{}); err != nil {
		panic("注册 Auth 控制器失败: " + err.Error())
	}
}
