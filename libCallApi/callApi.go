package libCallApi

//lint:file-ignore SA4006 gopls/staticcheck false-positive in this file (span-related diagnostics)

import (
	"bytes"
	"context"
	"encoding/base64"
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
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func (m RemoteApiModel) ConsumeRestBasicAuthApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", err
	}
	req.SetBasicAuth(m.RemoteApiList[apiName].AuthData.User, m.RemoteApiList[apiName].AuthData.Password)
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
		return nil, errorDesc, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, nil
}

func (m RemoteApiModel) GetApi(apiName string) RemoteApi {
	return m.RemoteApiList[apiName]
}

func (m RemoteApiModel) ConsumeRestApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", http.StatusInternalServerError, err
	}
	if _, ok := headers["Authorization"]; !ok {
		req.SetBasicAuth(m.RemoteApiList[apiName].AuthData.User, m.RemoteApiList[apiName].AuthData.Password)
	}
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, resp.StatusCode, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
		return nil, errorDesc, resp.StatusCode, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, resp.StatusCode, nil
}

type RequestBodyType int

const (
	JSON RequestBodyType = iota
	Form
	Empty
)

type CallData[Resp any] struct {
	httpClient *http.Client
	Api        RemoteApi
	Path       string
	Method     string
	Headers    map[string]string
	Req        any
	SslVerify  bool
	BodyType   RequestBodyType
	Timeout    time.Duration
	EnableLog  bool
	LogLevel   int
	Builder    func(int, []byte, map[string]string) (*Resp, error)
	Context    context.Context // Context for distributed tracing and request cancellation
	// LogValue is optional and used only for tracing attributes (derived from the caller's LogValue()).
	LogValue slog.Value
}

type CallResp struct {
	Headers map[string]string
	Status  int
}

// TODO replace response.Error with errors.Join(err, libError.New
func GetResp[Resp any, Error any](api RemoteApi, resp *http.Response) (*Resp, *Error, *CallResp, error) {
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
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "API_OK_RESP_JSON", "error in %s GetResp.Unmarshal:%s", api.Name, string(responseData)))
		}
	default:
		err = json.Unmarshal(responseData, &errJson)
		if err != nil {
			return nil, nil, nil, errors.Join(err, libError.NewWithDescription(status.StatusCode(resp.StatusCode), "API_NOK_RESP_JSON", "error in %s GetResp.Unmarshal:%s", api.Name, string(responseData)))
		}
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	return &respJson, &errJson, &CallResp{Status: resp.StatusCode, Headers: headerMap}, nil
}

func GetJSONResp[Resp any](api RemoteApi, resp *http.Response, Builder func(int, []byte, map[string]string) (*Resp, error)) (*Resp, error) {
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
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", "error in GetResp.json.Unmarshal: %s", responseData))
	}
	return &jsonResp, nil
}

func PrepareCall[Resp any](c CallData[Resp]) (*http.Request, error) {
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
	} else {
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
	req, err := http.NewRequestWithContext(ctx, c.Method, c.Api.Domain+"/"+c.Path, buffer)
	if err != nil {
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusInternalServerError, "Generate Request Failed", "error in PrepareCall.NewRequestWithContext M=%s,Url:%s,json:%s", c.Method, c.Api.Domain+"/"+c.Path, buffer.String()))
	}

	// Explicitly inject trace context into headers for distributed tracing
	// This ensures trace context is propagated even if otelhttp.NewTransport doesn't extract it correctly
	propagator := otel.GetTextMapPropagator()
	if propagator != nil {
		propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
	}

	if _, ok := c.Headers["Authorization"]; !ok {
		req.SetBasicAuth(c.Api.AuthData.User, c.Api.AuthData.Password)
	}
	switch c.BodyType {
	case JSON:
		req.Header.Add("Content-Type", "application/json")
	case Form:
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Add("Accept", "application/json")
	for header, value := range c.Headers {
		req.Header.Add(header, value)
	}

	return req, nil
}

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

func ConsumeRest[Resp any](c CallData[Resp]) (*Resp, *response.WsRemoteResponse, *CallResp, error) {
	req, err := PrepareCall(c)
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
		c.Api.Name,
		c.Api.Domain,
		c.Method,
		c.Path,
		c.Timeout,
		c.SslVerify,
	)
	for k, v := range libTracing.SpanAttrsFromSlogValue("call", c.LogValue) {
		spanAttrs[k] = v
	}

	startTime := time.Now()

	// Ensure propagation by running request with the span context.
	resp, err, traceCtx := libTracing.TraceFuncWithSpanName(ctx, spanName, spanAttrs, func(spanCtx context.Context) (*http.Response, error) {
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
	defer resp.Body.Close()

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

	respJson, errResp, callResp, err := GetResp[Resp, response.WsRemoteResponse](c.Api, resp)
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

func DefaultBuilderfunc[Resp any](stat int, rawResp []byte, headers map[string]string) (*Resp, error) {
	if stat != http.StatusOK {
		return nil, libError.NewWithDescription(status.StatusCode(stat), "API_RESP_NOK", "build request failed, status %d", stat)
	}
	var resp Resp
	err := json.Unmarshal(rawResp, &resp)
	if err != nil {
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", "error in GetResp.json.Unmarshal: %s", rawResp))
	}
	return &resp, nil
}

func ConsumeRestJSON[Resp any](c *CallData[Resp]) (*Resp, error) {
	req, err := PrepareCall(*c)
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
		c.Api.Name,
		c.Api.Domain,
		c.Method,
		c.Path,
		c.Timeout,
		c.SslVerify,
	)
	for k, v := range libTracing.SpanAttrsFromSlogValue("call", c.LogValue) {
		spanAttrs[k] = v
	}
	startTime := time.Now()

	// Ensure propagation by running request with the span context.
	resp, err, traceCtx := libTracing.TraceFuncWithSpanName(ctx, spanName, spanAttrs, func(spanCtx context.Context) (*http.Response, error) {
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
	defer resp.Body.Close()

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

	respJson, err := GetJSONResp(c.Api, resp, c.Builder)
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

func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

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
	defer resp.Body.Close()
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
