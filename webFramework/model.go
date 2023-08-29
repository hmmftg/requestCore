package webFramework

import (
	"net/http"

	"github.com/hmmftg/requestCore/libQuery"
)

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
	GetArgs(args ...any) map[string]string
	ParseCommand(command, title string, request libQuery.RecordData, parser libQuery.FieldParser) string
}

type RequestHandler interface {
	Respond(code, status int, message string, data any, abort bool)
	HandleErrorState(err error, status int, message string, data any)
}

type WebFramework struct {
	Ctx any
	//Handler response.ResponseHandler
	Parser RequestParser
}
