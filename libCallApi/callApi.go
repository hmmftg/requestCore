package libCallApi

//lint:file-ignore SA4006 gopls/staticcheck false-positive in this file (span-related diagnostics)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"time"

	"github.com/google/go-querystring/query"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

// ConsumeRestBasicAuthAPI calls a remote REST API using basic authentication.
func (m RemoteAPIModel) ConsumeRestBasicAuthAPI(w webFramework.WebFramework, requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	api := m.RemoteAPIList[apiName]
	if headers == nil {
		headers = make(map[string]string)
	}
	if err := api.EnsureAuthorization(w, headers); err != nil {
		return nil, "AUTH_FAILED", err
	}
	req, err := http.NewRequest(method, api.Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", err
	}
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "#" + m.RemoteAPIList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "#" + m.RemoteAPIList[apiName].Name + "#", err
	}
	defer func() { _ = resp.Body.Close() }()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "#" + m.RemoteAPIList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "#" + m.RemoteAPIList[apiName].Name + "#", err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteAPIList[apiName].Name, resp.Status)
		return nil, errorDesc, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteAPIList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, nil
}

// GetApi returns the RemoteAPI registered under the given name.
func (m RemoteAPIModel) GetApi(apiName string) RemoteAPI {
	return m.RemoteAPIList[apiName]
}

// ConsumeRestAPI calls a remote REST API and returns the response body, status text, status code, and error.
func (m RemoteAPIModel) ConsumeRestAPI(w webFramework.WebFramework, requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	api := m.RemoteAPIList[apiName]
	if headers == nil {
		headers = make(map[string]string)
	}
	if err := api.EnsureAuthorization(w, headers); err != nil {
		return nil, "AUTH_FAILED", http.StatusInternalServerError, err
	}
	req, err := http.NewRequest(method, api.Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", http.StatusInternalServerError, err
	}
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "# " + m.RemoteAPIList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "# " + m.RemoteAPIList[apiName].Name + "#", http.StatusRequestTimeout, err
	}
	defer func() { _ = resp.Body.Close() }()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "# " + m.RemoteAPIList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "# " + m.RemoteAPIList[apiName].Name + "#", http.StatusRequestTimeout, err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, resp.StatusCode, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteAPIList[apiName].Name, resp.Status)
		return nil, errorDesc, resp.StatusCode, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteAPIList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, resp.StatusCode, nil
}

// responseBodySummary returns a safe summary for error messages (no raw body). Optional hash helps support correlate with logs.
func responseBodySummary(body []byte, statusCode int) string {
	if len(body) == 0 {
		return fmt.Sprintf("size=0 status=%d", statusCode)
	}
	h := sha256.Sum256(body)
	return fmt.Sprintf("size=%d status=%d hash=%s", len(body), statusCode, hex.EncodeToString(h[:])[:8])
}

const maxLogBodyBytes = 500

func logResponseBodyForDebug(apiName, label string, body []byte) {
	if len(body) == 0 {
		return
	}
	preview := string(body)
	if len(preview) > maxLogBodyBytes {
		preview = preview[:maxLogBodyBytes] + "... truncated"
	}
	slog.Debug("response body for debug", slog.String("api", apiName), slog.String("label", label), slog.String("body", preview))
}

// RequestBodyType identifies the kind of request body to send.
type RequestBodyType int

const (
	// JSON indicates a JSON request body.
	JSON RequestBodyType = iota
	// Form indicates a form-urlencoded request body.
	Form
	// Empty indicates no request body.
	Empty
)

// CallData holds all parameters needed to perform an instrumented remote API call.
type CallData[Resp any] struct {
	httpClient *http.Client
	API        RemoteAPI
	Path       string
	Method     string
	Headers    map[string]string
	Req        any
	SSLVerify  bool
	BodyType   RequestBodyType
	Timeout    time.Duration
	EnableLog  bool
	LogLevel   int
	Builder    func(int, []byte, map[string]string) (*Resp, error)
	Context    context.Context // Context for distributed tracing and request cancellation
	// LogValue is optional and used only for tracing attributes (derived from the caller's LogValue()).
	LogValue slog.Value
}

// CallResp contains the HTTP status and response headers from a remote call.
type CallResp struct {
	Headers map[string]string
	Status  int
}

