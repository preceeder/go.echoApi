package echoApi

import (
	"github.com/labstack/echo/v4"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
)

type ParamBinding struct {
	Params      reflect.Type
	DefaultData map[string]*reflect.Value // data 的默认对象
}

type HandlerCache struct {
	path                string                //url路径
	httpMethod          string                //http方法 get post
	HandlerFunc         reflect.Value         // 业务 handler
	Params              []ParamBinding        // 参数绑定信息
	Middlewares         []echo.MiddlewareFunc // 接口中间件
	NoUseBasePrefixPath bool                  // 是否禁用 BasePrefixPathInvalid   bool
	CtxParams           map[string]string     // 可以写入 ctx 的数据
}

type ApiRouteConfig struct {
	ALL    *ApiRouterSubConfig // 这个里面就只能是一个对象了
	POST   []ApiRouterSubConfig
	GET    []ApiRouterSubConfig
	PUT    []ApiRouterSubConfig
	DELETE []ApiRouterSubConfig
}
type ApiRouterSubConfig struct {
	FuncName            any // func or string
	Path                any // string
	Middlewares         []echo.MiddlewareFunc
	NoUseAllConfig      bool              // 是否禁用全局配置  ALL的配置
	NoUseModel          any               // uri 是否使用model 名   bool
	NoUseBasePrefixPath any               // 是否禁用 BasePrefixPathInvalid   bool
	CtxParams           map[string]string // 可以写入 ctx 的数据
}

// BasePrefixPath 全局的路由前缀
var BasePrefixPath string = "/api"

// Routes 路由集合
var Routes = []HandlerCache{}

// Register 注册控制器
func Register(controller interface{}) bool {
	ctrlName := reflect.TypeOf(controller).String()
	module := ctrlName
	if strings.Contains(ctrlName, ".") {
		module = ctrlName[strings.Index(ctrlName, ".")+1:]
	}
	v := reflect.ValueOf(controller)

	// 获取接口配置
	arc := v.Elem().FieldByName("ApiRouteConfig")
	var apiData ApiRouteConfig
	if arc.IsValid() {
		apiData = arc.Interface().(ApiRouteConfig)
	} else {
		slog.Error("接口配置错误", "func name", ctrlName)
		return false
	}

	var pathMap map[string][]RouteConfiguration
	// 将路由 扁平化 为 map[string][]PW
	pathMap = ExpandTheRoute(apiData, strings.ToLower(module))

	//遍历方法
	for i := 0; i < v.NumMethod(); i++ {
		method := v.Method(i)
		action := v.Type().Method(i).Name
		hmd, isIn := pathMap[action]
		if !isIn {
			continue
		}
		for _, dv := range hmd {
			httpMethod := dv.HttpMethod
			//路径处理
			middlewares := dv.Middlewares
			//遍历参数
			paramsNum := method.Type().NumIn()
			params := make([]ParamBinding, 0, paramsNum)
			for j := 1; j < paramsNum; j++ {
				pp := method.Type().In(j)
				// 需要处理一下 默认值
				var DefaultData = map[string]*reflect.Value{}
				DefaultData = buildDefaultData(pp) // 还有数组类型
				params = append(params, ParamBinding{Params: pp, DefaultData: DefaultData})

			}
			route := HandlerCache{
				path:                dv.Path,
				HandlerFunc:         method,
				Params:              params,
				httpMethod:          httpMethod,
				Middlewares:         middlewares,
				NoUseBasePrefixPath: dv.NoUseBasePrefixPath,
				CtxParams:           dv.CtxParams,
			}
			Routes = append(Routes, route)
		}
	}
	return true
}

type RouteConfiguration struct {
	Path                string
	Middlewares         []echo.MiddlewareFunc
	HttpMethod          string
	NoUseBasePrefixPath bool
	CtxParams           map[string]string // 可以写入 ctx 的数据
}

var Methods = []string{"GET", "POST", "PUT", "DELETE"}

