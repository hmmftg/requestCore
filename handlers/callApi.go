package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
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

	resp1 := libCallApi.Call[Resp](param)

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
	resp, err := libCallApi.RemoteCall(param)
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
	resp, err := libCallApi.RemoteCall(param)
	if err != nil {
		webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-error", method), err))
		return *new(Resp), err
	}
	webFramework.AddLog(w, CallApiLogEntry, slog.Any(fmt.Sprintf("%s-resp", method), resp))
	return *resp, nil
}
func callApiNoLog[Resp any](
	_ string,
	param libCallApi.CallParam) (*Resp, error) {
	resp1 := libCallApi.Call[Resp](param)

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
	method string,
	param libCallApi.CallParam) (*Resp, error) {
	result, err := callApiNoLog[WsResponse[Resp]](method, param)
	if result == nil {
		return nil, err
	}
	return &result.Result, err
}
