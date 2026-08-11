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
	"github.com/hmmftg/requestCore/libRequest"
)

// CustomMockConverter is a sqlmock value converter that handles common Go types.
type CustomMockConverter struct{}

// Model holds the query, columns, values, and error for a mock database interaction.
type Model struct {
	Query   string
	Columns []string
	Values  [][]any
	Args    []any
	Err     error
}

// TestOptions configures the test server including path, method, handler, and middleware.
type TestOptions struct {
	Path            string
	Name            string
	Method          string
	Handler         any
	Middleware      gin.HandlerFunc
	MiddlewareFiber fiber.Handler
	Silent          bool
}

// TestCase defines a single test case including URL, headers, expected status, and body checks.
type TestCase struct {
	Name           string
	URL            string
	Header         Header
	Model          libQuery.QueryRunnerModel
	Request        any
	Status         int
	CheckBody      []string
	CheckNotInBody []string
	CheckHeader    map[string]string
	DontReadBody   bool
	MockError      error
	Desired        any
	DesiredError   string
	DesiredResp    string
}

// KeyValuePair holds a single key-value pair used for test headers.
type KeyValuePair struct {
	Key   string
	Value string
}

// Header is a slice of KeyValuePair used to set HTTP headers in test requests.
type Header []KeyValuePair

// AnyString is a sqlmock argument type that matches any driver value.
type AnyString string

const (
	oracleSign string = ":"
	pgSign     string = "$"
)

// ConvertValue converts a Go value into a driver.Value for the mock database.
func (CustomMockConverter) ConvertValue(v interface{}) (driver.Value, error) {
	switch value := v.(type) {
	case string:
		return value, nil
	case int64:
		return value, nil
	case float64:
		return value, nil
	case sql.Out:
		return value, nil
	default:
		return nil, fmt.Errorf("cannot convert %T with value %v", v, v)
	}
}

// Map converts a Header to a map with dynamic header support
func (h Header) Map() map[string][]string {
	headerMap := map[string][]string{
		"Request-Id": {"0123456789"},
		"User-Id":    {"testuser"},
	}

	// Add dynamic headers from configuration
	if config := libRequest.GetGlobalHeaderConfig(); config != nil {
		// Add optional headers with default values
		for _, headerConfig := range config.OptionalHeaders {
			if headerConfig.DefaultValue != "" {
				headerMap[headerConfig.HeaderName] = []string{headerConfig.DefaultValue}
			}
		}

		// Add custom headers with default values
		for _, headerConfig := range config.CustomHeaders {
			if headerConfig.DefaultValue != "" {
				headerMap[headerConfig.HeaderName] = []string{headerConfig.DefaultValue}
			}
		}
	}

	// Override with explicitly set headers
	for _, m := range h {
		headerMap[m.Key] = []string{m.Value}
	}

	return headerMap
}

// setHeaders sets given headers with dynamic header support
func (h Header) setHeaders(r *http.Request) {
	// Set core headers
	r.Header.Add("Request-Id", "0123456789")
	r.Header.Add("User-Id", "testuser")

	// Add dynamic headers from configuration
	if config := libRequest.GetGlobalHeaderConfig(); config != nil {
		// Add optional headers with default values
		for _, headerConfig := range config.OptionalHeaders {
			if headerConfig.DefaultValue != "" {
				r.Header.Add(headerConfig.HeaderName, headerConfig.DefaultValue)
			}
		}

		// Add custom headers with default values
		for _, headerConfig := range config.CustomHeaders {
			if headerConfig.DefaultValue != "" {
				r.Header.Add(headerConfig.HeaderName, headerConfig.DefaultValue)
			}
		}
	}

	// Override with explicitly set headers
	for _, h := range h {
		r.Header.Add(h.Key, h.Value)
	}
}

// TestingWsParams holds remote API, error description, message description, and access role maps for testing.
type TestingWsParams struct {
	RemoteAPIs  map[string]libCallApi.RemoteAPI `yaml:"remoteApis"`
	ErrorDesc   map[string]string               `yaml:"errorDesc"`
	MessageDesc map[string]string               `yaml:"messageDesc"`
	AccessRoles map[string]string               `yaml:"accessRoles"`
}

// GetFonts returns the font map for the test environment (always nil in tests).
func (p *TestingWsParams) GetFonts() map[string]opentype.Font {
	return nil
}

// GetRoles returns the access role map for the test environment (always nil in tests).
func (p *TestingWsParams) GetRoles() map[string]string {
	return nil
}

// GetParams returns the parameter map for the test environment (always nil in tests).
func (p *TestingWsParams) GetParams() map[string]string {
	return nil
}

// GetImages returns the image map for the test environment (always nil in tests).
func (p *TestingWsParams) GetImages() map[string]image.Image {
	return nil
}

// GetLogPath returns the log file path for the test environment.
func (p *TestingWsParams) GetLogPath() string {
	return "test.log"
}

// GetLogSize returns the log rotation size in megabytes for the test environment.
func (p *TestingWsParams) GetLogSize() int {
	return 1
}

// GetLogCompress returns whether log files should be compressed in the test environment.
func (p *TestingWsParams) GetLogCompress() bool {
	return false
}

// GetSkipPaths returns the paths to skip in access logging for the test environment.
func (p *TestingWsParams) GetSkipPaths() []string {
	return nil
}

// GetHeaderName returns the header name used for request identification in tests.
func (p *TestingWsParams) GetHeaderName() string {
	return "test"
}

// TestEnv holds the parameter and request-core interfaces for a test environment.
type TestEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

// GetInterface returns the request-core interface for the test environment.
func (env TestEnv) GetInterface() requestCore.RequestCoreInterface {
	return env.Interface
}

// GetParams returns the parameter interface for the test environment.
func (env TestEnv) GetParams() libParams.ParamInterface {
	return env.Params
}

// SetInterface sets the request-core interface for the test environment.
func (env *TestEnv) SetInterface(core requestCore.RequestCoreInterface) {
	env.Interface = core
}

// SetParams sets the parameter interface for the test environment.
func (env *TestEnv) SetParams(params libParams.ParamInterface) {
	env.Params = params
}
