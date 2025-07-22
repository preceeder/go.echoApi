package echoApi

import (
	"bytes"
	"github.com/labstack/echo/v4"
	"github.com/preceeder/go/base"
	"log/slog"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"time"
)

// BaseErrorMiddleware 全局 panic 捕获中间件（Echo 版本）
func BaseErrorMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					// 打印堆栈
					trace := string(debug.Stack()) // 你已有的封装

					// 记录错误日志
					slog.Error("base panic",
						"err", rec,
						"trace", trace,
						"REQUESTID", c.Get("requestId"),
						"USERID", c.Get("userId"), // Echo 中从 context 中取值
						"method", c.Request().Method,
						"uri", c.Request().URL.Path,
					)

					// 构造统一错误响应
					htperr := BaseHttpError{
						Code:      500,
						ErrorCode: CodeSystemError,
						Message:   "system error",
					}
					_ = c.JSON(htperr.GetCode(), htperr.GetResponse())
				}
			}()

			// 生成 requestId
			requestId := strconv.FormatInt(time.Now().UnixMilli(), 10) + RandStr(4)
			c.Set("requestId", requestId)
			return next(c)
		}
	}
}

var DefaultHeaders = map[string]string{
	"Access-Control-Allow-Origin":      "*",
	"Access-Control-Allow-Methods":     "OPTIONS,GET,POST,PUT,DELETE",
	"Access-Control-Allow-Headers":     "Content-Length,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type,x-auth-version,x-auth-channel,x-auth-channel-detail,x-auth-package,x-auth-timestamp,x-auth-announce,x-auth-token,x-auth-app,x-auth-signature",
	"Access-Control-Expose-Headers":    "Content-Length,Access-Control-Allow-Origin,Access-Control-Allow-Headers,Cache-Control,Content-Language,Content-Type",
	"Access-Control-Allow-Credentials": "true",
}

func CorsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			// 设置默认 CORS 响应头（你可以定义 DefaultHeaders）
			for key, value := range DefaultHeaders {
				res.Header().Set(key, value)
			}

			// OPTIONS 请求直接返回
			if req.Method == http.MethodOptions {
				return c.NoContent(http.StatusOK)
			}

			return next(c)
		}
	}
}

func EchoLogger(serverLogHide, hideServerMiddleLogHeaders bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// 执行后续处理
			err := next(c)
			cost := time.Since(start)

			if serverLogHide {
				return err
			}

			// 获取参数
			params := GetRequestParamsEcho(c)

			// 获取 headers
			headers := http.Header{}
			if !hideServerMiddleLogHeaders {
				headers = c.Request().Header
			}

			// 获取 client IP（可根据需要调整）
			ip := c.Request().Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = c.RealIP()
			}

			slog.Info("",
				"method", c.Request().Method,
				"requestId", c.Get("requestId"),
				"userId", c.Get("userId"),
				"status", c.Response().Status,
				"ip", ip,
				"headers", headers,
				"errors", err,
				"params", params,
				"cost", cost.Milliseconds())

			return err
		}
	}
}

// 自定义 Response 捕获器
type ResponseInterceptor struct {
	http.ResponseWriter
	Body   *bytes.Buffer
	status int
}

func (r *ResponseInterceptor) Write(b []byte) (int, error) {
	return r.Body.Write(b) // 不立即写出去
}

func (r *ResponseInterceptor) WriteHeader(code int) {
	r.status = code
}

func InterceptMiddleware(f func(c echo.Context, w *ResponseInterceptor) []byte) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orig := c.Response().Writer

			writer := &ResponseInterceptor{
				ResponseWriter: orig,
				Body:           &bytes.Buffer{},
				status:         http.StatusOK,
			}
			defer writer.Body.Reset()
			// 替换 writer 为我们自己的
			c.Response().Writer = writer

			err := next(c)
			if err != nil {
				slog.Error("响应拦截前处理失败", "error", err.Error(), "REQUEST", c.Get("requestId"))
				return err
			}
			nb := f(c, writer)

			writer.ResponseWriter.WriteHeader(writer.status)
			_, err = writer.ResponseWriter.Write(nb)
			if err != nil {
				slog.Error("响应拦截后处理失败", "error", err.Error(), "REQUEST", c.Get("requestId"))
				return err
			}
			return nil
		}
	}
}

