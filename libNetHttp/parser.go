package libNetHttp

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"
)

func InitContext(r *http.Request, w http.ResponseWriter) NetHttpParser {
	return NetHttpParser{
		Request:  r,
		Response: w,
		Locals:   make(map[string]any),
		Params:   make(map[string]string),
	}
}

func (c NetHttpParser) GetMethod() string {
	return c.Request.Method
}

func (c NetHttpParser) GetPath() string {
	return c.Request.URL.Path
}

func (c NetHttpParser) GetHeader(target webFramework.HeaderInterface) error {
	// Parse headers into target struct
	// This is a simplified implementation - you might want to use a more sophisticated header parsing
	// For now, we'll manually map common headers
	if target == nil {
		return nil
	}

	// Example implementation - you can expand this based on your HeaderInterface
	if user := c.Request.Header.Get("User-Id"); user != "" {
		target.SetUser(user)
	}
	if program := c.Request.Header.Get("Program"); program != "" {
		target.SetProgram(program)
	}
	if module := c.Request.Header.Get("Module"); module != "" {
		target.SetModule(module)
	}
	if method := c.Request.Header.Get("Method"); method != "" {
		target.SetMethod(method)
	}

	return nil
}

func (c NetHttpParser) GetHeaderValue(name string) string {
	return c.Request.Header.Get(name)
}

func (c NetHttpParser) GetHttpHeader() http.Header {
	return c.Request.Header
}

