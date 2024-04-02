// nolint:,staticcheck,ineffassign
package webFramework

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

type FakeParser struct {
	Method      string
	Path        string
	Header      HeaderInterface
	HttpHeader  http.Header
	ReqHeader   map[string]string
	Body        any
	Uri         any
	UrlQuery    any
	RawUrlQuery string
	Locals      map[string]any
	Args        map[string]string
	Urlparams   map[string]string
	JsonResp    any
}

func (f FakeParser) GetMethod() string {
	return f.Method
}
func (f FakeParser) GetPath() string {
	return f.Path
}
func (f FakeParser) GetHeader(target HeaderInterface) error {
	return nil
}
func (f FakeParser) GetHeaderValue(name string) string {
	return f.ReqHeader[name]
}
func (f FakeParser) GetHttpHeader() http.Header {
	return f.HttpHeader
}
func (f FakeParser) GetBody(target any) error {
	target = f.Body
	return nil
}
func (f FakeParser) GetUri(target any) error {
	target = f.Uri
	return nil
}
func (f FakeParser) GetUrlQuery(target any) error {
	target = f.UrlQuery
	return nil
}
func (f FakeParser) GetRawUrlQuery() string {
	return f.RawUrlQuery
}
func (f FakeParser) GetLocal(name string) any {
	return f.Locals[name]
}
func (f FakeParser) GetLocalString(name string) string {
	return fmt.Sprintf("%v", f.Locals[name])
}
func (f FakeParser) GetUrlParam(name string) string {
	return f.Urlparams[name]
}
func (f FakeParser) GetUrlParams() map[string]string {
	return f.Urlparams
}
func (f FakeParser) CheckUrlParam(name string) (string, bool) {
	p, ok := f.Urlparams[name]
	return p, ok
}
func (f FakeParser) SetLocal(name string, value any) {
	value = f.Locals[name]
}
func (f FakeParser) SetReqHeader(name string, value string) {
	f.ReqHeader[name] = value
}
func (f FakeParser) GetArgs(args ...any) map[string]string {
	return f.Args
}
func (f FakeParser) ParseCommand(command, title string, request RecordData, parser FieldParser) string {
	return ""
}
func (f FakeParser) SendJSONRespBody(status int, resp any) error {
	resp = f.JsonResp
	return nil
}
func (f FakeParser) Next() error {
	return nil
}
func (f FakeParser) Abort() error {
	return nil
}

func (c FakeParser) FormFile(name string) (multipart.File, *multipart.FileHeader, error) {
	f, headers, err := c.FormFile(name)

	return f, headers, err
}

func (c FakeParser) FormValue(name string) string {
	value := c.FormValue(name)

	return value
}
