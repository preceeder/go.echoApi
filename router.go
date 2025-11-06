package echoApi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// DefaultFieldInfo 默认值字段信息（优化：使用索引而非字段名）
type DefaultFieldInfo struct {
	FieldIndex int           // 字段索引，避免运行时 FieldByName 查找
	DefaultVal reflect.Value // 默认值
}

type ParamBinding struct {
	Params        reflect.Type       // 原始参数类型（可能是指针）
	ElemType      reflect.Type       // 元素类型（如果是指针，则为指向的类型）
	IsPtr         bool               // 是否为指针类型（预计算，避免运行时判断）
	DefaultFields []DefaultFieldInfo // 按字段索引的默认值（性能优化）
}

type Route struct {
	Path                string                // URL 路径
	Method              string                // HTTP 方法
	Handler             reflect.Value         // 业务 handler
	Params              []ParamBinding        // 参数绑定信息
	Middlewares         []echo.MiddlewareFunc // 接口中间件
	NoUseBasePrefixPath bool                  // 是否禁用 BasePrefixPath
	CtxParams           map[string]string     // 可以写入 ctx 的数据
}

// RouteBuilder 路由构建器，提供类型安全的路由配置
type RouteBuilder struct {
	Method              string
	Path                string
	FuncName            any // 支持 string 或函数引用（如 a.GetTime）
	Middlewares         []echo.MiddlewareFunc
	NoUseAllConfig      bool
	UseModel            bool // 是否使用模型名作为路径前缀
	NoUseBasePrefixPath bool
	CtxParams           map[string]string
}

// RouteConfig 路由配置，支持全局和按方法配置
type RouteConfig struct {
	Global *RouteBuilder // 全局配置
	GET    []RouteBuilder
	POST   []RouteBuilder
	PUT    []RouteBuilder
	DELETE []RouteBuilder
	WS     []RouteBuilder // WebSocket 路由
}

// Controller 控制器接口，所有控制器必须实现
type Controller interface {
	RouteConfig() RouteConfig
}

// BasePrefixPath 全局的路由前缀
var BasePrefixPath = "/api"

// routes 路由集合（使用互斥锁保护）
var (
	routes   []Route
	routesMu sync.RWMutex
)

// Register 注册控制器（类型安全版本）
func Register(ctrl Controller) error {
	ctrlType := reflect.TypeOf(ctrl)
	if ctrlType.Kind() != reflect.Ptr {
		return fmt.Errorf("controller must be a pointer, got %T", ctrl)
	}

	ctrlValue := reflect.ValueOf(ctrl)
	module := getModuleName(ctrlType)

	config := ctrl.RouteConfig()

	// 将路由扁平化为 map[方法名][]RouteBuilder
	pathMap := expandRouteConfig(config, strings.ToLower(module))

	// 遍历控制器方法
	methodCount := ctrlType.NumMethod()
	for i := 0; i < methodCount; i++ {
		method := ctrlValue.Method(i)
		methodType := ctrlType.Method(i)
		actionName := methodType.Name

		builders, exists := pathMap[actionName]
		if !exists {
			continue
		}

		for _, builder := range builders {
			params, err := buildParamBindings(methodType)
			if err != nil {
				slog.Error("构建参数绑定失败", "method", actionName, "error", err)
				continue
			}

			route := Route{
				Path:                builder.Path,
				Method:              builder.Method,
				Handler:             method,
				Params:              params,
				Middlewares:         builder.Middlewares,
				NoUseBasePrefixPath: builder.NoUseBasePrefixPath,
				CtxParams:           builder.CtxParams,
			}

			routesMu.Lock()
			routes = append(routes, route)
			routesMu.Unlock()
		}
	}

	return nil
}

// getModuleName 获取模块名（去掉包名前缀）
func getModuleName(t reflect.Type) string {
	typeName := t.String()
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		return typeName[idx+1:]
	}
	return typeName
}