func (c NetHttpParser) GetBody(target any) error {
	if c.Request.Body == nil {
		return nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	if len(body) == 0 {
		return nil
	}

	return json.Unmarshal(body, target)
}

func (c NetHttpParser) GetUri(target any) error {
	// Parse URL parameters into target struct
	// This is a simplified implementation
	if target == nil {
		return nil
	}

	// You might want to use a more sophisticated parameter parsing library
	// For now, we'll use reflection or manual mapping
	return c.parseStructFromMap(target, c.Params)
}

func (c NetHttpParser) GetUrlQuery(target any) error {
	if target == nil {
		return nil
	}

	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	return c.parseStructFromMap(target, queryParams)
}

func (c NetHttpParser) GetRawUrlQuery() string {
	return c.Request.URL.RawQuery
}

func (c NetHttpParser) GetLocal(name string) any {
	return c.Locals[name]
}

func (c NetHttpParser) GetLocalString(name string) string {
	if value, exists := c.Locals[name]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

func (c NetHttpParser) GetUrlParam(name string) string {
	return c.Params[name]
}

func (c NetHttpParser) GetUrlParams() map[string]string {
	return c.Params
}

func (c NetHttpParser) CheckUrlParam(name string) (string, bool) {
	value, exists := c.Params[name]
	return value, exists
}

func (c NetHttpParser) SetLocal(name string, value any) {
	c.Locals[name] = value
}

func (c NetHttpParser) SetReqHeader(name string, value string) {
	c.Request.Header.Set(name, value)
}

func (c NetHttpParser) SetRespHeader(name string, value string) {
	c.Response.Header().Set(name, value)
}

func (c NetHttpParser) GetArgs(args ...any) map[string]string {
	netHttpArgs := map[string]string{
		"userId":   c.GetLocalString("userId"),
		"userName": c.GetLocalString("userName"),
		"appName":  c.GetLocalString("appName"),
		"action":   c.GetLocalString("action"),
		"bankCode": c.GetLocalString("bankCode"),
		"path":     c.Request.URL.Path,
	}

	for _, arg := range args {
		if argStr, ok := arg.(string); ok {
			netHttpArgs[argStr] = c.Params[argStr]
		}
	}

	return netHttpArgs
}

func (c NetHttpParser) ParseCommand(command, title string, request webFramework.RecordData, parser webFramework.FieldParser) string {
	if request.GetValueMap() == nil {
		return libQuery.ParseCommand(command, c.GetLocalString("userId"),
			c.GetLocalString("appName"),
			c.GetLocalString("action"),
			title,
			map[string]string{}, parser)
	}
	return libQuery.ParseCommand(command, c.GetLocalString("userId"),
		c.GetLocalString("appName"),
		c.GetLocalString("action"),
		title,
		request.GetValueMap(), parser)
}

func (c NetHttpParser) SendJSONRespBody(status int, resp any) error {
	c.Response.Header().Set("Content-Type", "application/json")
	c.Response.WriteHeader(status)

	if resp == nil {
		return nil
	}

	return json.NewEncoder(c.Response).Encode(resp)
}

func (c NetHttpParser) Next() error {
	// For net/http, we don't have a built-in Next() concept like middleware chains
	// This could be implemented using a custom middleware pattern
	return nil
}

func (c NetHttpParser) Abort() error {
	// For net/http, we can't really "abort" in the same way as Gin/Fiber
	// We can set a status and return early
	c.Response.WriteHeader(http.StatusInternalServerError)
	return nil
}

func (c NetHttpParser) FormValue(name string) string {
	return c.Request.FormValue(name)
}

func (c NetHttpParser) SaveFile(formTagName, path string) error {
	file, _, err := c.Request.FormFile(formTagName)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create the file
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the uploaded file to the destination
	_, err = io.Copy(dst, file)
	if err != nil {
		return err
	}

	return nil
}

func (c NetHttpParser) FileAttachment(path, fileName string) {
	c.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	http.ServeFile(c.Response, c.Request, path)
}

func (c NetHttpParser) AddCustomAttributes(attr slog.Attr) {
	// For net/http, we can store custom attributes in locals
	// This is a simplified implementation
	if c.Locals == nil {
		c.Locals = make(map[string]any)
	}
	c.Locals[attr.Key] = attr.Value
}

// Helper function to parse map into struct
func (c NetHttpParser) parseStructFromMap(target any, data map[string]string) error {
	// This is a simplified implementation
	// You might want to use a more sophisticated library like mapstructure
	// or implement reflection-based parsing

	// For now, we'll use JSON marshaling/unmarshaling as a workaround
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, target)
}

// SetParams sets URL parameters (useful for routing)
func (c *NetHttpParser) SetParams(params map[string]string) {
	c.Params = params
}

// AddParam adds a single URL parameter
func (c *NetHttpParser) AddParam(key, value string) {
	if c.Params == nil {
		c.Params = make(map[string]string)
	}
	c.Params[key] = value
}

// ParseForm parses form data
func (c NetHttpParser) ParseForm() error {
	return c.Request.ParseForm()
}

// ParseMultipartForm parses multipart form data
func (c NetHttpParser) ParseMultipartForm(maxMemory int64) error {
	return c.Request.ParseMultipartForm(maxMemory)
}

// GetFormValue gets form value
func (c NetHttpParser) GetFormValue(key string) string {
	return c.Request.FormValue(key)
}

// GetFormValues gets all form values for a key
func (c NetHttpParser) GetFormValues(key string) []string {
	return c.Request.Form[key]
}

// GetPostFormValue gets POST form value
func (c NetHttpParser) GetPostFormValue(key string) string {
	return c.Request.PostFormValue(key)
}

// GetPostFormValues gets all POST form values for a key
func (c NetHttpParser) GetPostFormValues(key string) []string {
	return c.Request.PostForm[key]
}

// GetCookie gets a cookie by name
func (c NetHttpParser) GetCookie(name string) (*http.Cookie, error) {
	return c.Request.Cookie(name)
}

// GetCookies gets all cookies
func (c NetHttpParser) GetCookies() []*http.Cookie {
	return c.Request.Cookies()
}

// SetCookie sets a cookie
func (c NetHttpParser) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response, cookie)
}

// Redirect redirects to a URL
func (c NetHttpParser) Redirect(url string, statusCode int) {
	http.Redirect(c.Response, c.Request, url, statusCode)
}

// ServeFile serves a file
func (c NetHttpParser) ServeFile(name string) {
	http.ServeFile(c.Response, c.Request, name)
}

// ServeContent serves content
func (c NetHttpParser) ServeContent(name string, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(c.Response, c.Request, name, modtime, content)
}
