package libCallApi

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

// CallParam is a pointer alias for CallParamData used in Call.
type CallParam *CallParamData

// CallParamData holds the parameters for a generic remote API call.
type CallParamData struct {
	HTTPClient  *http.Client
	Parameters  map[string]any
	Headers     map[string]string
	API         RemoteAPI
	Timeout     time.Duration
	Method      string
	Path        string
	Query       string
	QueryStack  *[]string
	ValidateTLS bool
	EnableLog   bool
	JSONBody    any
	Parser      webFramework.RequestParser `json:"-"` // Parser for distributed tracing and request cancellation
}

// LogValue returns a structured slog.Value summarizing the call parameters.
func (r CallParamData) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("api", r.API.Name),
		slog.String("domain", r.API.Domain),
		slog.String("method", r.Method),
		slog.String("path", r.Path),
		slog.String("query", r.Query),
		slog.Any("params", r.Parameters),
		slog.Any("headers", r.Headers),
		slog.Any("request", r.JSONBody),
	)
}

// BuilerFunc is the signature for a response builder function.
type BuilerFunc[Resp any] func(status int, rawResp []byte, headers map[string]string) (*Resp, error)

// RemoteCallParamData holds the parameters for a typed remote API call.
type RemoteCallParamData[Req, Resp any] struct {
	HTTPClient  *http.Client
	Parameters  map[string]any             `json:"-"`
	Headers     map[string]string          `json:"-"`
	API         RemoteAPI                  `json:"api"`
	Timeout     time.Duration              `json:"-"`
	Method      string                     `json:"method"`
	Path        string                     `json:"path"`
	Query       string                     `json:"-"`
	QueryStack  *[]string                  `json:"-"`
	ValidateTLS bool                       `json:"-"`
	EnableLog   bool                       `json:"-"`
	JSONBody    Req                        `json:"body"`
	BodyType    RequestBodyType            `json:"-"`
	Builder     BuilerFunc[Resp]           `json:"-"`
	Parser      webFramework.RequestParser `json:"-"` // Parser for distributed tracing and request cancellation
}

// LogValue returns a structured slog.Value summarizing the remote call parameters with masked auth.
func (r RemoteCallParamData[Req, Resp]) LogValue() slog.Value {
	headers := maps.Clone(r.Headers)
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Authorization"] = "[masked]"
	return slog.GroupValue(
		slog.String("api", r.API.Name),
		slog.String("domain", r.API.Domain),
		slog.String("method", r.Method),
		slog.String("path", r.Path),
		slog.String("query", r.Query),
		slog.Any("params", r.Parameters),
		slog.Any("headers", headers),
		slog.Any("request", r.JSONBody),
	)
}

// CallResult wraps the outcome of a Call including response, ws response, status, and error.
type CallResult[RespType any] struct {
	Resp   *RespType
	WsResp *response.WsRemoteResponse
	Status *CallResp
	Error  error
}

// Call executes a remote API call and returns a CallResult.
func Call[RespType any](w webFramework.WebFramework, param CallParam) CallResult[RespType] {
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		if len(*param.QueryStack) > 1 {
			*param.QueryStack = (*param.QueryStack)[1:]
		} else {
			*param.QueryStack = nil
		}
	}

	// Prepare context for distributed tracing / cancellation
	ctx := context.Background()
	if param.Parser != nil {
		ctx = param.Parser.GetContext()
	}

	callData := CallData[RespType]{
		API:        param.API,
		Path:       param.Path + param.Query,
		Method:     param.Method,
		Headers:    param.Headers,
		SSLVerify:  !param.ValidateTLS,
		EnableLog:  param.EnableLog,
		Timeout:    param.Timeout,
		Req:        param.JSONBody,
		Context:    ctx,
		LogValue:   (*CallParamData)(param).LogValue(),
		httpClient: param.HTTPClient,
	}

	resp, wsResp, callResp, err := ConsumeRest(w, callData)
	return CallResult[RespType]{resp, wsResp, callResp, err}
}

// RemoteCall executes a typed remote API call and returns the parsed response.
func RemoteCall[Req, Resp any](w webFramework.WebFramework, param *RemoteCallParamData[Req, Resp]) (*Resp, error) {
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		if len(*param.QueryStack) > 1 {
			*param.QueryStack = (*param.QueryStack)[1:]
		} else {
			*param.QueryStack = nil
		}
	}

	callData := CallData[Resp]{
		API:        param.API,
		Path:       param.Path + param.Query,
		Method:     param.Method,
		Headers:    param.Headers,
		SSLVerify:  !param.ValidateTLS,
		EnableLog:  param.EnableLog,
		Timeout:    param.Timeout,
		Req:        param.JSONBody,
		BodyType:   param.BodyType,
		Builder:    param.Builder,
		Context:    w.Ctx,
		LogValue:   param.LogValue(),
		httpClient: param.HTTPClient,
	}

	return ConsumeRestJSON(w, &callData)
}