// extractFuncName 从 FuncName 字段提取函数名（支持字符串和函数引用）
func extractFuncName(funcName any) string {
	if funcName == nil {
		return ""
	}

	// 如果是字符串，直接返回
	if s, ok := funcName.(string); ok {
		return s
	}

	// 如果是函数引用，使用反射提取函数名
	funcType := reflect.TypeOf(funcName)
	if funcType.Kind() == reflect.Func {
		funcValue := reflect.ValueOf(funcName)
		pc := funcValue.Pointer()
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			fullName := fn.Name()
			// 移除 "-fm" 后缀（如果是方法值）
			fullName = strings.TrimSuffix(fullName, "-fm")
			// 提取最后的函数名
			parts := strings.Split(fullName, ".")
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}

	return ""
}

// hasJsonField 检查结构体类型是否包含需要从 body 绑定的字段（有 json 标签且没有 query/param 标签）
func hasJsonField(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		// 检查是否有 json 标签，且没有 query 和 param 标签
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if field.Tag.Get("query") == "" && field.Tag.Get("param") == "" {
				return true
			}
		}
	}
	return false
}

// bindParamPrecise 精确绑定参数，根据字段标签从不同源绑定，避免冲突
func bindParamPrecise(c echo.Context, target interface{}, paramType reflect.Type, bodyBytes []byte) error {
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}
	if paramType.Kind() != reflect.Struct {
		return fmt.Errorf("参数类型必须是结构体")
	}

	targetValue := reflect.ValueOf(target).Elem()
	queryParams := c.QueryParams()

	// 解析 body（如果需要）
	var bodyMap map[string]interface{}
	needBody := false
	for i := 0; i < paramType.NumField(); i++ {
		field := paramType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if field.Tag.Get("query") == "" && field.Tag.Get("param") == "" {
				needBody = true
				break
			}
		}
	}

	if needBody && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
			// body 解析失败不是致命错误，继续处理其他字段
			bodyMap = make(map[string]interface{})
		}
	}

	// 遍历字段，根据标签从不同源绑定
	for i := 0; i < paramType.NumField(); i++ {
		field := paramType.Field(i)
		fieldValue := targetValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		queryTag := field.Tag.Get("query")
		jsonTag := field.Tag.Get("json")
		paramTag := field.Tag.Get("param")

		// 优先级：param > query > json（路径参数 > 查询参数 > body）
		if paramTag != "" && paramTag != "-" {
			// 从路径参数绑定
			if paramValue := c.Param(paramTag); paramValue != "" {
				if err := setFieldValueFromString(fieldValue, field.Type, paramValue); err != nil {
					return fmt.Errorf("字段 %s (param:%s) 绑定失败: %w", field.Name, paramTag, err)
				}
				continue
			}
		}

		if queryTag != "" && queryTag != "-" {
			// 从 query 参数绑定
			if queryValue := queryParams.Get(queryTag); queryValue != "" {
				if err := setFieldValueFromString(fieldValue, field.Type, queryValue); err != nil {
					return fmt.Errorf("字段 %s (query:%s) 绑定失败: %w", field.Name, queryTag, err)
				}
				continue
			}
		}

		if jsonTag != "" && jsonTag != "-" && queryTag == "" && paramTag == "" {
			// 从 body 绑定（只有 json 标签，没有 query/param 标签）
			jsonName := strings.Split(jsonTag, ",")[0]
			if bodyValue, exists := bodyMap[jsonName]; exists {
				if err := setFieldValueFromJSON(fieldValue, field.Type, bodyValue); err != nil {
					return fmt.Errorf("字段 %s (json:%s) 绑定失败: %w", field.Name, jsonName, err)
				}
				continue
			}
		}
	}

	return nil
}

