package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRetry"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type WsResponse[Result any] struct {
	HTTPStatus   int                      `json:"-"`
	HTTPHeaders  map[string]string        `json:"-"`
	Status       int                      `json:"status"`
	Description  string                   `json:"description"`
	Result       Result                   `json:"result,omitempty"`
	ErrorData    []response.ErrorResponse `json:"errors,omitempty"`
	PrintReceipt *response.Receipt        `json:"printReceipt,omitempty"`
}

const (
	CallAPILogEntry string = "ApiCall"
)

func (w *WsResponse[any]) SetStatus(status int) {
	w.HTTPStatus = status
}
func (w *WsResponse[any]) SetHeaders(headers map[string]string) {
	w.HTTPHeaders = headers
}

func CallAPIInternal[Resp any](
	w webFramework.WebFramework,
	_ requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(method, param))

	resp1 := libCallApi.Call[Resp](w, param)

	if resp1.Error != nil {
		webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-error", method), resp1.Error))
		if ok, err := response.Unwrap(resp1.Error); ok {
			return nil, response.Errors(http.StatusInternalServerError, "REMOTE_CALL_ERROR", param, err)
		}
		return nil, errors.Join(resp1.Error,
			libError.New(
				http.StatusInternalServerError,
				"REMOTE_CALL_ERROR",
				param,
			))
	}
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp1))
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState().Input(param).SetStatus(resp1.Status.Status)
	}
	return resp1.Resp, nil
}

func CallAPI[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := CallAPIInternal[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}

func CallAPIWithReceipt[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, *response.Receipt, error) {
	result, err := CallAPIInternal[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, nil, err
	}
	return &result.Result, result.PrintReceipt, err
}

func CallAPIJSON[Req any, Resp any](
	w webFramework.WebFramework,
	_ requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req, Resp],
) (Resp, error) {
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(method, param))

	param.BodyType = libCallApi.JSON
	if param.Parser == nil {
		param.Parser = w.Parser // Set parser for distributed tracing if not already set
	}
	resp, err := libCallApi.RemoteCall(w, param)
	if err != nil {
		webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-error", method), err))
		return *new(Resp), err
	}
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp))
	return *resp, nil
}

func CallAPIForm[Req any, Resp any](
	w webFramework.WebFramework,
	_ requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req, Resp],
) (Resp, error) {
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(method, param))

	param.BodyType = libCallApi.Form
	if param.Parser == nil {
		param.Parser = w.Parser // Set parser for distributed tracing if not already set
	}
	resp, err := libCallApi.RemoteCall(w, param)
	if err != nil {
		webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-error", method), err))
		return *new(Resp), err
	}
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp))
	return *resp, nil
}
func callAPINoLog[Resp any](
	w webFramework.WebFramework,
	_ string,
	param libCallApi.CallParam) (*Resp, error) {
	resp1 := libCallApi.Call[Resp](w, param)

	if resp1.Error != nil {
		if ok, err := response.Unwrap(resp1.Error); ok {
			return nil, response.Errors(http.StatusInternalServerError, "REMOTE_CALL_ERROR", param, err)
		}
		return nil, errors.Join(resp1.Error,
			libError.New(
				http.StatusInternalServerError, "REMOTE_CALL_ERROR",
				param,
			))
	}
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState().Input(param).SetStatus(resp1.Status.Status)
	}
	return resp1.Resp, nil
}

