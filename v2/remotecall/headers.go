package remotecall

import (
	"context"
	"net/http"
	"os"
	"strconv"
)

// RequestIDHeader is the HTTP header name used to propagate the request ID
// to downstream services.
const RequestIDHeader = "X-Request-ID"

// CorrelationIDHeader is the HTTP header name used to propagate the
// correlation ID to downstream services.
const CorrelationIDHeader = "X-Correlation-ID"

// HeaderContext carries request-scoped values for header building.
type HeaderContext struct {
	AppName       string
	RequestID     string
	CorrelationID string
}

// BuildBaseHeaders returns a fresh http.Header with base headers for an
// outbound remote API call. It sets:
//   - Accept: application/json
//   - X-App-ID: <appName>-<pid>
//   - X-Request-ID: <requestID> (when non-empty)
//   - X-Correlation-ID: <correlationID> (when non-empty)
//
// Each invocation returns a new http.Header; callers can safely mutate the
// result. This is the v2 replacement for handlers.BuildBaseRemoteHeaders.
func BuildBaseHeaders(ctx HeaderContext) http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json")
	h.Set("X-App-ID", ctx.AppName+"-"+strconv.Itoa(os.Getpid()))
	if ctx.RequestID != "" {
		h.Set(RequestIDHeader, ctx.RequestID)
	}
	if ctx.CorrelationID != "" {
		h.Set(CorrelationIDHeader, ctx.CorrelationID)
	}
	return h
}

// mergeHeaders merges extra headers into base, returning a new http.Header.
// Extra headers override base headers for the same key.
func mergeHeaders(base, extra http.Header) http.Header {
	result := base.Clone()
	for k, vs := range extra {
		result[k] = vs
	}
	return result
}

// buildCallHeaders constructs the final headers for a call by merging
// client default headers, base headers, extra call headers, and applying auth.
func buildCallHeaders(
	ctx context.Context,
	defaultHeaders http.Header,
	hdrCtx HeaderContext,
	extra http.Header,
	auth AuthProvider,
) (http.Header, error) {
	base := BuildBaseHeaders(hdrCtx)
	if len(defaultHeaders) > 0 {
		base = mergeHeaders(base, defaultHeaders)
	}
	if len(extra) > 0 {
		base = mergeHeaders(base, extra)
	}
	if auth != nil {
		if err := auth.Apply(ctx, base); err != nil {
			return nil, err
		}
	}
	return base, nil
}
