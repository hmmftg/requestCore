package libCallApi

import (
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/response"
)

type CallParam *CallParamData
type CallParamData struct {
	httpClient  *http.Client
	Parameters  map[string]any
	Headers     map[string]string
	Api         RemoteApi
	Timeout     time.Duration
	Method      string
	Path        string
	Query       string
	QueryStack  *[]string
	ValidateTls bool
	EnableLog   bool
	JsonBody    any
}

func (r CallParamData) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("api", r.Api.Name),
		slog.String("domain", r.Api.Domain),
		slog.String("method", r.Method),
		slog.String("path", r.Path),
		slog.String("query", r.Query),
		slog.Any("params", r.Parameters),
		slog.Any("headers", r.Headers),
		slog.Any("request", r.JsonBody),
	)
}

type RemoteCallParamData[Req, Resp any] struct {
	httpClient  *http.Client
	Parameters  map[string]any                                                                           `json:"-"`
	Headers     map[string]string                                                                        `json:"-"`
	Api         RemoteApi                                                                                `json:"api"`
	Timeout     time.Duration                                                                            `json:"-"`
	Method      string                                                                                   `json:"method"`
	Path        string                                                                                   `json:"path"`
	Query       string                                                                                   `json:"-"`
	QueryStack  *[]string                                                                                `json:"-"`
	ValidateTls bool                                                                                     `json:"-"`
	EnableLog   bool                                                                                     `json:"-"`
	JsonBody    Req                                                                                      `json:"body"`
	BodyType    RequestBodyType                                                                          `json:"-"`
	Builder     func(status int, rawResp []byte, headers map[string]string) (*Resp, response.ErrorState) `json:"-"`
}

func (r RemoteCallParamData[Req, Resp]) LogValue() slog.Value {
	headers := maps.Clone(r.Headers)
	headers["Authorization"] = "[masked]"
	return slog.GroupValue(
		slog.String("api", r.Api.Name),
		slog.String("domain", r.Api.Domain),
		slog.String("method", r.Method),
		slog.String("path", r.Path),
		slog.String("query", r.Query),
		slog.Any("params", r.Parameters),
		slog.Any("headers", headers),
		slog.Any("request", r.JsonBody),
	)
}

type CallResult[RespType any] struct {
	Resp   *RespType
	WsResp *response.WsRemoteResponse
	Status *CallResp
	Error  response.ErrorState
}

func Call[RespType any](param CallParam) CallResult[RespType] {
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		if len(*param.QueryStack) > 1 {
			*param.QueryStack = (*param.QueryStack)[1:]
		} else {
			*param.QueryStack = nil
		}
	}
	callData := CallData[RespType]{
		Api:        param.Api,
		Path:       param.Path + param.Query,
		Method:     param.Method,
		Headers:    param.Headers,
		SslVerify:  !param.ValidateTls,
		EnableLog:  param.EnableLog,
		Timeout:    param.Timeout,
		Req:        param.JsonBody,
		httpClient: param.httpClient,
	}
	resp, wsResp, callResp, err := ConsumeRest[RespType](callData)
	return CallResult[RespType]{resp, wsResp, callResp, err}
}

func RemoteCall[Req, Resp any](param *RemoteCallParamData[Req, Resp]) (*Resp, response.ErrorState) {
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		if len(*param.QueryStack) > 1 {
			*param.QueryStack = (*param.QueryStack)[1:]
		} else {
			*param.QueryStack = nil
		}
	}
	callData := CallData[Resp]{
		Api:        param.Api,
		Path:       param.Path + param.Query,
		Method:     param.Method,
		Headers:    param.Headers,
		SslVerify:  !param.ValidateTls,
		EnableLog:  param.EnableLog,
		Timeout:    param.Timeout,
		Req:        param.JsonBody,
		BodyType:   param.BodyType,
		Builder:    param.Builder,
		httpClient: param.httpClient,
	}
	return ConsumeRestJSON[Resp](&callData)
}