// TODO replace response.Error with errors.Join(err, libError.New
func GetResp[Resp any, Error any](api RemoteAPI, resp *http.Response) (*Resp, *Error, *CallResp, error) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_READ_TIMED_OUT", "timeout in GetResp.ReadAll(%s)", api.Name))
		}
		return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "API_UNABLE_TO_READ", "error in GetResp.ReadAll(%s)", api.Name))
	}
	var respJson Resp
	var errJson Error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.Unmarshal(responseData, &respJson)
		if err != nil {
			logResponseBodyForDebug(api.Name, "API_OK_RESP_JSON", responseData)
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "API_OK_RESP_JSON", "error in %s GetResp.Unmarshal: %s", api.Name, responseBodySummary(responseData, resp.StatusCode)))
		}
	default:
		err = json.Unmarshal(responseData, &errJson)
		if err != nil {
			logResponseBodyForDebug(api.Name, "API_NOK_RESP_JSON", responseData)
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(status.StatusCode(resp.StatusCode), "API_NOK_RESP_JSON", "error in %s GetResp.Unmarshal: %s", api.Name, responseBodySummary(responseData, resp.StatusCode)))
		}
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	return &respJson, &errJson, &CallResp{Status: resp.StatusCode, Headers: headerMap}, nil
}

// GetJSONResp reads and parses an HTTP response into a typed result using an optional builder.
func GetJSONResp[Resp any](api RemoteAPI, resp *http.Response, Builder func(int, []byte, map[string]string) (*Resp, error)) (*Resp, error) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_READ_TIMED_OUT", "error in GetResp.ReadAll"))
		}
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_UNABLE_TO_READ", "error in GetResp.ReadAll"))
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	if Builder != nil {
		return Builder(resp.StatusCode, responseData, headerMap)
	}

	var jsonResp Resp
	err = json.Unmarshal(responseData, &jsonResp)
	if err != nil {
		logResponseBodyForDebug(api.Name, "API_UNABLE_PARSE_RESP", responseData)
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", "error in GetResp.json.Unmarshal: %s", responseBodySummary(responseData, resp.StatusCode)))
	}
	return &jsonResp, nil
}

