package libCallApi

import (
	"time"

	"github.com/hmmftg/requestCore/response"
)

type CallParam *CallParamData
type CallParamData struct {
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
type RemoteCallParamData[Req any] struct {
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
	JsonBody    Req
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
	callData := CallData{
		Api:       param.Api,
		Path:      param.Path + param.Query,
		Method:    param.Method,
		Headers:   param.Headers,
		SslVerify: !param.ValidateTls,
		EnableLog: param.EnableLog,
		Timeout:   param.Timeout,
		Req:       param.JsonBody,
	}
	resp, wsResp, callResp, err := ConsumeRest[RespType](callData)
	return CallResult[RespType]{resp, wsResp, callResp, err}
}

type ApiResp interface {
	SetStatus(int)
	SetHeaders(map[string]string)
}

func RemoteCall[Req any, Resp ApiResp](param *RemoteCallParamData[Req]) (*Resp, response.ErrorState) {
	if param.QueryStack != nil && len(*param.QueryStack) > 0 {
		param.Query = (*param.QueryStack)[0]
		if len(*param.QueryStack) > 1 {
			*param.QueryStack = (*param.QueryStack)[1:]
		} else {
			*param.QueryStack = nil
		}
	}
	callData := CallData{
		Api:       param.Api,
		Path:      param.Path + param.Query,
		Method:    param.Method,
		Headers:   param.Headers,
		SslVerify: !param.ValidateTls,
		EnableLog: param.EnableLog,
		Timeout:   param.Timeout,
		Req:       param.JsonBody,
	}
	return ConsumeRestJSON[Resp](&callData)
}
