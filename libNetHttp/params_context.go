package libNetHttp

import (
	"context"
	"net/http"
)

type paramsContextKey string

const urlParamsKey paramsContextKey = "nethttp.urlparams"

func WithURLParams(r *http.Request, params map[string]string) *http.Request {
	if len(params) == 0 {
		return r
	}
	ctx := r.Context()
	return r.WithContext(contextWithURLParams(ctx, params))
}

func URLParamsFromRequest(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}
	return urlParamsFromContext(r.Context())
}

func contextWithURLParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, urlParamsKey, params)
}

func urlParamsFromContext(ctx context.Context) map[string]string {
	params, ok := ctx.Value(urlParamsKey).(map[string]string)
	if !ok {
		return nil
	}
	return params
}
