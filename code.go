package echoApi

type ErrorCode int

const (
	CodeApiTimeOutError   ErrorCode = 9999  // 接口超时
	CodeSystemError       ErrorCode = 10000 // 系统错误
	CodeParameterError    ErrorCode = 10001 // 参数错误
	CodeRequestFrequently ErrorCode = 10002 // 请求超限
	CodeLoginError        ErrorCode = 10003 // 登录失败
	CodeLoinForbid        ErrorCode = 10004 // 禁止登录
	CodeProhibit          ErrorCode = 10005 // 禁止操作 operation
	CodeTokenError        ErrorCode = 1001  // token验证失败
)

var CodeMessage = map[ErrorCode]string{
	CodeSystemError:       "System error",
	CodeParameterError:    "Invalid parameter",
	CodeRequestFrequently: "Frequent requests",
	CodeLoginError:        "Login failed",
	CodeLoinForbid:        "Prohibit login",
	CodeProhibit:          "Prohibit operation",
}

type StatusCode int

const (
	StatusCodeSystemError StatusCode = 500 // 系统错误 代码
	StatusCodeLoginError  StatusCode = 400 // 登录失败
	StatusCodeTokenError  StatusCode = 401 // token 验证失败代码
	StatusCodeCommonErr   StatusCode = 403 // 通用错误代码
	StatusCodeSuccess     StatusCode = 200 // 成功代码
)