func CallAPINoLog[Resp any](
	w webFramework.WebFramework,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := callAPINoLog[WsResponse[Resp]](w, method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}

// APICallInfo is a compatibility alias for webFramework.TransactionInfo.
// New code should use webFramework.TransactionInfo directly.
type APICallInfo = webFramework.TransactionInfo

// CallAPILogKeys holds configurable log key templates for AddLog entries.
// When a field is empty, the corresponding default is used.
type CallAPILogKeys struct {
	Request  string // default: <Method>
	Response string // default: <Method>-resp
	Failure  string // default: <Method>-error
}

// CallAPIOptions holds optional parameters for CallAPIJSONWithOpts.
type CallAPIOptions struct {
	Method  string        // log key, e.g. "soha-authorize"
	Timeout time.Duration // server-side elapsed-time guard (0 = no guard)

	// LogKeys, when set, overrides the default AddLog key templates.
	// Empty fields fall back to the defaults.
	LogKeys CallAPILogKeys

	// MetricsRecorder, if set, records Prometheus-style metrics for the call.
	// If nil, the default global Prometheus recorder is used.
	MetricsRecorder libTracing.HTTPClientMetricsRecorder

	// OnComplete is an optional callback invoked after the remote call
	// finishes (success or failure). It receives the call metadata so the
	// application layer can record it in its own transaction logger, metrics,
	// or any other observability system. nil = no callback.
	OnComplete func(webFramework.TransactionInfo)

	// NormalizeError, when set, is applied to the returned error after
	// raw observability (AddLog, metrics, transaction logging, OnComplete)
	// has been recorded. This preserves raw Splunk logs while allowing
	// callers to normalize the error returned to their consumers.
	NormalizeError func(error) error

	// RetryPolicy, when set, enables retry behavior. The single-attempt
	// logic is reused for each attempt with full observability per attempt.
	// nil = no retries (single attempt, existing behavior).
	RetryPolicy *libRetry.RetryPolicy

	// TimeoutStatusCode selects the status stored in the returned timeout
	// libError when the server-side elapsed-time guard fires. Zero = default
	// (http.StatusRequestTimeout, 408). This affects ONLY the returned
	// libError's application status; TransactionInfo.StatusCode and metrics
	// status_class continue to represent the actual observed remote HTTP
	// status.
	TimeoutStatusCode int

	// MaskFunc, when set, is applied to Request, parsed Response, and
	// ResponseBody (the result is stored in MaskedResponseBody) before
	// TransactionLogger and OnComplete receive TransactionInfo. nil = no
	// masking (raw values passed through). Must NOT mutate the outbound
	// request or the typed response returned by CallAPIJSONWithOpts. Never
	// applied to webFramework.AddLog attrs (AddLog uses
	// RemoteCallParamData.LogValue which independently masks Authorization).
	MaskFunc func(any) any
}

// CallAPIJSONWithOpts is an enhanced version of CallAPIJSON that adds:
//   - Prometheus metrics recording via libTracing.RecordHTTPClientCallWithOutcome
//   - A server-side elapsed-time timeout guard (in addition to the HTTP client timeout)
//   - HTTP status code preservation via RemoteCallError (independently of the caller's builder)
//   - Framework-level transaction logging via webFramework.TransactionLogger
//   - An optional OnComplete callback for application-layer extension hooks
//   - Optional error normalization via NormalizeError
//   - Optional retry via RetryPolicy
//   - Configurable log keys via LogKeys
//
// webFramework.AddLog is called on every code path (request, error, response)
// and is never skipped or conditionally bypassed. Transaction logging runs
// after AddLog and never replaces it.
//
// Custom headers: set param.Headers directly (e.g. param.Headers["SIGNATURE"] = "...").
func CallAPIJSONWithOpts[Req any, Resp any](
	w webFramework.WebFramework,
	_ requestCore.RequestCoreInterface,
	param *libCallApi.RemoteCallParamData[Req, Resp],
	opts CallAPIOptions,
) (Resp, error) {
	param.BodyType = libCallApi.JSON
	if param.Parser == nil {
		param.Parser = w.Parser
	}

	// Resolve log keys
	reqKey, respKey, failKey := resolveLogKeys(opts)

	// Resolve QueryStack once before retry loop to pin the effective query
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		param.QueryStack = nil
	}

	// If no retry policy, execute a single attempt directly
	if opts.RetryPolicy == nil {
		resp, _, err := executeSingleAttempt(w, param, opts, reqKey, respKey, failKey)
		return finalizeResult(resp, err, opts)
	}

	// Retry path: use libRetry.WithRetry
	originalBuilder := param.Builder
	if originalBuilder == nil {
		originalBuilder = libCallApi.StatusPreservingBuilder[Resp]
	}

	retryResult := libRetry.WithRetry(opts.RetryPolicy, func(attempt int) (*Resp, int, error) {
		attemptReqKey := formatRetryKey(reqKey, attempt)
		attemptRespKey := formatRetryKey(respKey, attempt)
		attemptFailKey := formatRetryKey(failKey, attempt)

		// Clone mutable parts for this attempt
		attemptParam := *param
		if param.Headers != nil {
			attemptParam.Headers = make(map[string]string, len(param.Headers))
			for k, v := range param.Headers {
				attemptParam.Headers[k] = v
			}
		}
		attemptParam.Builder = originalBuilder

		return executeSingleAttempt(w, &attemptParam, opts, attemptReqKey, attemptRespKey, attemptFailKey)
	})

	return finalizeResult(retryResult.Response, retryResult.Error, opts)
}