// EchoResponseAndRecoveryHandler 响应和错误处理
// ginRecoveryMidFuncs 错误发生时 的处理
// normalResponseHandler 正常响应的 额外处理
// errorResponseHandler 错误响应的 额外处理
func EchoResponseAndRecoveryHandler(
	normalResponseHandler func(c echo.Context, res HttpResponse) HttpResponse,
	errorResponseHandler func(c echo.Context, res HttpError) HttpError,
	recoveryMidFuncs ...func(c echo.Context, code int, err any, requestId string),
) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestId, _ := c.Get("requestId").(string)
			var resStatus = http.StatusInternalServerError

			defer func() {
				if r := recover(); r != nil {
					// 判断是否为 HttpError
					if he, ok := r.(HttpError); ok {
						if errorResponseHandler != nil {
							he = errorResponseHandler(c, he)
						}
						_ = c.JSON(he.GetCode(), he.GetResponse())
						resStatus = he.GetCode()
					} else {
						_ = c.JSON(resStatus, map[string]any{
							"success":   false,
							"errorCode": 10000,
							"message":   "",
						})
					}

					slog.Error("Recovery from panic",
						"err", r,
						"trace", string(debug.Stack()),
						"uri", c.Request().URL.String(),
						"method", c.Request().Method,
						"header", c.Request().Header,
						"requestId", requestId,
					)

					for _, f := range recoveryMidFuncs {
						f(c, resStatus, r, requestId)
					}
				}
			}()

			err := next(c)

			// 如果中间件或 handler 设置了返回值到 context
			if v := c.Get("Response"); v != nil {
				switch val := v.(type) {
				case HttpError:
					if errorResponseHandler != nil {
						val = errorResponseHandler(c, val)
					}
					return c.JSON(val.GetCode(), val.GetResponse())
				case HttpResponse:
					if normalResponseHandler != nil {
						val = normalResponseHandler(c, val)
					}
					return c.JSON(val.GetCode(), val.GetResponse())
				default:
					return c.JSON(http.StatusOK, val)
				}
			}

			return err
		}
	}
}

// 路由匹配处理， 内部使用
func matchRoute(route HandlerCache) echo.MiddlewareFunc {
	handlerFunc := func(f echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 准备反射参数
			err := f(c)
			if err != nil {
				return err
			}

			invokeArgs := []reflect.Value{}

			// Handler 第一个参数是 receiver，所以 nil
			//invokeArgs = append(invokeArgs, reflect.Value{})

			ctx := base.Context{RequestId: c.Get("requestId").(string)}
			if userId := c.Get("userId"); userId != nil {
				ctx.UserId = base.UserId(c.Get("userId").(string))
			}
			gc := GContext{c, ctx}
			invokeArgs = append(invokeArgs, reflect.ValueOf(gc))
			// 遍历 Params 参数绑定
			for _, paramBind := range route.Params {
				// 生成参数对象
				var paramType reflect.Type = paramBind.Params
				var isPtr = false
				if paramBind.Params.Kind() == reflect.Ptr {
					paramType = paramBind.Params.Elem()
					isPtr = true
				}

				arg := reflect.New(paramType)
				err = c.Bind(arg.Interface())
				if err != nil {
					slog.Error("", "paramBind", arg.Interface(), "err", err.Error())
				}

				// 设置默认值
				if len(paramBind.DefaultData) > 0 {
					fillParamWithDefault(arg.Elem(), paramBind.DefaultData)
				}
				if isPtr {
					invokeArgs = append(invokeArgs, arg)
				} else {
					invokeArgs = append(invokeArgs, arg.Elem())
				}
			}

			// 调用
			out := route.HandlerFunc.Call(invokeArgs)

			if len(out) > 0 && !out[0].IsNil() {
				c.Set("Response", out[0].Interface())
			}
			return nil
		}
	}
	return handlerFunc
}

func fillParamWithDefault(arg reflect.Value, defaultData map[string]*reflect.Value) {
	argType := arg.Type()

	for i := 0; i < argType.NumField(); i++ {
		field := argType.Field(i)
		valueField := arg.Field(i)

		if !valueField.CanSet() {
			continue
		}

		fieldName := field.Name

		// 看当前值是不是零值
		if isZero(valueField) {
			if defVal, ok := defaultData[fieldName]; ok {
				valueField.Set(*defVal)
			}
		}

		// 如果是匿名嵌套结构体，递归
		if field.Anonymous && valueField.Kind() == reflect.Struct {
			fillParamWithDefault(valueField, defaultData)
		}
	}
}
