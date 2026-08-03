package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type WsResponse[Result any] struct {
	HttpStatus   int                      `json:"-"`
	HttpHeaders  map[string]string        `json:"-"`
	Status       int                      `json:"status"`
	Description  string                   `json:"description"`
	Result       Result                   `json:"result,omitempty"`
	ErrorData    []response.ErrorResponse `json:"errors,omitempty"`
	PrintReceipt *response.Receipt        `json:"printReceipt,omitempty"`
}

const (
	CallApiLogEntry string = "ApiCall"
)

func (w *WsResponse[any]) SetStatus(status int) {
	w.HttpStatus = status
}
func (w *WsResponse[any]) SetHeaders(headers map[string]string) {
	w.HttpHeaders = headers
}

func CallApiInternal[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(method, param))

	resp1 := libCallApi.Call[Resp](w, param)

	if resp1.Error != nil {
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", method), resp1.Error))
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
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp1))
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState().Input(param).SetStatus(resp1.Status.Status)
	}
	return resp1.Resp, nil
}

func CallApi[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := CallApiInternal[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}

func CallApiWithReceipt[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, *response.Receipt, error) {
	result, err := CallApiInternal[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, nil, err
	}
	return &result.Result, result.PrintReceipt, err
}

func CallApiJSON[Req any, Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req, Resp],
) (Resp, error) {
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(method, param))

	param.BodyType = libCallApi.JSON
	if param.Parser == nil {
		param.Parser = w.Parser // Set parser for distributed tracing if not already set
	}
	resp, err := libCallApi.RemoteCall(w, param)
	if err != nil {
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", method), err))
		return *new(Resp), err
	}
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp))
	return *resp, nil
}

func CallApiForm[Req any, Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req, Resp],
) (Resp, error) {
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(method, param))

	param.BodyType = libCallApi.Form
	if param.Parser == nil {
		param.Parser = w.Parser // Set parser for distributed tracing if not already set
	}
	resp, err := libCallApi.RemoteCall(w, param)
	if err != nil {
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", method), err))
		return *new(Resp), err
	}
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp))
	return *resp, nil
}
func callApiNoLog[Resp any](
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

func CallApiNoLog[Resp any](
	w webFramework.WebFramework,
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := callApiNoLog[WsResponse[Resp]](w, method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}

// ApiCallInfo describes a completed remote API call for observability hooks.
// The OnComplete callback in CallApiOptions receives this so application
// layers (e.g. Cartino) can record it in their own transaction logger.
type ApiCallInfo struct {
	ServiceName  string        // param.Api.Name
	URL          string        // full request URL (domain + "/" + path)
	Endpoint     string        // param.Path
	Method       string        // param.Method (HTTP method)
	StatusCode   int           // actual HTTP status (0 if request failed before getting a response)
	Duration     time.Duration // elapsed time
	Error        error         // nil on success
	Request      any           // param.JsonBody (caller may mask sensitive fields)
	Response     any           // parsed response (nil on error)
	ResponseBody []byte        // raw response body (from RemoteCallError on error, nil on success)
}

// CallApiOptions holds optional parameters for CallApiJSONWithOpts.
type CallApiOptions struct {
	Method  string        // log key, e.g. "soha-authorize"
	Timeout time.Duration // server-side elapsed-time guard (0 = no guard)

	// OnComplete is an optional callback invoked after the remote call
	// finishes (success or failure). It receives the call metadata so the
	// application layer can record it in its own transaction logger, metrics,
	// or any other observability system. nil = no callback.
	OnComplete func(ApiCallInfo)
}

// CallApiJSONWithOpts is an enhanced version of CallApiJSON that adds:
//   - Prometheus metrics recording via libTracing.RecordHTTPClientCall
//   - A server-side elapsed-time timeout guard (in addition to the HTTP client timeout)
//   - HTTP status code preservation via RemoteCallError (using StatusPreservingBuilder)
//   - An optional OnComplete callback for application-layer transaction logging
//
// webFramework.AddLog is called on every code path (request, error, response)
// and is never skipped or conditionally bypassed.
//
// Custom headers: set param.Headers directly (e.g. param.Headers["SIGNATURE"] = "...").
func CallApiJSONWithOpts[Req any, Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	param *libCallApi.RemoteCallParamData[Req, Resp],
	opts CallApiOptions,
) (Resp, error) {
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(opts.Method, param))

	param.BodyType = libCallApi.JSON
	if param.Parser == nil {
		param.Parser = w.Parser
	}

	// Wrap the builder in a closure to capture the actual HTTP status code
	// returned by the remote server, since RemoteCall does not expose it.
	var actualStatus int
	originalBuilder := param.Builder
	if originalBuilder == nil {
		originalBuilder = libCallApi.StatusPreservingBuilder[Resp]
	}
	param.Builder = func(stat int, rawResp []byte, headers map[string]string) (*Resp, error) {
		actualStatus = stat
		return originalBuilder(stat, rawResp, headers)
	}

	start := time.Now()
	resp, err := libCallApi.RemoteCall(w, param)
	elapsed := time.Since(start)

	statusCode := resolveStatusCode(err, actualStatus)

	if err != nil {
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", opts.Method), err))
		libTracing.RecordHTTPClientCall(param.Api.Name, param.Method, statusCode, elapsed, err)
		invokeOnComplete(opts, param, resp, err, statusCode, elapsed)
		return *new(Resp), err
	}

	webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-resp", opts.Method), resp))

	if opts.Timeout > 0 && elapsed > opts.Timeout {
		timeoutErr := fmt.Errorf("%s: elapsed %s exceeds timeout %s: %w",
			param.Api.Domain, elapsed, opts.Timeout, errTimeout)
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", opts.Method), timeoutErr))
		libTracing.RecordHTTPClientCall(param.Api.Name, param.Method, statusCode, elapsed, timeoutErr)
		invokeOnComplete(opts, param, resp, timeoutErr, statusCode, elapsed)
		return *new(Resp), timeoutErr
	}

	libTracing.RecordHTTPClientCall(param.Api.Name, param.Method, statusCode, elapsed, nil)
	invokeOnComplete(opts, param, resp, err, statusCode, elapsed)
	return *resp, nil
}

var errTimeout = errors.New("server-side timeout exceeded")

// resolveStatusCode returns the actual HTTP status code captured by the builder
// closure. On error, it prefers the RemoteCallError's status if available.
func resolveStatusCode(err error, actualStatus int) int {
	if err != nil {
		var rce *libCallApi.RemoteCallError
		if errors.As(err, &rce) {
			return rce.Status
		}
		return 0
	}
	if actualStatus > 0 {
		return actualStatus
	}
	return http.StatusOK
}

func invokeOnComplete[Req any, Resp any](
	opts CallApiOptions,
	param *libCallApi.RemoteCallParamData[Req, Resp],
	resp *Resp,
	err error,
	statusCode int,
	elapsed time.Duration,
) {
	if opts.OnComplete == nil {
		return
	}
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
	opts.OnComplete(ApiCallInfo{
		ServiceName:  param.Api.Name,
		URL:          param.Api.Domain + "/" + param.Path,
		Endpoint:     param.Path,
		Method:       param.Method,
		StatusCode:   statusCode,
		Duration:     elapsed,
		Error:        err,
		Request:      param.JsonBody,
		Response:     respAny,
		ResponseBody: rawBody,
	})
}