// setFieldValueFromString 根据类型设置字段值（用于 query 和 param）
func setFieldValueFromString(fieldValue reflect.Value, fieldType reflect.Type, strValue string) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("字段不可设置")
	}

	switch fieldType.Kind() {
	case reflect.String:
		fieldValue.SetString(strValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(strValue, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(strValue, 10, 64)
		if err != nil {
			return err
		}
		fieldValue.SetUint(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(strValue, 64)
		if err != nil {
			return err
		}
		fieldValue.SetFloat(val)
	case reflect.Bool:
		val, err := strconv.ParseBool(strValue)
		if err != nil {
			return err
		}
		fieldValue.SetBool(val)
	default:
		// 对于复杂类型，使用 Echo 的默认绑定
		return fmt.Errorf("不支持的字段类型: %v", fieldType.Kind())
	}
	return nil
}

// setFieldValueFromJSON 从 JSON 值设置字段值
func setFieldValueFromJSON(fieldValue reflect.Value, fieldType reflect.Type, jsonValue interface{}) error {
	if !fieldValue.CanSet() {
		return fmt.Errorf("字段不可设置")
	}

	// 如果类型匹配，直接设置
	if jsonVal := reflect.ValueOf(jsonValue); jsonVal.Type().AssignableTo(fieldType) {
		fieldValue.Set(jsonVal)
		return nil
	}

	// 如果类型可转换，尝试转换
	if jsonVal := reflect.ValueOf(jsonValue); jsonVal.Type().ConvertibleTo(fieldType) {
		fieldValue.Set(jsonVal.Convert(fieldType))
		return nil
	}

	// 对于复杂类型，使用 JSON 序列化/反序列化
	jsonBytes, err := json.Marshal(jsonValue)
	if err != nil {
		return err
	}

	ptr := reflect.New(fieldType)
	if err := json.Unmarshal(jsonBytes, ptr.Interface()); err != nil {
		return err
	}

	fieldValue.Set(ptr.Elem())
	return nil
}

// expandRouteConfig 展开路由配置
func expandRouteConfig(config RouteConfig, module string) map[string][]RouteBuilder {
	result := make(map[string][]RouteBuilder)

	var global *RouteBuilder
	if config.Global != nil {
		global = config.Global
	}

	// 处理各个 HTTP 方法和 WebSocket
	methods := []struct {
		name     string
		builders []RouteBuilder
	}{
		{"GET", config.GET},
		{"POST", config.POST},
		{"PUT", config.PUT},
		{"DELETE", config.DELETE},
		{"WS", config.WS}, // WebSocket 路由
	}

	for _, m := range methods {
		for _, builder := range m.builders {
			// 合并全局配置
			if global != nil && !builder.NoUseAllConfig {
				builder = mergeBuilder(*global, builder)
			}

			builder.Method = m.name

			// 构建完整路径
			pathModel := ""
			if builder.UseModel {
				pathModel = module
			}

			fullPath := buildPath(pathModel, builder.Path)
			builder.Path = fullPath

			// 提取函数名（支持字符串和函数引用两种形式）
			funcName := extractFuncName(builder.FuncName)
			if funcName == "" {
				slog.Error("FuncName 不能为空", "method", m.name, "path", builder.Path)
				continue
			}

			result[funcName] = append(result[funcName], builder)
		}
	}

	return result
}

// mergeBuilder 合并全局配置和局部配置
func mergeBuilder(global, local RouteBuilder) RouteBuilder {
	result := local

	// 合并中间件（全局在前）
	if global.Middlewares != nil {
		result.Middlewares = append(global.Middlewares, local.Middlewares...)
	}

	// 合并 CtxParams
	if global.CtxParams != nil {
		if result.CtxParams == nil {
			result.CtxParams = make(map[string]string)
		}
		for k, v := range global.CtxParams {
			result.CtxParams[k] = v
		}
	}

	return result
}

// buildPath 构建完整路径
func buildPath(model, path string) string {
	var parts []string
	if model != "" {
		parts = append(parts, model)
	}
	if path != "" {
		parts = append(parts, strings.TrimPrefix(path, "/"))
	}

	cleanParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			cleanParts = append(cleanParts, p)
		}
	}

	return "/" + strings.Join(cleanParts, "/")
}