// executeSingleAttempt runs one attempt of the remote call with full
// observability (AddLog, metrics, transaction logging, OnComplete).
// Returns the response, the HTTP status code, and an error.
func executeSingleAttempt[Req any, Resp any](
	w webFramework.WebFramework,
	param *libCallApi.RemoteCallParamData[Req, Resp],
	opts CallAPIOptions,
	reqKey, respKey, failKey string,
) (*Resp, int, error) {
	webFramework.AddLog(w, CallAPILogEntry, slog.Any(reqKey, param))

	// Wrap the builder in a closure to:
	// 1. Capture the actual HTTP status code (RemoteCall does not expose it).
	// 2. Intercept non-2xx responses and always return RemoteCallError,
	//    regardless of the caller's custom builder. Custom builders are only
	//    called for 2xx (successful) responses.
	var actualStatus int
	originalBuilder := param.Builder
	if originalBuilder == nil {
		originalBuilder = libCallApi.StatusPreservingBuilder[Resp]
	}
	param.Builder = func(stat int, rawResp []byte, headers map[string]string) (*Resp, error) {
		actualStatus = stat
		if stat < 200 || stat >= 300 {
			return nil, &libCallApi.RemoteCallError{
				Status: stat,
				Body:   rawResp,
				Err:    fmt.Errorf("HTTP %d", stat),
			}
		}
		return originalBuilder(stat, rawResp, headers)
	}

	start := time.Now()
	resp, err := libCallApi.RemoteCall(w, param)
	elapsed := time.Since(start)

	statusCode := resolveStatusCode(err, actualStatus)
	requestURL := BuildRequestURL(param.API.Domain, param.Path, param.Query)
	recorder := opts.MetricsRecorder
	if recorder == nil {
		recorder = libTracing.DefaultHTTPClientMetricsRecorder()
	}

	if err != nil {
		webFramework.AddLog(w, CallAPILogEntry, slog.Any(failKey, err))
		recorder.Record(param.API.Name, param.Method, statusCode, elapsed, "failure")
		logTransactionAndCallback(w, opts, param, resp, err, statusCode, elapsed, requestURL)
		return nil, statusCode, err
	}

	webFramework.AddLog(w, CallAPILogEntry, slog.Any(respKey, resp))

	if opts.Timeout > 0 && elapsed >= opts.Timeout {
		timeoutErr := BuildTimeoutError(param.API.Domain, opts.TimeoutStatusCode)
		webFramework.AddLog(w, CallAPILogEntry, slog.Any(failKey, timeoutErr))
		recorder.Record(param.API.Name, param.Method, statusCode, elapsed, "timeout")
		logTransactionAndCallback(w, opts, param, resp, timeoutErr, statusCode, elapsed, requestURL)
		return nil, statusCode, timeoutErr
	}

	recorder.Record(param.API.Name, param.Method, statusCode, elapsed, "success")
	logTransactionAndCallback(w, opts, param, resp, err, statusCode, elapsed, requestURL)
	return resp, statusCode, nil
}

