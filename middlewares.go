package echoApi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"log/slog"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

// 约定 在echo.Context 中 插入 context = conetxt.Context{}, 携带reqeustId属性
// BaseErrorMiddleware 全局 panic 捕获中间件（Echo 版本）
func BaseErrorMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// 生成 requestId,
			requestId := strconv.FormatInt(time.Now().UnixMilli(), 10) + RandStr(4)
			defer func() {
				if rec := recover(); rec != nil {
					// 打印堆栈
					trace := string(debug.Stack()) // 你已有的封装

					// 记录错误日志
					slog.Error("base panic",
						"err", rec,
						"trace", trace,
						"requestId", c.Get("requestId"),
						"method", c.Request().Method,
						"uri", c.Request().URL.Path,
					)

					// 构造统一错误响应
					htperr := BaseHttpError{
						StatusCode: 500,
						Message:    "system error",
					}
					_ = c.JSON(htperr.GetStatusCode(), htperr.GetResponse(requestId))
				}
			}()
			ctx := context.WithValue(context.Background(), "requestId", requestId)
			c.Set("context", ctx)
			return next(c)
		}
	}
}

// CorsConfig CORS 配置
type CorsConfig struct {
	AllowOrigins     []string // 允许的源（生产环境不应使用 "*"）
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCorsConfig 默认 CORS 配置（开发环境）
func DefaultCorsConfig() CorsConfig {
	return CorsConfig{
		AllowOrigins:     []string{"*"}, // 开发环境可以使用 "*"，生产环境应该指定具体域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With", "X-Username", "X-ChannelId"},
		ExposeHeaders:    []string{"X-Username", "X-ChannelId"},
		AllowCredentials: false, // 注意：当 AllowOrigins 包含 "*" 时，AllowCredentials 必须为 false
		MaxAge:           86400,
	}
}

// ToHeaders 将配置转换为 HTTP 头部
func (c CorsConfig) ToHeaders() map[string]string {
	headers := make(map[string]string)

	if len(c.AllowOrigins) > 0 {
		headers["Access-Control-Allow-Origin"] = strings.Join(c.AllowOrigins, ", ")
	}
	if len(c.AllowMethods) > 0 {
		headers["Access-Control-Allow-Methods"] = strings.Join(c.AllowMethods, ",")
	}
	if len(c.AllowHeaders) > 0 {
		headers["Access-Control-Allow-Headers"] = strings.Join(c.AllowHeaders, ",")
	}
	if len(c.ExposeHeaders) > 0 {
		headers["Access-Control-Expose-Headers"] = strings.Join(c.ExposeHeaders, ",")
	}
	if c.AllowCredentials {
		headers["Access-Control-Allow-Credentials"] = "true"
	}
	if c.MaxAge > 0 {
		headers["Access-Control-Max-Age"] = fmt.Sprintf("%d", c.MaxAge)
	}

	return headers
}

// CorsMiddleware 创建 CORS 中间件
func CorsMiddleware(config CorsConfig) echo.MiddlewareFunc {
	headers := config.ToHeaders()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			// 设置 CORS 响应头
			for key, value := range headers {
				res.Header().Set(key, value)
			}

			// OPTIONS 请求直接返回
			if req.Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}

func EchoLogger(serverLogHide, hideServerMiddleLogHeaders bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			ctx := c.Get("context").(context.Context)
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
			ip := c.RealIP()

			slog.Info("",
				"method", c.Request().Method,
				"requestId", ctx.Value("requestId"),
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
			ctx := c.Get("context").(context.Context)

			// WebSocket 升级请求跳过响应拦截（避免干扰握手）
			if c.Request().Header.Get("Upgrade") == "websocket" {
				return next(c)
			}

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
				slog.Error("响应拦截前处理失败", "error", err.Error(), "requestId", ctx.Value("requestId"))
				return err
			}
			nb := f(c, writer)

			writer.ResponseWriter.WriteHeader(writer.status)
			_, err = writer.ResponseWriter.Write(nb)
			if err != nil {
				slog.Error("响应拦截后处理失败", "error", err.Error(), "requestId", c.Get("requestId"))
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
			ctx := c.Get("context").(context.Context)

			// WebSocket 升级请求跳过响应处理中间件（避免干扰握手）
			if c.Request().Header.Get("Upgrade") == "websocket" {
				return next(c)
			}

			requestId := ctx.Value("requestId").(string)
			var resStatus = http.StatusInternalServerError

			defer func() {
				if r := recover(); r != nil {
					// 判断是否为 HttpError
					if he, ok := r.(HttpError); ok {
						if errorResponseHandler != nil {
							he = errorResponseHandler(c, he)
						}
						statusCode := he.GetStatusCode()
						_ = c.JSON(statusCode, he.GetResponse(requestId))
						resStatus = statusCode
					} else {
						// 统一错误响应格式
						_ = c.JSON(resStatus, BaseHttpError{
							StatusCode: resStatus,
							Code:       "INTERNAL_ERROR",
							Message:    "内部服务器错误",
							RequestId:  requestId,
						}.GetResponse(requestId))
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
			if err != nil {
				var er *echo.HTTPError
				if errors.As(err, &er) {
					htperr := BaseHttpError{
						StatusCode: er.Code,
						Message:    err.Error(),
					}
					return c.JSON(htperr.StatusCode, htperr.GetResponse(requestId))
				} else {
					panic(err)
				}
			}

			// 如果中间件或 handler 设置了返回值到 context
			if v := c.Get("Response"); v != nil {
				switch val := v.(type) {
				case HttpError:
					if errorResponseHandler != nil {
						val = errorResponseHandler(c, val)
					}
					return c.JSON(val.GetStatusCode(), val.GetResponse(requestId))
				case HttpResponse:
					if normalResponseHandler != nil {
						val = normalResponseHandler(c, val)
					}
					return c.JSON(val.GetStatusCode(), val.GetResponse(requestId))
				default:
					return c.JSON(http.StatusOK, val)
				}
			}

			return err
		}
	}
}

// fillParamWithDefaultOptimized 优化版本的默认值填充（使用字段索引而非字段名）
// 性能优化：避免运行时 FieldByName 查找
func fillParamWithDefaultOptimized(arg reflect.Value, defaultFields []DefaultFieldInfo) {
	argType := arg.Type()

	for _, fieldInfo := range defaultFields {
		if fieldInfo.FieldIndex >= argType.NumField() {
			continue
		}

		valueField := arg.Field(fieldInfo.FieldIndex)
		if !valueField.CanSet() {
			continue
		}

		// 只有在值为零值时才设置默认值
		if isZero(valueField) {
			// 确保类型兼容
			if fieldInfo.DefaultVal.Type().AssignableTo(valueField.Type()) {
				valueField.Set(fieldInfo.DefaultVal)
			} else if fieldInfo.DefaultVal.Type().ConvertibleTo(valueField.Type()) {
				valueField.Set(fieldInfo.DefaultVal.Convert(valueField.Type()))
			}
		}
	}
}
