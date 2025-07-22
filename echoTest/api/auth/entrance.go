package auth

import (
	"github.com/preceeder/go/echoApi"
)

type Auth struct {
	ApiRouteConfig echoApi.ApiRouteConfig
}

func init() {
	a := Auth{}

	a.ApiRouteConfig = echoApi.ApiRouteConfig{
		ALL: &echoApi.ApiRouterSubConfig{
			NoUseModel: true,
			CtxParams: map[string]string{
				"RequestUnSecret": "true",
			},
		},
		GET: []echoApi.ApiRouterSubConfig{
			{
				Path:     "/time",
				FuncName: a.GetTime,
				CtxParams: map[string]string{
					"PrintResponse":   "true",
					"RequestUnSecret": "true",
					//"ResponseSecret":  "false",
				},
			},
			{
				Path:     "/token",
				FuncName: a.GetToken,
			},
		},
	}
	echoApi.Register(&a)
}
