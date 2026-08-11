package libNetHttp

import "net/http"

// NetHTTPParser implements the webFramework.RequestParser interface for net/http.
type NetHTTPParser struct {
	Request  *http.Request
	Response http.ResponseWriter
	Locals   map[string]any
	Params   map[string]string
}
