package libContext

import (
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/webFramework"
)

type TestingParser struct {
	Root                     *testing.T
	Method, Path, RawQuery   string
	Header                   webFramework.HeaderInterface
	HeaderError              error
	Uri                      any
	UriError                 error
	HttpHeader               http.Header
	Body, UrlQuery           any
	BodyError, UrlQueryError error
	Headers                  map[string]any
	Locals                   map[string]any
	UrlParams                map[string]string
	Args                     map[string]string
	NextError                error
	AbortError               error
	SendError                error
	ParsedCommands           map[string]string
}

const (
	HeaderEnvKey = "h"
	LocalEnvKey  = "l"
)

func parseEnv(t *testing.T, key string) map[string]any {
	rawEnv := os.Getenv(key)
	valueList := strings.Split(rawEnv, "@")
	result := make(map[string]any, 0)
	for _, h := range valueList {
		pair := strings.Split(h, "#")
		if len(pair) == 1 {
			t.Fatalf("bad environment: %s\n", pair)
		}
		result[pair[0]] = pair[1]
	}
	return result
}

func initTestContext(t *testing.T) TestingParser {
	headers := parseEnv(t, HeaderEnvKey)
	locals := parseEnv(t, LocalEnvKey)
	return TestingParser{
		Root:    t,
		Method:  os.Getenv("m"),
		Headers: headers,
		Locals:  locals,
	}
}

func (t TestingParser) GetMethod() string {
	return t.Method
}
func (t TestingParser) GetPath() string {
	return t.Path
}

func setTarget(target any, value any) {
	targetPtr := reflect.ValueOf(target)
	targetPtr.Set(reflect.ValueOf(value))
}
func (t TestingParser) GetHeader(target webFramework.HeaderInterface) error {
	setTarget(target, t.Header)
	return t.HeaderError
}
func (t TestingParser) GetHeaderValue(name string) string {
	head, ok := t.Headers[name].(string)
	if !ok {
		t.Root.Fatalf("wrong header[%s] type:%T\n", name, t.Headers[name])
	}
	return head
}
func (t TestingParser) GetHttpHeader() http.Header {
	return t.HttpHeader
}
func (t TestingParser) GetBody(target any) error {
	setTarget(target, t.Body)
	return t.BodyError
}
func (t TestingParser) GetUri(target any) error {
	setTarget(target, t.Uri)
	return t.UriError
}
func (t TestingParser) GetUrlQuery(target any) error {
	setTarget(target, t.UrlQuery)
	return t.UrlQueryError
}
func (t TestingParser) GetRawUrlQuery() string {
	return t.RawQuery
}
func (t TestingParser) GetLocal(name string) any {
	return t.Locals[name]
}
func (t TestingParser) GetLocalString(name string) string {
	loc, ok := t.Locals[name].(string)
	if !ok {
		t.Root.Fatalf("wrong local[%s] type:%T\n", name, t.Locals[name])
	}
	return loc
}
func (t TestingParser) GetUrlParam(name string) string {
	return t.UrlParams[name]
}
func (t TestingParser) GetUrlParams() map[string]string {
	return t.UrlParams
}
func (t TestingParser) CheckUrlParam(name string) (string, bool) {
	param, ok := t.UrlParams[name]
	return param, ok
}
func (t TestingParser) SetLocal(name string, value any) {
	t.Locals[name] = value
}
func (t TestingParser) SetReqHeader(name string, value string) {
	t.Headers[name] = value
}
func (t TestingParser) GetArgs(args ...any) map[string]string {
	return t.Args
}
func (t TestingParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	return t.ParsedCommands[command]
}
func (t TestingParser) SendJSONRespBody(status int, resp any) error {
	return t.SendError
}
func (t TestingParser) Next() error {
	return t.NextError
}
func (t TestingParser) Abort() error {
	return t.AbortError
}

func (c TestingParser) FormValue(name string) string {
	value := c.FormValue(name)

	return value
}

func (c TestingParser) SaveFile(
	formTagName, path string,
) error {
	fileErr := c.SaveFile(formTagName, path)
	if fileErr != nil {
		return fileErr
	}

	return nil
}

func (c TestingParser) FileAttachment(path, fileName string) {
	c.FileAttachment(path, fileName)
}
