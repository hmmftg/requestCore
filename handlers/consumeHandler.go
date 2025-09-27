package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/webFramework"
)

func ExtractValue(name string, source func(string) string, dest map[string]string) {
	if strings.Contains(name, "#") {
		headerSplit := strings.Split(name, "#")
		dest[headerSplit[1]] = source(headerSplit[0])
	} else {
		dest[name] = source(name)
	}
}

func ExtractHeaders(w webFramework.WebFramework, headers, locals []string) map[string]string {
	headersMap := make(map[string]string, 0)
	for _, header := range headers {
		ExtractValue(header, w.Parser.GetHeaderValue, headersMap)
	}
	for _, local := range locals {
		ExtractValue(local, w.Parser.GetLocalString, headersMap)
	}
	return headersMap
}

type CallArgs[Req any, Resp any] struct {
	Title, Path, Api, Method string
	HasQuery, IsJson         bool
	HasInitializer           bool
	ForwardAuth              bool
	Transmitter              func(
		path, api, method string,
		requestByte []byte,
		headers map[string]string,
		parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
		consumer func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
	) (int, map[string]string, any, error)
	Args, Locals, Headers []string
	Parser                func(respBytes []byte, desc string, status int) (int, map[string]string, any, error)
	RecoveryHandler       func(any)
}

func (c CallArgs[Req, Resp]) Parameters() HandlerParameters {
	var mode libRequest.Type
	if c.IsJson {
		mode = libRequest.JSON
	} else {
		mode = libRequest.Query
	}
	save := false
	if c.HasInitializer {
		save = true
	}
	return HandlerParameters{
		Title:           c.Title,
		Body:            mode,
		ValidateHeader:  false,
		SaveToRequest:   save,
		Path:            c.Path,
		HasReceipt:      false,
		RecoveryHandler: c.RecoveryHandler,
		FileResponse:    false,
		LogArrays:       nil,
		LogTags:         nil,
		EnableTracing:   false,
		TracingSpanName: "",
	}
}

const (
	HeadersMap = "headersMap"
	FinalPath  = "finalPath"
)

func (c CallArgs[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	if c.ForwardAuth {
		c.Headers = append(c.Headers, "Authorization")
	}
	headersMap := ExtractHeaders(req.W, c.Headers, c.Locals)
	if !c.ForwardAuth {
		remoteApi := req.Core.Params().GetRemoteApi(c.Api)
		headersMap["Authorization"] = "Basic " + libCallApi.BasicAuth(remoteApi.AuthData.User, remoteApi.AuthData.Password)
	}
	req.W.Parser.SetLocal(HeadersMap, headersMap)

	finalPath := c.Path
	for _, value := range req.W.Parser.GetUrlParams() {
		//normalized := strings.ReplaceAll(param.Value, "*", "/")
		finalPath += "/" + value //normalized
	}
	req.W.Parser.SetLocal(FinalPath, finalPath)
	return nil
}
func (c CallArgs[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	finalPath := req.W.Parser.GetLocalString(FinalPath)
	headers, ok := req.W.Parser.GetLocal(HeadersMap).(map[string]string)
	if !ok {
		return req.Response, libError.NewWithDescription(
			http.StatusInternalServerError,
			"BAD_LOCAL_HEADERS",
			"unable to get headers, wrong data type: %T", req.W.Parser.GetLocal(HeadersMap))
	}

	resp, err := libCallApi.RemoteCall(
		&libCallApi.RemoteCallParamData[Req, Resp]{
			Headers:  headers,
			JsonBody: *req.Request,
			Api:      *req.Core.Params().GetRemoteApi(c.Api),
			Method:   c.Method,
			Path:     finalPath,
		},
	)
	if err != nil {
		return req.Response, err
	}
	return *resp, nil
}
func (c CallArgs[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}
func (c CallArgs[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {}

func CallRemote[Req any, Resp any](
	core requestCore.RequestCoreInterface,
	callArg CallArgs[Req, Resp],
	simulation bool,
	args ...string,
) any {
	return BaseHandler(core, callArg, simulation, args)
}

func CallRemoteWithRespParser[Req any, Resp any](
	core requestCore.RequestCoreInterface,
	callArgs CallArgs[Req, Resp],
	simulation bool,
	args ...string,
) any {
	return BaseHandler(core, callArgs, simulation, args)
}

func InitPostRequest(
	w webFramework.WebFramework,
	reqLog libRequest.RequestPtr,
	method, url string,
	checkDuplicate func(libRequest.Request) error,
	insertRequest func(libRequest.Request) error,
	args ...any,
) (int, map[string]string, error) {
	err := checkDuplicate(*reqLog)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	err = insertRequest(*reqLog)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]string{"desc": "UNABLE_TO_REGISTER", "message": "Unable To Register Request"}, err
	}
	var params []any
	for _, arg := range args {
		params = append(params, w.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

type ConsumeHandlerType[Req, Resp any] struct {
	Title           string
	Params          libCallApi.RemoteCallParamData[Req, Resp]
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	SaveToRequest   bool
	HasReceipt      bool
	Headers         []string
	Api             string
	Method          string
	Query           string
	RecoveryHandler func(any)
}

func (h *ConsumeHandlerType[Req, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{
		Title:           h.Title,
		Body:            h.Mode,
		ValidateHeader:  h.VerifyHeader,
		SaveToRequest:   h.SaveToRequest,
		Path:            h.Path,
		HasReceipt:      h.HasReceipt,
		RecoveryHandler: h.RecoveryHandler,
		FileResponse:    false,
		LogArrays:       nil,
		LogTags:         nil,
		EnableTracing:   false,
		TracingSpanName: "",
	}
}

func (h *ConsumeHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	for _, value := range req.W.Parser.GetUrlParams() {
		//normalized := strings.ReplaceAll(param.Value, "*", "/")
		h.Path += "/" + value //normalized
	}
	return nil
}

func (h *ConsumeHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	headersMap := ExtractHeaders(req.W, h.Headers, nil)
	resp, errCall := CallApiJSON(req.W, req.Core, h.Title,
		&libCallApi.RemoteCallParamData[Req, Resp]{
			Api:         *req.Core.Params().GetRemoteApi(h.Api),
			Method:      h.Method,
			Path:        h.Path,
			Query:       h.Query,
			JsonBody:    *req.Request,
			ValidateTls: false,
			EnableLog:   false,
			Headers:     headersMap,
			Builder:     req.Builder,
		})
	if errCall != nil {
		return req.Response, errCall
	}
	return resp, nil
}

func (h *ConsumeHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}

func (h *ConsumeHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {}

func ConsumeHandler[Req, Resp any](
	core requestCore.RequestCoreInterface,
	params *ConsumeHandlerType[Req, Resp],
	simulation bool,
) any {
	return BaseHandler(core, params, simulation)
}
