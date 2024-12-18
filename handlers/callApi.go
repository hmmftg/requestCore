package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libRequest"
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
	param libCallApi.CallParam) (*Resp, response.ErrorState) {
	var reqLog libRequest.RequestPtr
	dump, err := json.MarshalIndent(param, "", "  ")
	if err == nil {
		reqLog = core.RequestTools().LogStart(w, method, string(dump))
	} else {
		reqLog = core.RequestTools().LogStart(w, method, fmt.Sprintf("params: %+v", param))
	}
	resp1 := libCallApi.Call[Resp](param)
	dump, err = json.MarshalIndent(resp1, "", "  ")
	if err == nil {
		core.RequestTools().LogEnd(method, string(dump), reqLog)
	} else {
		core.RequestTools().LogEnd(method, fmt.Sprintf("resp: %+v", resp1), reqLog)
	}

	if resp1.Error != nil {
		return nil, response.Errors(http.StatusInternalServerError, "REMOTE_CALL_ERROR", param, resp1.Error)
	}
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState().Input(param).SetStatus(resp1.Status.Status)
	}
	return resp1.Resp, nil
}

func CallApi[Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, response.ErrorState) {
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
	param libCallApi.CallParam) (*Resp, *response.Receipt, response.ErrorState) {
	result, err := CallApiInternal[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, nil, err
	}
	return &result.Result, result.PrintReceipt, err
}

func CallApiJSON[Req any, Resp libCallApi.ApiResp](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req],
) (Resp, response.ErrorState) {
	var reqLog libRequest.RequestPtr
	dump, errJSON := json.MarshalIndent(param, "", "  ")
	if errJSON == nil {
		reqLog = core.RequestTools().LogStart(w, method, string(dump))
	} else {
		reqLog = core.RequestTools().LogStart(w, method, fmt.Sprintf("params: %+v", param))
	}
	resp, err := libCallApi.RemoteCall[Req, Resp](param)
	if err != nil {
		core.RequestTools().LogEnd(method, "remote call error: "+err.Error(), reqLog)
		return *new(Resp), err
	}
	// core.RequestTools().LogEnd(method, fmt.Sprintf("resp: %+v", resp), reqLog)
	dump, errJSON = json.MarshalIndent(resp, "", "  ")
	if errJSON != nil {
		core.RequestTools().LogEnd(method, "invalid json resp: "+errJSON.Error(), reqLog)
		return *new(Resp), response.ToError("", "", errJSON)
	}
	core.RequestTools().LogEnd(method, string(dump), reqLog)
	return *resp, nil
}

func callApiNoLog[Resp any](
	_ string,
	param libCallApi.CallParam) (*Resp, error) {
	resp1 := libCallApi.Call[Resp](param)

	if resp1.Error != nil {
		return nil, response.Errors(http.StatusInternalServerError, "REMOTE_CALL_ERROR", param, resp1.Error)
	}
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState().Input(param).SetStatus(resp1.Status.Status)
	}
	return resp1.Resp, nil
}

func CallApiNoLog[Resp any](
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := callApiNoLog[WsResponse[Resp]](method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}
