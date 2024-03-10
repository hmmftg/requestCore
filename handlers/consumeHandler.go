package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func ConsumeRemoteGet(
	w webFramework.WebFramework,
	api, url string,
	core requestCore.RequestCoreInterface,
	args ...any) (int, int, string, any, bool, error) {
	var params []any
	for _, arg := range args {
		switch arg.(type) {
		case string:
			if arg == "QUERY" {
				continue
			}
			argString := arg.(string)
			if len(w.Parser.GetUrlParam(argString)) > 0 {
				params = append(params, w.Parser.GetUrlParam(argString))
			} else {
				if strings.Contains(argString, ":") {
					argSplit := strings.Split(argString, ":")
					switch argSplit[0] {
					case "db":
						_, _, _, argDb, err := libQuery.CallSql[libQuery.QueryData](argSplit[1], core.GetDB())
						if err != nil {
							return http.StatusBadRequest, 1, "UNABLE_TO_PARSE_DB_ARG", "unable to parse db argument", true, err
						}
						params = append(params, argDb[0].Value)
					case "consume":
						consumeArgs := strings.Split(argSplit[1], ",")
						// 200, 0, "OK", resp.Result, false, nil
						_, _, _, remoteData, _, err := ConsumeRemoteGet(w, consumeArgs[0], consumeArgs[1], core, consumeArgs[2])
						if err != nil {
							return http.StatusBadRequest, 1, "UNABLE_TO_PARSE_DB_ARG", "unable to parse db argument", true, err
						}
						remoteMap := remoteData.(map[string]any)
						params = append(params, remoteMap[consumeArgs[3]])
					}
				} else {
					params = append(params, w.Parser.GetLocalString(argString))
				}
			}
		}
	}
	path := fmt.Sprintf(url, params...)

	reqLog := core.RequestTools().LogStart(w, "ConsumeRemoteGet", path)
	headersMap := extractHeaders(w, DefaultHeaders(), DefaultLocals())

	respBytes, desc, err := core.Consumer().ConsumeRestBasicAuthApi(nil, api, path, "application/x-www-form-urlencoded", "GET", headersMap)
	if err != nil {
		return http.StatusInternalServerError, 1, desc, string(respBytes), true, err
	}
	core.RequestTools().LogEnd("ConsumeRemoteGet", desc, reqLog)

	var resp response.WsRemoteResponse
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusInternalServerError, 1, "invalid resp from " + api, err.Error(), true, err
	}
	stat, err := strconv.Atoi(strings.Split(desc, " ")[0])
	if stat != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := resp.ErrorData // .(requestCore.ErrorResponse)
			err = errors.New(errorDesc[0].Code)
			if errorDesc[0].Description != nil {
				switch v := errorDesc[0].Description.(type) {
				case string:
					err = errors.New(v)
				}
			}
			return stat, 1, errorDesc[0].Code, errorDesc[0].Description, true, err
		}
		if len(resp.Description) > 0 {
			return stat, 1, api + " Resp", resp.Description, true, err
		}
		return http.StatusInternalServerError, 1, "invalid resp from " + api, err.Error(), true, err
	}

	return http.StatusOK, 0, "OK", resp.Result, false, nil
}

func extractValue(name string, source func(string) string, dest map[string]string) {
	if strings.Contains(name, "#") {
		headerSplit := strings.Split(name, "#")
		dest[headerSplit[1]] = source(headerSplit[0])
	} else {
		dest[name] = source(name)
	}
}

func extractHeaders(w webFramework.WebFramework, headers, locals []string) map[string]string {
	headersMap := make(map[string]string, 0)
	for _, header := range headers {
		extractValue(header, w.Parser.GetHeaderValue, headersMap)
	}
	for _, local := range locals {
		extractValue(local, w.Parser.GetLocalString, headersMap)
	}
	return headersMap
}

func ConsumeRemoteGetApi(
	api, url string,
	core requestCore.RequestCoreInterface,
	args ...any) any {
	log.Println("ConsumeRemoteGetApi...")
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		fullPath := url
		if len(args) > 0 && args[0] == "QUERY" {
			fullPath = fmt.Sprintf("%s?%s", fullPath, w.Parser.GetRawUrlQuery())
		}

		status, code, desc, message, broken, err := ConsumeRemoteGet(w, api, fullPath, core, args...)
		if err != nil {
			core.Responder().HandleErrorState(err, status, desc, message, w)
			return
		}
		core.Responder().Respond(status, code, desc, message, broken, w)
	}
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

