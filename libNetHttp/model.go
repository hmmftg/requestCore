package libNetHttp

import "net/http"

type NetHttpParser struct {
	Request  *http.Request
	Response http.ResponseWriter
	Locals   map[string]any
	Params   map[string]string
}
