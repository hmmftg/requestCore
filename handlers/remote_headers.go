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

// BuildBaseRemoteHeaders returns a fresh map of base headers for an outbound
// remote API call. It sets:
//   - Accept: application/json
//   - X-App-ID: <appName>-<pid>
//   - X-Request-ID: <requestID> (when present in the parser's local storage)
//
// It does not add authorization; RemoteApi.EnsureAuthorization remains
// responsible for that. Each invocation returns a new map; callers can safely
// mutate the result.
func BuildBaseRemoteHeaders(w webFramework.WebFramework, appName string) map[string]string {
	headers := map[string]string{
		"Accept":   "application/json",
		"X-App-ID": appName + "-" + strconv.Itoa(os.Getpid()),
	}
	if w.Parser != nil {
		if rid, ok := w.Parser.GetLocal(RequestIDLocalKey).(string); ok && rid != "" {
			headers[RequestIDHeader] = rid
		}
	}
	return headers
}
