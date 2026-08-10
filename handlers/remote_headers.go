package handlers

import (
	"os"
	"strconv"

	"github.com/hmmftg/requestCore/webFramework"
)

// RequestIDLocalKey is the per-request local-storage key for the request ID
// used for cross-service correlation. Set it during request initialization
// via w.Parser.SetLocal(RequestIDLocalKey, requestID).
const RequestIDLocalKey = "request_id"

// RequestIDHeader is the HTTP header name used to propagate the request ID
// to downstream services.
const RequestIDHeader = "X-Request-ID"

// CorrelationIDLocalKey is the per-request local-storage key for the
// correlation ID, an alternative cross-service correlation identifier to the
// request ID. Set it during request initialization via
// w.Parser.SetLocal(CorrelationIDLocalKey, correlationID).
const CorrelationIDLocalKey = "correlation_id"

// CorrelationIDHeader is the HTTP header name used to propagate the
// correlation ID to downstream services.
const CorrelationIDHeader = "X-Correlation-ID"

// BuildBaseRemoteHeaders returns a fresh map of base headers for an outbound
// remote API call. It sets:
//   - Accept: application/json
//   - X-App-ID: <appName>-<pid>
//   - X-Request-ID: <requestID> (when present in the parser's local storage)
//   - X-Correlation-ID: <correlationID> (from the argument when non-empty,
//     otherwise from CorrelationIDLocalKey in parser locals when present)
//
// The correlationID argument takes precedence over the parser local. It does
// not add authorization; RemoteApi.EnsureAuthorization remains responsible for
// that. Each invocation returns a new map; callers can safely mutate the result.
func BuildBaseRemoteHeaders(w webFramework.WebFramework, appName, correlationID string) map[string]string {
	headers := map[string]string{
		"Accept":   "application/json",
		"X-App-ID": appName + "-" + strconv.Itoa(os.Getpid()),
	}
	if w.Parser != nil {
		if rid, ok := w.Parser.GetLocal(RequestIDLocalKey).(string); ok && rid != "" {
			headers[RequestIDHeader] = rid
		}
		cid := correlationID
		if cid == "" {
			if local, ok := w.Parser.GetLocal(CorrelationIDLocalKey).(string); ok {
				cid = local
			}
		}
		if cid != "" {
			headers[CorrelationIDHeader] = cid
		}
	}
	return headers
}