// buildParamBindings 构建参数绑定信息
// 注意：第一个参数（索引1）应该是 GContext，会被跳过，因为 buildHandler 会手动添加
func buildParamBindings(methodType reflect.Method) ([]ParamBinding, error) {
	numParams := methodType.Type.NumIn()

	// 参数说明：
	// - 索引 0: receiver（方法的接收者）
	// - 索引 1: 应该是 GContext（会在 buildHandler 中手动处理，跳过）
	// - 索引 2+: 业务参数（需要绑定）

	if numParams <= 1 {
		// 只有 receiver，没有参数
		return nil, nil
	}

	// 从第三个参数开始（跳过 receiver 和 GContext）
	startIndex := 2
	if numParams <= startIndex {
		// 只有 receiver 和 GContext，没有业务参数
		return nil, nil
	}

	params := make([]ParamBinding, 0, numParams-startIndex)
	for i := startIndex; i < numParams; i++ {
		paramType := methodType.Type.In(i)

		isPtr := paramType.Kind() == reflect.Ptr
		elemType := paramType
		if isPtr {
			elemType = paramType.Elem()
		}

		// 构建默认值字段信息（传递原始参数类型，函数内部会处理）
		defaultFields := buildDefaultFieldsWithIndex(paramType)

		params = append(params, ParamBinding{
			Params:        paramType,
			ElemType:      elemType,
			IsPtr:         isPtr,
			DefaultFields: defaultFields,
		})
	}

	return params, nil
}

// MountRoutes 挂载所有路由到 Echo 实例
func MountRoutes(e *echo.Echo) {
	routesMu.RLock()
	defer routesMu.RUnlock()

	for _, route := range routes {
		handler := buildHandler(route)

		finalPath := route.Path
		if BasePrefixPath != "" && !route.NoUseBasePrefixPath {
			finalPath = strings.TrimSuffix(BasePrefixPath, "/") + finalPath
		}

		switch strings.ToUpper(route.Method) {
		case "GET":
			e.GET(finalPath, handler)
		case "POST":
			e.POST(finalPath, handler)
		case "PUT":
			e.PUT(finalPath, handler)
		case "DELETE":
			e.DELETE(finalPath, handler)
		case "WS":
			// WebSocket 使用 GET 方法注册，但在 handler 中检测升级
			e.GET(finalPath, buildWebSocketHandler(route))
		default:
			slog.Error("不支持的 HTTP 方法", "method", route.Method)
		}
	}
}

