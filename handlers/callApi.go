package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type WsResponse[Result any] struct {
	Status       int                      `json:"status"`
	Description  string                   `json:"description"`
	Result       Result                   `json:"result,omitempty"`
	ErrorData    []response.ErrorResponse `json:"errors,omitempty"`
	PrintReceipt *response.Receipt        `json:"printReceipt,omitempty"`
}

func callApi[Resp any](
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
	result, err := callApi[WsResponse[Resp]](w, core, method, param)
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
	result, err := callApi[WsResponse[Resp]](w, core, method, param)
	if result == nil {
		return nil, nil, err
	}
	return &result.Result, result.PrintReceipt, err
}

func CallApiJSON[Req, Resp any](
	w webFramework.WebFramework,
	core requestCore.RequestCoreInterface,
	method string,
	param *libCallApi.RemoteCallParamData[Req],
) (*Resp, response.ErrorState) {
	var reqLog libRequest.RequestPtr
	dump, err := json.MarshalIndent(param, "", "  ")
	if err == nil {
		reqLog = core.RequestTools().LogStart(w, method, string(dump))
	} else {
		reqLog = core.RequestTools().LogStart(w, method, fmt.Sprintf("params: %+v", param))
	}
	resp, err := libCallApi.RemoteCall[Req, Resp](param)
	dump, errJSON := json.MarshalIndent(resp, "", "  ")
	if err == nil {
		core.RequestTools().LogEnd(method, string(dump), reqLog)
	} else {
		log.Println(errJSON)
		core.RequestTools().LogEnd(method, fmt.Sprintf("resp: %+v", resp), reqLog)
	}
	return resp, nil
}

func callApiNoLog[Resp any](
	method string,
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