func ExpandTheRoute(apiData ApiRouteConfig, module string) map[string][]RouteConfiguration {
	sd := map[string][]RouteConfiguration{}
	var ac *ApiRouterSubConfig

	if apiData.ALL != nil {
		ac = apiData.ALL
	}

	for _, method := range Methods {
		var tc []ApiRouterSubConfig

		switch method {
		case "GET":
			tc = apiData.GET
		case "POST":
			tc = apiData.POST
		case "PUT":
			tc = apiData.PUT
		case "DELETE":
			tc = apiData.DELETE
		default:
			slog.Error("不支持的请求类型", "method", method)
		}

		if tc != nil {
			for _, vl := range tc {
				funname := ""
				if reflect.TypeOf(vl.FuncName).Kind().String() == "func" {
					dd := reflect.ValueOf(vl.FuncName)
					pname := runtime.FuncForPC(dd.Pointer()).Name()
					funname = strings.TrimSuffix(pname, "-fm")
					fun := strings.Split(funname, ".")
					funname = fun[len(fun)-1]
				} else if reflect.TypeOf(vl.FuncName).Kind().String() == "string" {
					funname, _ = vl.FuncName.(string)
				} else {
					panic("不支持的数据类型")
				}

				temp := []ApiRouterSubConfig{vl}
				if ac != nil && !vl.NoUseAllConfig {
					// 全局配置放在最后
					temp = append(temp, *ac)
				}
				rpath, middlewares, noUseBasePrifix, ctxParams := subApiconfigPares(temp, module)
				sd[funname] = append(sd[funname], RouteConfiguration{Path: rpath, Middlewares: middlewares, HttpMethod: method, NoUseBasePrefixPath: noUseBasePrifix, CtxParams: ctxParams})
			}
		}
	}
	return sd
}

func subApiconfigPares(config []ApiRouterSubConfig, module string) (path string, middlewares []echo.MiddlewareFunc, noUseBasePrefix bool, ctxParams map[string]string) {
	noUseBasePrefix = false
	lc := len(config)
	path = ""
	pathModel := ""
	ctxParams = make(map[string]string)
	if lc > 0 {
		for i := 0; i < lc; i++ {
			if config[i].NoUseBasePrefixPath != nil {
				if config[i].NoUseBasePrefixPath.(bool) == true {
					noUseBasePrefix = true
				}
				break
			}
		}

		for i := lc - 1; i >= 0; i-- {
			if config[i].CtxParams != nil {
				for ctxk, ctxv := range config[i].CtxParams {
					ctxParams[ctxk] = ctxv
				}
			}
		}

		for i := 0; i < lc; i++ {
			if config[i].NoUseModel != nil {
				if config[i].NoUseModel.(bool) == false {
					pathModel = module
				} else {
					pathModel = ""
				}
				break
			} else {
				pathModel = module
			}
		}

		for i := 0; i < lc; i++ {
			if config[i].Path != nil {
				pt, _ := config[i].Path.(string)
				if pt != "" {
					path += pt
				}
				break
			}
		}

		for i := lc - 1; i >= 0; i-- {
			// 优先执行 all的中间件, 然后在执行 私有的 中间件
			if config[i].Middlewares != nil {
				middlewares = append(middlewares, config[i].Middlewares...)
				//break
			}
		}
	} else {
		noUseBasePrefix = false
		pathModel = module
	}

	var pathSlice []string
	path = pathModel + path
	for _, v := range strings.Split(path, "/") {
		if v != "" {
			pathSlice = append(pathSlice, v)
		}
	}
	path = "/" + strings.Join(pathSlice, "/")
	return
}

func MountRoutes(e *echo.Echo) {
	for _, route := range Routes {

		handlerFunc := CtxPramsHandler(route)

		// 加上中间件
		fullHandler := handlerFunc
		if len(route.Middlewares) > 0 {
			fullHandler = func(c echo.Context) error {
				h := handlerFunc
				for i := len(route.Middlewares) - 1; i >= 0; i-- {
					h = route.Middlewares[i](h)
				}
				return h(c)
			}
		}
		fulleHandler := matchRoute(route)(fullHandler)

		finalPath := route.path
		// 处理 BasePrefixPath
		if BasePrefixPath != "" && !route.NoUseBasePrefixPath {
			finalPath = strings.TrimSuffix(BasePrefixPath, "/") + finalPath
		}

		switch strings.ToUpper(route.httpMethod) {
		case "GET":
			e.GET(finalPath, fulleHandler)
		case "POST":
			e.POST(finalPath, fulleHandler)
		case "PUT":
			e.PUT(finalPath, fulleHandler)
		case "DELETE":
			e.DELETE(finalPath, fulleHandler)
		default:
			slog.Error("MountRoutes错误", "method", route.httpMethod)
		}
	}
}

// CtxPramsHandler  路由控制参数写入
func CtxPramsHandler(handler HandlerCache) echo.HandlerFunc {
	return func(c echo.Context) error {
		if handler.CtxParams != nil {
			for key, value := range handler.CtxParams {
				c.Set(key, value)
			}
		}
		return nil
	}
}