func DefaultHeaders() []string {
	return []string{
		"Authorization",
		"Request-Id",
		"Branch-Id",
		"Person-Id",
	}
}

func DefaultLocals() []string {
	return []string{
		"bankCode#Bank-Code",
		"User-Id",
	}
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
	return HandlerParameters{c.Title, mode, false, save, c.Path, false, c.RecoveryHandler}
}

const (
	HeadersMap = "headersMap"
	FinalPath  = "finalPath"
)

func (c CallArgs[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) response.ErrorState {
	headers := make([]string, 0)
	locals := make([]string, 0)
	if c.ForwardAuth {
		headers = append(headers, "Authorization")
	}
	headers = append(headers, "Request-Id")
	headers = append(headers, "Branch-Id")
	headers = append(headers, "Person-Id")
	locals = append(locals, "User-Id")
	headersMap := extractHeaders(req.W, headers, locals)
	if !c.ForwardAuth {
		remoteApi := req.Core.Consumer().GetApi(c.Api)
		headersMap["Authorization"] = "Basic " + libCallApi.BasicAuth(remoteApi.User, remoteApi.Password)
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
func (c CallArgs[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	finalPath := req.W.Parser.GetLocalString(FinalPath)
	headers, ok := req.W.Parser.GetLocal(HeadersMap).(map[string]string)
	if !ok {
		return req.Response, response.Error(
			http.StatusInternalServerError,
			"BAD_LOCAL_HEADERS",
			req.W.Parser.GetLocal(HeadersMap),
			fmt.Errorf("wront data type: %T", req.W.Parser.GetLocal(HeadersMap)))
	}
	requestByte, _ := json.Marshal(req.Request)
	status, descArray, resp, err := c.Transmitter(
		finalPath, c.Api, c.Method,
		requestByte, headers, c.Parser,
		req.Core.Consumer().ConsumeRestApi)
	if err != nil {
		return req.Response, response.Error(
			status,
			descArray["desc"],
			descArray["message"],
			libError.Join(err, "error calling api"))
	}
	return resp.(Resp), nil
}
func (c CallArgs[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	return req.Response, nil
}
func (c CallArgs[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {}

func CallRemote[Req any, Resp any](
	title, path, api, method string, hasQuery, isJson bool,
	hasInitializer bool,
	transmitter func(
		path, api, method string,
		requestByte []byte,
		headers map[string]string,
		parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
		consumer func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
	) (int, map[string]string, any, error),
	core requestCore.RequestCoreInterface,
	simulation bool,
	args ...string,
) any {
	callArg := CallArgs[Req, Resp]{
		Title:          title,
		Path:           path,
		Api:            api,
		Method:         method,
		HasQuery:       hasQuery,
		IsJson:         isJson,
		HasInitializer: hasInitializer,
		ForwardAuth:    false,
		Transmitter:    transmitter,
		Args:           args,
		Locals:         DefaultLocals(),
		Headers:        DefaultHeaders(),
		Parser:         response.ParseRemoteRespJson,
	}
	return BaseHandler[Req, Resp, CallArgs[Req, Resp]](core, callArg, simulation, args)
}

func CallRemoteWithRespParser[Req any, Resp any](
	title, path, api, method string, hasQuery, isJson, forwardAuth bool,
	hasInitializer bool,
	transmitter func(
		path, api, method string,
		requestByte []byte,
		headers map[string]string,
		parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
		consumer func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
	) (int, map[string]string, any, error),
	core requestCore.RequestCoreInterface,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	simulation bool,
	args ...string,
) any {
	callArg := CallArgs[Req, Resp]{
		Title:          title,
		Path:           path,
		Api:            api,
		Method:         method,
		HasQuery:       hasQuery,
		IsJson:         isJson,
		HasInitializer: hasInitializer,
		ForwardAuth:    forwardAuth,
		Transmitter:    transmitter,
		Args:           args,
		Locals:         DefaultLocals(),
		Headers:        DefaultHeaders(),
		Parser:         parseRemoteResp,
	}
	return BaseHandler[Req, Resp, CallArgs[Req, Resp]](core, callArg, simulation, args)
}

// initializer func(c webFramework.WebFramework, method, url string, reqLog libRequest.RequestPtr, args ...any) (int, map[string]string, error),
func InitPostRequest(
	w webFramework.WebFramework,
	reqLog libRequest.RequestPtr,
	method, url string,
	checkDuplicate func(libRequest.Request) error,
	addEvent func(webFramework.WebFramework, string, string, string, libRequest.RequestPtr),
	insertRequest func(libRequest.Request) error,
	args ...any,
) (int, map[string]string, error) {
	err := checkDuplicate(*reqLog)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	addEvent(w, reqLog.BranchId, method, "start", reqLog)
	err = insertRequest(*reqLog)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]string{"desc": "PWC_REGISTER", "message": "Unable To Register Request"}, err
	}
	var params []any
	for _, arg := range args {
		params = append(params, w.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

func ConsumeRemotePost(
	w webFramework.WebFramework,
	reqLog libRequest.RequestPtr,
	request any,
	method, methodName, api, url string,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	consumeHandler func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
) (int, string, any, error) {
	reqBytes, _ := json.Marshal(request)
	headers := map[string]string{
		"Request-Id": reqLog.Id,
	}

	status, result, resp, err := libCallApi.TransmitRequestWithAuth(url, api, method, reqBytes, headers, parseRemoteResp, consumeHandler)
	if err != nil {
		return status, result["desc"], result["message"], err
	}
	return http.StatusOK, "", resp, nil
}

func CallHandler[Req any, Resp any](
	title, path, api, method, query string, isJson bool,
	hasInitializer bool,
	headers []string,
	core requestCore.RequestCoreInterface,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		var w webFramework.WebFramework
		if hasInitializer {
			w = libContext.InitContext(c)
		} else {
			w = libContext.InitContextNoAuditTrail(c)
		}
		finalPath := path
		for _, value := range w.Parser.GetUrlParams() {
			//normalized := strings.ReplaceAll(param.Value, "*", "/")
			finalPath += "/" + value //normalized
		}
		code, desc, arrayErr, req, reqLog, err := libRequest.GetRequest[Req](w, isJson)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		w.Parser.SetLocal("reqLog", reqLog)
		reqLog.Incoming = req

		if hasInitializer {
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, title, u.Path, reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], w)
				return
			}
		}
		headersMap := extractHeaders(w, headers, nil)
		resp, errCall := CallApi[Resp](w, core, title,
			&libCallApi.CallParamData{
				Api:         core.Consumer().GetApi(api),
				Method:      method,
				Path:        finalPath,
				Query:       query,
				JsonBody:    req,
				ValidateTls: false,
				EnableLog:   false,
				Headers:     headersMap,
			})
		if errCall != nil {
			core.Responder().Error(w, errCall)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)
	}
}

type ConsumeHandlerType[Req any, Resp libCallApi.ApiResp] struct {
	Title           string
	Params          libCallApi.RemoteCallParamData[Req]
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
	}
}

func (h *ConsumeHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) response.ErrorState {
	for _, value := range req.W.Parser.GetUrlParams() {
		//normalized := strings.ReplaceAll(param.Value, "*", "/")
		h.Path += "/" + value //normalized
	}
	return nil
}

func (h *ConsumeHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	headersMap := extractHeaders(req.W, h.Headers, nil)
	resp, errCall := CallApiJSON[Req, Resp](req.W, req.Core, h.Title,
		&libCallApi.RemoteCallParamData[Req]{
			Api:         req.Core.Consumer().GetApi(h.Api),
			Method:      h.Method,
			Path:        h.Path,
			Query:       h.Query,
			JsonBody:    *req.Request,
			ValidateTls: false,
			EnableLog:   false,
			Headers:     headersMap,
		})
	if errCall != nil {
		return req.Response, errCall
	}
	return resp, nil
}

func (h *ConsumeHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	return req.Response, nil
}

func (h *ConsumeHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {}

func ConsumeHandler[Req any, Resp libCallApi.ApiResp](
	core requestCore.RequestCoreInterface,
	params *ConsumeHandlerType[Req, Resp],
	simulation bool,
) any {
	return BaseHandler[Req, Resp, *ConsumeHandlerType[Req, Resp]](core, params, simulation)
}
