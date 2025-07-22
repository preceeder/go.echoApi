package echoApi

import "reflect"

type HttpResponse interface {
	GetResponse() any
	GetCode() int
}

var HttpResponseType = reflect.TypeOf((*HttpResponse)(nil)).Elem()

type BaseHttpResponse struct {
	Success   bool      `json:"success"`
	HttpCode  int       `json:"-"` // 默认情况下 http_code 和code 一致
	Code      int       `json:"code"`
	ErrorCode ErrorCode `json:"errorCode"`
	Data      any       `json:"data"`
	Message   string    `json:"message"`
}

func (h BaseHttpResponse) GetResponse() any {
	if h.Code == 0 {
		return map[string]any{"success": h.Success, "code": 200, "errorCode": 0, "data": h.Data, "message": h.Message}
	}
	return map[string]any{"success": h.Success, "code": h.Code, "errorCode": 0, "data": h.Data, "message": h.Message}
}

func (h BaseHttpResponse) GetCode() int {
	if h.HttpCode > 0 {
		return h.HttpCode
	}
	if h.Code == 0 {
		return 200
	}
	return h.Code
}

type HttpError interface {
	GetCode() int // 正常情况都是 200， 错误情况一般是  403
	GetResponse() any
	Error() string
}

var HttpErrorType = reflect.TypeOf((*HttpError)(nil)).Elem()

type BaseHttpError struct {
	Success   bool       `json:"success"`
	HttpCode  int        `json:"-"` // 默认情况下 http_code 和 code一致
	Code      StatusCode `json:"code"`
	ErrorCode ErrorCode  `json:"errorCode"`
	Message   string     `json:"message"`
}

func (h BaseHttpError) GetResponse() any {
	return map[string]any{"success": false, "errorCode": h.ErrorCode, "message": h.Message}
}

func (h BaseHttpError) Error() string {
	return h.Message
}

func (h BaseHttpError) GetCode() int {
	if h.HttpCode > 0 {
		return h.HttpCode
	}
	if h.Code == 0 {
		return 403
	}
	return int(h.Code)
}
