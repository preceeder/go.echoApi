package chat

import (
	"github.com/preceeder/go.echoApi"
)

type Chat struct{}

// RouteConfig 实现 Controller 接口，定义路由配置
func (c *Chat) RouteConfig() echoApi.RouteConfig {
	return echoApi.RouteConfig{
		// 全局配置
		Global: &echoApi.RouteBuilder{
			UseModel:            false, // 不使用模型名作为路径前缀
			NoUseBasePrefixPath: false,
			CtxParams: map[string]string{
				"RequestUnSecret": "true", // WebSocket 通常不需要加密
			},
		},
		// WebSocket 路由
		WS: []echoApi.RouteBuilder{
			{
				Path:     "/ws",
				FuncName: c.HandleWebSocket, // WebSocket 处理函数
				CtxParams: map[string]string{
					"PrintResponse": "false", // WebSocket 不需要打印响应
				},
			},
			{
				Path:     "/ws/echo",
				FuncName: c.HandleEcho, // 回显示例
			},
		},
	}
}

func init() {
	// 注册控制器
	if err := echoApi.Register(&Chat{}); err != nil {
		panic("注册 Chat 控制器失败: " + err.Error())
	}
}