// buildHandler 构建路由处理器（支持中间件）
func buildHandler(route Route) echo.HandlerFunc {
	params := route.Params
	handlerFunc := route.Handler
	paramsCount := len(params)
	middlewares := route.Middlewares

	// 核心处理器
	coreHandler := func(c echo.Context) error {
		// 设置上下文参数
		if route.CtxParams != nil {
			for key, value := range route.CtxParams {
				c.Set(key, value)
			}
		}

		// 构建调用参数
		invokeArgs := make([]reflect.Value, 1, paramsCount+1)

		// 第一个参数是 GContext
		requestId, _ := c.Get("requestId").(string)
		invokeArgs[0] = reflect.ValueOf(c)

		// 绑定参数（支持多个参数）
		// 先检查是否有参数需要 body，如果有则提前保存，避免多次读取导致 EOF
		needBodyForAnyParam := false
		for i := range params {
			if hasJsonField(params[i].ElemType) {
				needBodyForAnyParam = true
				break
			}
		}

		// 如果需要 body，提前保存
		var bodyBytes []byte
		if needBodyForAnyParam {
			bodyBytes, _ = io.ReadAll(c.Request().Body)
			c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		for i := range params {
			paramBind := &params[i]

			arg := reflect.New(paramBind.ElemType)

			// 如果已保存 body，确保每次绑定前都能读取
			if needBodyForAnyParam {
				c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// 使用精确绑定，根据字段标签分别从不同源绑定，避免冲突
			if err := bindParamPrecise(c, arg.Interface(), paramBind.ElemType, bodyBytes); err != nil {
				// 参数绑定失败，返回错误响应
				return c.JSON(http.StatusBadRequest, BaseHttpError{
					StatusCode: http.StatusBadRequest,
					Code:       "INVALID_PARAM",
					Message:    "参数绑定失败: " + err.Error(),
					RequestId:  requestId,
				}.GetResponse(requestId))
			}

			// 设置默认值（在绑定之后，这样默认值只会在字段为空时生效）
			if len(paramBind.DefaultFields) > 0 {
				fillParamWithDefaultOptimized(arg.Elem(), paramBind.DefaultFields)
			}

			if paramBind.IsPtr {
				invokeArgs = append(invokeArgs, arg)
			} else {
				invokeArgs = append(invokeArgs, arg.Elem())
			}
		}

		// 调用 handler
		results := handlerFunc.Call(invokeArgs)

		if len(results) > 0 && !results[0].IsNil() {
			c.Set("Response", results[0].Interface())
		}

		return nil
	}

	// 应用中间件（从后往前包装）
	handler := coreHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}

// buildWebSocketHandler 构建 WebSocket 处理器
func buildWebSocketHandler(route Route) echo.HandlerFunc {
	handlerFunc := route.Handler
	methodType := handlerFunc.Type()
	middlewares := route.Middlewares

	// 核心 WebSocket 处理器
	coreHandler := func(c echo.Context) error {
		// 首先检查是否是 WebSocket 升级请求，如果不是则返回错误
		if !websocket.IsWebSocketUpgrade(c.Request()) {
			return c.String(http.StatusBadRequest, "WebSocket upgrade required")
		}

		// 设置上下文参数
		if route.CtxParams != nil {
			for key, value := range route.CtxParams {
				c.Set(key, value)
			}
		}

		// 检测是否是 WebSocket 升级请求
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// 生产环境应该检查 Origin，这里为了演示允许所有来源
				// 实际使用时应该从 CORS 配置中获取允许的源
				return true
			},
		}

		// 升级连接为 WebSocket
		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		// 注意：不要在这里 defer conn.Close()
		// 连接的生命周期由 handler 函数管理

		// 检查 handler 参数签名
		// WebSocket handler 签名应该是：func(c GContext, conn *websocket.Conn) error
		numIn := methodType.NumIn()
		if numIn < 2 {
			conn.Close()
			return fmt.Errorf("WebSocket handler 至少需要 2 个参数: GContext 和 *websocket.Conn, 当前有 %d 个参数", numIn)
		}

		// 准备调用参数
		args := make([]reflect.Value, numIn)
		args[0] = reflect.ValueOf(c) // GContext

		// 第二个参数应该是 *websocket.Conn（指针类型）
		connType := methodType.In(1)
		expectedConnType := reflect.TypeOf((*websocket.Conn)(nil))
		if connType != expectedConnType {
			conn.Close()
			return fmt.Errorf("WebSocket handler 第二个参数必须是 *websocket.Conn, 当前是 %s, 期望是 %s", connType, expectedConnType)
		}
		args[1] = reflect.ValueOf(conn)

		// 如果还有其他参数（可选），需要从 query 或其他地方绑定
		// 这里简化处理，只支持 GContext 和 *websocket.Conn
		if numIn > 2 {
			slog.Warn("WebSocket handler 有额外参数，将被忽略", "handler", methodType.Name(), "params", numIn)
		}

		// 调用 handler（handler 函数是阻塞的，会一直运行直到连接关闭）
		// handler 内部应该管理连接的完整生命周期
		// 注意：WebSocket 升级后，连接已经被 hijacked，不能再通过 Echo 的 Response 写入
		results := handlerFunc.Call(args)

		// 处理返回值（如果有错误）
		// 注意：WebSocket 连接已被 hijacked，不能返回 HTTP 错误响应
		// 如果 handler 返回错误，连接可能已经关闭或需要关闭
		if len(results) > 0 {
			if errVal := results[0]; !errVal.IsNil() {
				if err, ok := errVal.Interface().(error); ok && err != nil {
					// handler 返回错误时，尝试关闭连接（Close 是幂等的）
					// 但连接可能已经被 handler 关闭了
					// 注意：不能通过 c.JSON 或 c.String 返回错误，因为连接已被 hijacked
					conn.Close()
					// WebSocket 连接已被 hijacked，返回 nil 避免 Echo 尝试写入响应
					return nil
				}
			}
		}

		// handler 正常返回 nil 时，连接应该已经被 handler 内部关闭了
		// 为了安全，再次关闭（Close 是幂等的，不会重复关闭已关闭的连接）
		conn.Close()
		// WebSocket 连接已被 hijacked，返回 nil 避免 Echo 尝试写入响应
		return nil
	}

	// 应用中间件（从后往前包装）
	handler := coreHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}
