// Package libChi provides a Chi web framework adapter for requestCore.
package libChi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hmmftg/requestCore/libNetHttp"
)

// ExtractURLParams extracts Chi URL route parameters from the request into a map.
func ExtractURLParams(r *http.Request) map[string]string {
	routeCtx := chi.RouteContext(r.Context())
	if routeCtx == nil || len(routeCtx.URLParams.Keys) == 0 {
		return map[string]string{}
	}

	params := make(map[string]string, len(routeCtx.URLParams.Keys))
	for i := range routeCtx.URLParams.Keys {
		params[routeCtx.URLParams.Keys[i]] = routeCtx.URLParams.Values[i]
	}
	return params
}

// BindURLParams binds Chi URL route parameters to the given parser.
func BindURLParams(r *http.Request, parser *libNetHttp.NetHttpParser) {
	if parser == nil {
		return
	}
	for key, value := range ExtractURLParams(r) {
		parser.AddParam(key, value)
	}
}

// InitParser creates a NetHttpParser for the request and binds Chi URL parameters to it.
func InitParser(r *http.Request, w http.ResponseWriter) *libNetHttp.NetHttpParser {
	parser := libNetHttp.InitContext(r, w)
	BindURLParams(r, parser)
	return parser
}

// ParamsMiddleware is HTTP middleware that injects Chi URL parameters into the request context.
func ParamsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, libNetHttp.WithURLParams(r, ExtractURLParams(r)))
	})
}