// resolveLogKeys returns the request, response, and failure log keys,
// falling back to defaults for empty configured values.
func resolveLogKeys(opts CallAPIOptions) (string, string, string) {
	reqKey := opts.LogKeys.Request
	if reqKey == "" {
		reqKey = opts.Method
	}
	respKey := opts.LogKeys.Response
	if respKey == "" {
		respKey = opts.Method + "-resp"
	}
	failKey := opts.LogKeys.Failure
	if failKey == "" {
		failKey = opts.Method + "-error"
	}
	return reqKey, respKey, failKey
}

// finalizeResult applies NormalizeError if set and returns the final result.
func finalizeResult[Resp any](resp *Resp, err error, opts CallAPIOptions) (Resp, error) {
	if err != nil && opts.NormalizeError != nil {
		err = opts.NormalizeError(err)
	}
	if resp != nil {
		return *resp, err
	}
	return *new(Resp), err
}

// formatRetryKey inserts the retry marker before known suffixes (-resp, -error,
// -failed) or appends it for base keys. e.g. "svc-call" → "svc-call-retry-1",
// "svc-call-resp" → "svc-call-retry-1-resp", "svc-call-error" → "svc-call-retry-1-error",
// "svc-call-req-failed" → "svc-call-req-retry-1-failed".
func formatRetryKey(key string, attempt int) string {
	if attempt <= 1 {
		return key
	}
	retryMarker := fmt.Sprintf("-retry-%d", attempt-1)
	for _, suffix := range []string{"-resp", "-error", "-failed"} {
		if strings.HasSuffix(key, suffix) {
			return strings.TrimSuffix(key, suffix) + retryMarker + suffix
		}
	}
	return key + retryMarker
}

// resolveStatusCode returns the actual HTTP status code captured by the builder
// closure. On error, it prefers the RemoteCallError's status if available.
func resolveStatusCode(err error, actualStatus int) int {
	var rce *libCallApi.RemoteCallError
	if errors.As(err, &rce) {
		return rce.Status
	}
	if actualStatus > 0 {
		return actualStatus
	}
	return 0
}

// BuildRequestURL constructs the full request URL from domain, path, and query.
// It mirrors the URL construction in libCallApi.PrepareCall, which builds the
// request URL as Domain + "/" + Path + Query (direct concatenation).
// Callers must ensure Query is either empty or begins with "?".
func BuildRequestURL(domain, path, query string) string {
	return domain + "/" + path + query
}

// logTransactionAndCallback resolves the TransactionLogger from the framework
// context and invokes it, then invokes the OnComplete callback if set.
// This runs after webFramework.AddLog on each path and never bypasses it.
func logTransactionAndCallback[Req any, Resp any](
	w webFramework.WebFramework,
	opts CallAPIOptions,
	param *libCallApi.RemoteCallParamData[Req, Resp],
	resp *Resp,
	err error,
	statusCode int,
	elapsed time.Duration,
	requestURL string,
) {
	var respAny any
	if resp != nil {
		respAny = *resp
	}
	var rawBody []byte
	if err != nil {
		var rce *libCallApi.RemoteCallError
		if errors.As(err, &rce) {
			rawBody = rce.Body
		}
	}

	requestField := any(param.JsonBody)
	var maskedBody any
	if opts.MaskFunc != nil {
		requestField = opts.MaskFunc(requestField)
		respAny = opts.MaskFunc(respAny)
		if rawBody != nil {
			maskedBody = opts.MaskFunc(rawBody)
		}
	}

	info := webFramework.TransactionInfo{
		ServiceName:        param.API.Name,
		URL:                requestURL,
		Endpoint:           param.Path,
		Method:             param.Method,
		StatusCode:         statusCode,
		Duration:           elapsed,
		Error:              err,
		Request:            requestField,
		Response:           respAny,
		ResponseBody:       rawBody,
		MaskedResponseBody: maskedBody,
	}
	if logger, ok := w.Parser.GetLocal(webFramework.TransactionLoggerLocalKey).(webFramework.TransactionLogger); ok && logger != nil {
		logger.LogTransaction(info)
	}
	if opts.OnComplete != nil {
		opts.OnComplete(info)
	}
}
