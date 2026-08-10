package webFramework

import "time"

// TransactionLoggerLocalKey is the per-request local-storage key used to
// register a TransactionLogger for the duration of a request.
// Set it via w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger).
const TransactionLoggerLocalKey = "transaction_logger"

// TransactionInfo is the canonical generic payload describing a completed
// remote API call. It is used by TransactionLogger and CallApiOptions.OnComplete.
type TransactionInfo struct {
	ServiceName        string        // param.Api.Name
	URL                string        // full request URL (domain + "/" + path + query)
	Endpoint           string        // param.Path
	Method             string        // param.Method (HTTP method)
	StatusCode         int           // actual HTTP status (0 if request failed before getting a response)
	Duration           time.Duration // elapsed time
	Error              error         // nil on success
	Request            any           // param.JsonBody (caller may mask sensitive fields via CallApiOptions.MaskFunc)
	Response           any           // parsed response (nil on error; caller may mask via MaskFunc)
	ResponseBody       []byte        // raw response body from RemoteCallError on error, nil on success; preserved raw for diagnostics — may contain sensitive data
	MaskedResponseBody any           // MaskFunc(ResponseBody) on error paths when CallApiOptions.MaskFunc is set; nil when MaskFunc is nil or on success. Transaction loggers should emit this field by default and use ResponseBody only when the raw bytes are explicitly required.
}

// TransactionLogger is a framework-level interface for recording transaction
// logs after remote API calls. Register an implementation via
// w.Parser.SetLocal(TransactionLoggerLocalKey, logger) during request
// initialization. CallApiJSONWithOpts resolves and invokes it unconditionally
// on every code path (success, error, timeout) in addition to webFramework.AddLog.
type TransactionLogger interface {
	LogTransaction(info TransactionInfo)
}