// PrepareCall constructs an *http.Request from CallData, applying auth, headers, and tracing propagation.
func PrepareCall[Resp any](w webFramework.WebFramework, c CallData[Resp]) (*http.Request, error) {
	var to time.Duration
	if timeOutString, ok := c.Headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		to = time.Duration(timeoutSeconds * int(time.Second))
	} else if c.Timeout > 0 {
		to = c.Timeout
	} else {
		to = defaultTimeOut
	}
	if c.httpClient == nil {
		httpClient.Timeout = to
	} else if to != defaultTimeOut {
		// Only override a supplied client's timeout when an explicit
		// Time-Out header or CallData.Timeout is set. When neither is
		// provided (to == defaultTimeOut), preserve the caller's timeout.
		c.httpClient.Timeout = to
	}
	var buffer *bytes.Buffer
	switch c.BodyType {
	case JSON:
		jString, err := json.Marshal(c.Req)
		if err != nil {
			return nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "Generate Request Failed", "error in PrepareCall.Marshal: %T:%+v", c.Req, c.Req))
		}
		buffer = bytes.NewBuffer(jString)
	case Form:
		form, err := query.Values(c.Req)
		if err != nil {
			return nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "Generate Request Failed", "error in PrepareCall.Marshal: %T:%v", c.Req, c.Req))
		}
		buffer = bytes.NewBuffer([]byte(form.Encode()))
	case Empty:
		buffer = bytes.NewBuffer([]byte(""))
	}
	if buffer == nil {
		return nil, libError.NewWithDescription(http.StatusInternalServerError, "Generate Request Failed", "error in PrepareCall: type is not defined %d", c.BodyType)
	}
	// Use context from CallData if provided, otherwise use context.Background()
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, c.Method, c.API.Domain+"/"+c.Path, buffer)
	if err != nil {
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "Generate Request Failed", "error in PrepareCall.NewRequestWithContext M=%s,Url:%s,json:%s", c.Method, c.API.Domain+"/"+c.Path, buffer.String()))
	}

	// Explicitly inject trace context into headers for distributed tracing
	// This ensures trace context is propagated even if otelhttp.NewTransport doesn't extract it correctly
	propagator := otel.GetTextMapPropagator()
	if propagator != nil {
		propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
	}

	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if err := c.API.EnsureAuthorization(w, c.Headers); err != nil {
		return nil, err
	}
	switch c.BodyType {
	case JSON:
		req.Header.Add("Content-Type", "application/json")
	case Form:
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	// Apply caller-provided headers first, then add the default Accept
	// only when the caller has not already set one. This avoids duplicate
	// Accept headers when a caller provides a custom value.
	for header, value := range c.Headers {
		req.Header.Add(header, value)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	return req, nil
}

// SetLogs attaches an httptrace.ClientTrace to the request for verbose connection logging.
func (c CallData[Resp]) SetLogs(req *http.Request) *http.Request {
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			log.Println("Got Conn:", connInfo)
		},
		ConnectStart: func(network, addr string) {
			log.Println("Dial start:", network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			log.Println("Dial done:", network, addr, err)
		},
		GotFirstResponseByte: func() {
			log.Println("First response byte!")
		},
		WroteHeaders: func() {
			log.Println("Wrote headers")
		},
		WroteRequest: func(wr httptrace.WroteRequestInfo) {
			log.Println("Wrote request", wr)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	log.Println("Starting request!")
	return req
}

// ConsumeRest executes a remote API call and returns the parsed response, ws response, and call metadata.
func ConsumeRest[Resp any](w webFramework.WebFramework, c CallData[Resp]) (*Resp, *response.WsRemoteResponse, *CallResp, error) {
	req, err := PrepareCall(w, c)
	if err != nil {
		if ok, errPrepare := response.Unwrap(err); ok {
			return nil, nil, nil, errPrepare.Input(c)
		}
		return nil, nil, nil, err
	}

	cl := httpClient
	if c.httpClient != nil {
		cl = c.httpClient
	}

	if c.EnableLog {
		req = c.SetLogs(req)
	}

	// Distributed tracing / cancellation context
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	spanName, spanAttrs := libTracing.HTTPClientSpanNameAndAttrs(
		c.API.Name,
		c.API.Domain,
		c.Method,
		c.Path,
		c.Timeout,
		c.SSLVerify,
	)
	for k, v := range libTracing.SpanAttrsFromSlogValue("call", c.LogValue) {
		spanAttrs[k] = v
	}

	startTime := time.Now()

	// Ensure propagation by running request with the span context.
	resp, traceCtx, err := libTracing.TraceFuncWithSpanName(ctx, spanName, spanAttrs, func(spanCtx context.Context) (*http.Response, error) {
		return cl.Do(req.WithContext(spanCtx))
	})
	if err != nil {
		// Record connection/network errors
		if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
			libTracing.RecordError(traceCtx, err, map[string]string{
				"error.type": "http_client_error",
			})
			span.SetStatus(codes.Error, "HTTP request failed")
		}
		if os.IsTimeout(err) {
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", "error in ConsumeRest.ClientDo: %s %s", req.Method, req.RequestURI))
		}
		return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", "error in ConsumeRest.ClientDo: %s %s", req.Method, req.RequestURI))
	}
	defer func() { _ = resp.Body.Close() }()

	// Add HTTP response attributes to span
	if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
		duration := time.Since(startTime)
		libTracing.AddSpanAttributes(traceCtx, map[string]string{
			"http.status_code":      fmt.Sprintf("%d", resp.StatusCode),
			"http.response.size":    fmt.Sprintf("%d", resp.ContentLength),
			"http.request.duration": duration.String(),
		})

		// Set span status based on HTTP status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			span.SetStatus(codes.Ok, "")
		} else if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
	}

	var respJson *Resp
	var errResp *response.WsRemoteResponse

	respJson, errResp, callResp, err := GetResp[Resp, response.WsRemoteResponse](c.API, resp)
	if err != nil {
		// Record parsing/response errors
		if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
			libTracing.RecordError(traceCtx, err, map[string]string{
				"error.type":       "response_parsing_error",
				"http.status_code": fmt.Sprintf("%d", resp.StatusCode),
			})
		}
		if ok, errPrepare := response.Unwrap(err); ok {
			return nil, nil, nil, errPrepare.Input(resp)
		}
		return nil, nil, nil, err
	}

	return respJson, errResp, callResp, nil
}

// DefaultBuilderfunc is the default response builder that unmarshals JSON into the result type.
func DefaultBuilderfunc[Resp any](stat int, rawResp []byte, headers map[string]string) (*Resp, error) {
	if stat != http.StatusOK {
		return nil, libError.NewWithDescription(status.StatusCode(stat), "API_RESP_NOK", "build request failed, status %d", stat)
	}
	var resp Resp
	err := json.Unmarshal(rawResp, &resp)
	if err != nil {
		slog.Debug("DefaultBuilderfunc unmarshal failed", slog.String("body_summary", responseBodySummary(rawResp, stat)))
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", "error in GetResp.json.Unmarshal: %s", responseBodySummary(rawResp, stat)))
	}
	return &resp, nil
}

