package libNetHttp

import "net/http"

// NetHttpParser implements the webFramework.RequestParser interface for net/http.
type NetHttpParser struct {
	Request  *http.Request
	Response http.ResponseWriter
	Locals   map[string]any
	Params   map[string]string
}
