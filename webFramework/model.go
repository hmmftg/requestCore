package webFramework

import (
	"context"
	"net/http"
)

type RecordData interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	SetId(string)
	SetValue(string)
	GetSubCategory() string
	GetValue() any
	GetValueMap() map[string]string
}

type FieldParser interface {
	Parse(string) string
}

type RequestParser interface {
	GetMethod() string
	GetPath() string
	GetHeader(target any) error
	GetHeaderValue(name string) string
	GetHttpHeader() http.Header
	GetBody(target any) error
	GetUri(target any) error
	GetUrlQuery(target any) error
	GetRawUrlQuery() string
	GetLocal(name string) any
	GetLocalString(name string) string
	GetUrlParam(name string) string
	GetUrlParams() map[string]string
	CheckUrlParam(name string) (string, bool)
	SetLocal(name string, value any)
	SetReqHeader(name string, value string)
	GetArgs(args ...any) map[string]string
	ParseCommand(command, title string, request RecordData, parser FieldParser) string
	SendJSONRespBody(status int, resp any) error
	Next() error
	Abort() error
}

type RequestHandler interface {
	Respond(code, status int, message string, data any, abort bool)
	HandleErrorState(err error, status int, message string, data any)
}

type WebFramework struct {
	Ctx context.Context
	//Handler response.ResponseHandler
	Parser RequestParser
}
