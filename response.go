package echoApi

import "reflect"

type HttpResponse interface {
	GetResponse(string) any
	GetStatusCode() int
}

var HttpResponseType = reflect.TypeOf((*HttpResponse)(nil)).Elem()

type BaseHttpResponse struct {
	StatusCode int    `json:"-"` // 默认情况下 http_code 和code 一致
	RequestId  string `json:"requestId"`
	Data       any    `json:"data"`
}

func (h BaseHttpResponse) GetResponse(requestId string) any {
	if h.StatusCode == 0 {
		return map[string]any{"RequestId": requestId, "data": h.Data}
	}
	return map[string]any{"RequestId": requestId, "data": h.Data}
}

func (h BaseHttpResponse) GetStatusCode() int {
	if h.StatusCode > 0 {
		return h.StatusCode
	}
	return 200
}

type HttpError interface {
	GetStatusCode() int // 正常情况都是 200
	GetResponse(string) any
	Error() string
}

var HttpErrorType = reflect.TypeOf((*HttpError)(nil)).Elem()

type BaseHttpError struct {
	StatusCode int    `json:"-"` // 默认情况下 http_code 和 code一致
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestId  string `json:"requestId"`
}

func (h BaseHttpError) GetResponse(requestId string) any {
	return map[string]any{"requestId": requestId, "code": h.Code, "message": h.Message}
}

func (h BaseHttpError) Error() string {
	return h.Message
}

func (h BaseHttpError) GetStatusCode() int {
	if h.StatusCode > 0 {
		return h.StatusCode
	}
	return 500
}