// ConsumeRestJSON executes a remote API call and returns the parsed JSON response.
func ConsumeRestJSON[Resp any](w webFramework.WebFramework, c *CallData[Resp]) (*Resp, error) {
	req, err := PrepareCall(w, *c)
	if err != nil {
		if ok, errPrepare := response.Unwrap(err); ok {
			return nil, errPrepare.Input(c)
		}
		return nil, err
	}

	cl := httpClient
	if c.httpClient != nil {
		cl = c.httpClient
	}

	if c.EnableLog {
		req = c.SetLogs(req)
	}

	// Distributed tracing / cancellation context
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	spanName, spanAttrs := libTracing.HTTPClientSpanNameAndAttrs(
		c.API.Name,
		c.API.Domain,
		c.Method,
		c.Path,
		c.Timeout,
		c.SSLVerify,
	)
	for k, v := range libTracing.SpanAttrsFromSlogValue("call", c.LogValue) {
		spanAttrs[k] = v
	}
	startTime := time.Now()

	// Ensure propagation by running request with the span context.
	resp, traceCtx, err := libTracing.TraceFuncWithSpanName(ctx, spanName, spanAttrs, func(spanCtx context.Context) (*http.Response, error) {
		return cl.Do(req.WithContext(spanCtx))
	})
	if err != nil {
		// Record connection/network errors
		if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
			libTracing.RecordError(traceCtx, err, map[string]string{
				"error.type": "http_client_error",
			})
			span.SetStatus(codes.Error, "HTTP request failed")
		}
		if os.IsTimeout(err) {
			return nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", "error in ConsumeRest.ClientDo: %s %s", req.Method, req.RequestURI))
		}
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", "error in ConsumeRest.ClientDo: %s %s", req.Method, req.RequestURI))
	}
	defer func() { _ = resp.Body.Close() }()

	// Add HTTP response attributes to span
	if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
		duration := time.Since(startTime)
		libTracing.AddSpanAttributes(traceCtx, map[string]string{
			"http.status_code":      fmt.Sprintf("%d", resp.StatusCode),
			"http.response.size":    fmt.Sprintf("%d", resp.ContentLength),
			"http.request.duration": duration.String(),
		})

		// Set span status based on HTTP status code
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			span.SetStatus(codes.Ok, "")
		} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		} else if resp.StatusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
	}

	if c.Builder == nil {
		c.Builder = DefaultBuilderfunc[Resp]
	}

	respJson, err := GetJSONResp(c.API, resp, c.Builder)
	if err != nil {
		// Record parsing/response errors
		if span := trace.SpanFromContext(traceCtx); span.IsRecording() {
			libTracing.RecordError(traceCtx, err, map[string]string{
				"error.type":       "response_parsing_error",
				"http.status_code": fmt.Sprintf("%d", resp.StatusCode),
			})
		}
		if ok, errPrepare := response.Unwrap(err); ok {
			return nil, errPrepare.Input(c)
		}
		return nil, err
	}

	return respJson, nil
}

// TransmitRequestWithAuth sends a request through a consume handler and parses the remote response.
func TransmitRequestWithAuth(
	path, api, method string,
	requestByte []byte,
	headers map[string]string,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	consumeHandler func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
) (int, map[string]string, any, error) {
	var resp response.WsRemoteResponse
	respBytes, desc, status, err := consumeHandler(requestByte, api, path, "application/json", method, headers)
	if err != nil {
		return status, map[string]string{"desc": desc, "message": desc}, resp, err
	}
	status, result, respApi, err := parseRemoteResp(respBytes, desc, status)
	if err != nil || status != http.StatusOK {
		return status, result, respApi, err
	}
	return http.StatusOK, nil, respApi, nil
}

// BasicAuth returns the base64-encoded basic authentication string for the given credentials.
func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// TransmitSoap sends a SOAP request to the given URL and parses the XML response.
func TransmitSoap[Resp any](request any, url string, debug bool, timeout time.Duration) (*Resp, error) {
	requestBytes, _ := xml.MarshalIndent(&request, " ", "  ")
	req, requestErr := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewBuffer(requestBytes),
	)
	if requestErr != nil {
		return nil, requestErr
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, err
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debug {
		log.Println(string(result))
	}
	var respXml Resp
	err = xml.Unmarshal(result, &respXml)
	if err != nil {
		return nil, err
	}
	return &respXml, nil
}
