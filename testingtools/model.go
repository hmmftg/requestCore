package testingtools

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"image"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/image/font/opentype"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
)

type customMockConverter struct{}

type Model struct {
	Query   string
	Columns []string
	Values  [][]any
	Args    []any
	Err     error
}

type TestOptions struct {
	Path            string
	Name            string
	Method          string
	Handler         any
	Middleware      gin.HandlerFunc
	MiddlewareFiber fiber.Handler
	Silent          bool
}

type TestCase struct {
	Name         string
	Url          string
	Header       Header
	Model        libQuery.QueryRunnerModel
	Request      any
	Status       int
	CheckBody    []string
	CheckHeader  map[string]string
	DontReadBody bool
	MockError    error
	Desired      any
	DesiredError string
	DesiredResp  string
}

type KeyValuePair struct {
	Key   string
	Value string
}

type Header []KeyValuePair
type AnyString string

const (
	oracleSign string = ":"
	pgSign     string = "$"
)

func (customMockConverter) ConvertValue(v interface{}) (driver.Value, error) {
	switch value := v.(type) {
	case string:
		return value, nil
	case int64:
		return value, nil
	case sql.Out:
		return value, nil
	default:
		return nil, fmt.Errorf("cannot convert %T with value %v", v, v)
	}
}

// Map converts a Header to a map.
func (h Header) Map() map[string][]string {
	headerMap := map[string][]string{
		"Request-Id": {"0123456789"},
		"Branch-Id":  {"12345"},
		"Person-Id":  {"123456"},
		"User-Id":    {"testuser"},
	}

	for _, m := range h {
		headerMap[m.Key] = []string{m.Value}
	}

	return headerMap
}

// setHeaders sets given headers,
// if there are no given headers it set's some default headers.
func (header Header) setHeaders(r *http.Request) {
	r.Header.Add("Request-Id", "0123456789")
	r.Header.Add("Branch-Id", "12345")
	r.Header.Add("Person-Id", "123456")
	r.Header.Add("User-Id", "testuser")
	for _, h := range header {
		r.Header.Add(h.Key, h.Value)
	}
}

type TestingWsParams struct {
	RemoteApis  map[string]libCallApi.RemoteApi `yaml:"remoteApis"`
	ErrorDesc   map[string]string               `yaml:"errorDesc"`
	MessageDesc map[string]string               `yaml:"messageDesc"`
	AccessRoles map[string]string               `yaml:"accessRoles"`
}

func (p *TestingWsParams) GetFonts() map[string]opentype.Font {
	return nil
}
func (p *TestingWsParams) GetRoles() map[string]string {
	return nil
}
func (p *TestingWsParams) GetParams() map[string]string {
	return nil
}

func (p *TestingWsParams) GetImages() map[string]image.Image {
	return nil
}
func (p *TestingWsParams) GetLogPath() string {
	return "test.log"
}
func (p *TestingWsParams) GetLogSize() int {
	return 1
}
func (p *TestingWsParams) GetLogCompress() bool {
	return false
}
func (p *TestingWsParams) GetSkipPaths() []string {
	return nil
}
func (p *TestingWsParams) GetHeaderName() string {
	return "test"
}

type TestEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

func (env TestEnv) GetInterface() requestCore.RequestCoreInterface {
	return env.Interface
}
func (env TestEnv) GetParams() libParams.ParamInterface {
	return env.Params
}
func (env *TestEnv) SetInterface(core requestCore.RequestCoreInterface) {
	env.Interface = core
}
func (env *TestEnv) SetParams(params libParams.ParamInterface) {
	env.Params = params
}
