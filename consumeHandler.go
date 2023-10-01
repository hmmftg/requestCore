package requestCore

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

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func ConsumeRemoteGet(
	w webFramework.WebFramework,
	api, url string,
	core RequestCoreInterface,
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

	respBytes, desc, err := core.Consumer().ConsumeRestBasicAuthApi(nil, api, path, "application/x-www-form-urlencoded", "GET", nil)
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

func ConsumeRemoteGetApi(
	api, url string,
	core RequestCoreInterface,
	args ...any) any {
	log.Println("ConsumeRemoteGetApi...")
	return func(c context.Context) {
		w := libContext.InitContext(c)
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
	core RequestCoreInterface,
	args ...string,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		headers := make(map[string]string, 0)
		headers["Authorization"] = w.Parser.GetHeaderValue("Authorization")
		headers["Request-Id"] = w.Parser.GetHeaderValue("Request-Id")
		headers["Branch-Id"] = w.Parser.GetHeaderValue("Branch-Id")
		headers["Person-Id"] = w.Parser.GetHeaderValue("Person-Id")
		headers["Bank-Code"] = w.Parser.GetLocalString("bankCode")
		headers["User-Id"] = w.Parser.GetLocalString("User-Id")
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
		w.Parser.SetLocal("reqLog", &reqLog)
		reqLog.Incoming = req

		if hasInitializer {
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, title, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], w)
				return
			}
		}
		requestByte, _ := json.Marshal(req)
		status, descArray, resp, err := transmitter(finalPath, api, method, requestByte, headers, response.ParseRemoteRespJson, core.Consumer().ConsumeRestApi)
		if err != nil {
			core.Responder().HandleErrorState(err, status, descArray["desc"], descArray["message"], w)
			return
		}
		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)
	}
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
	core RequestCoreInterface,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	args ...string,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		headers := make(map[string]string, 0)
		if forwardAuth {
			headers["Authorization"] = w.Parser.GetHeaderValue("Authorization")
		} else {
			remoteApi := core.Consumer().GetApi(api)
			headers["Authorization"] = "Basic " + libCallApi.BasicAuth(remoteApi.User, remoteApi.Password)
		}
		headers["Request-Id"] = w.Parser.GetHeaderValue("Request-Id")
		headers["Branch-Id"] = w.Parser.GetHeaderValue("Branch-Id")
		headers["Person-Id"] = w.Parser.GetHeaderValue("Person-Id")
		headers["User-Id"] = w.Parser.GetLocalString("User-Id")
		finalPath := path
		for _, param := range w.Parser.GetUrlParams() {
			normalized := strings.ReplaceAll(param, "*", "/")
			finalPath += "/" + normalized
		}
		code, desc, arrayErr, req, reqLog, err := libRequest.GetRequest[Req](w, isJson)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}

		if hasInitializer {
			w.Parser.SetLocal("reqLog", &reqLog)
			reqLog.Incoming = req
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, title, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], w)
				return
			}
		}
		requestByte, _ := json.Marshal(req)
		status, descArray, resp, err := transmitter(finalPath, api, method, requestByte, headers, parseRemoteResp, core.Consumer().ConsumeRestApi)
		if err != nil {
			core.Responder().HandleErrorState(err, status, descArray["desc"], descArray["message"], w)
			return
		}
		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)
	}
}

// initializer func(c webFramework.WebFramework, method, url string, reqLog *libRequest.Request, args ...any) (int, map[string]string, error),
func InitPostRequest(
	ctx webFramework.WebFramework,
	reqLog *libRequest.Request,
	method, url string,
	checkDuplicate func(libRequest.Request) error,
	addEvent func(webFramework.WebFramework, string, string, string, *libRequest.Request),
	insertRequest func(libRequest.Request) error,
	args ...any,
) (int, map[string]string, error) {
	err := checkDuplicate(*reqLog)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	addEvent(ctx, reqLog.BranchId, method, "start", reqLog)
	err = insertRequest(*reqLog)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]string{"desc": "PWC_REGISTER", "message": "Unable To Register Request"}, err
	}
	var params []any
	for _, arg := range args {
		params = append(params, ctx.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

func ConsumeRemotePost(c any, reqLog *libRequest.Request, request any, method, methodName, api, url string,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	consumeHandler func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
	args ...any) (int, string, any, error) {
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

type WsResponse[Result any] struct {
	Status      int                      `json:"status"`
	Description string                   `json:"description"`
	Result      Result                   `json:"result,omitempty"`
	ErrorData   []response.ErrorResponse `json:"errors,omitempty"`
}

func CallApi[Resp any](
	w webFramework.WebFramework,
	core RequestCoreInterface,
	method string,
	param libCallApi.CallParam) (*Resp, *response.ErrorState) {
	var reqLog *libRequest.Request
	dump, err := json.MarshalIndent(param, "", "  ")
	if err == nil {
		reqLog = core.RequestTools().LogStart(w, method, string(dump))
	} else {
		reqLog = core.RequestTools().LogStart(w, method, fmt.Sprintf("params: %+v", param))
	}
	resp1 := libCallApi.Call[WsResponse[Resp]](param)
	dump, err = json.MarshalIndent(resp1, "", "  ")
	if err == nil {
		core.RequestTools().LogEnd(method, string(dump), reqLog)
	} else {
		core.RequestTools().LogEnd(method, fmt.Sprintf("resp: %+v", resp1), reqLog)
	}

	if resp1.Error != nil {
		return nil, resp1.Error
	}
	if resp1.Status.Status != http.StatusOK {
		return nil, resp1.WsResp.ToErrorState()
	}
	return &resp1.Resp.Result, nil
}

func CallHandler[Req any, Resp any](
	title, path, api, method, query string, isJson bool,
	hasInitializer bool,
	headers map[string]string,
	core RequestCoreInterface,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
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
		w.Parser.SetLocal("reqLog", &reqLog)
		reqLog.Incoming = req

		if hasInitializer {
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, title, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], w)
				return
			}
		}
		resp, errCall := CallApi[Resp](w, core, title,
			libCallApi.CallParam{
				Api:         core.Consumer().GetApi(api),
				Method:      method,
				Path:        finalPath,
				Query:       query,
				JsonBody:    req,
				ValidateTls: false,
				EnableLog:   false,
				Headers:     headers,
			})
		if errCall != nil {
			core.Responder().HandleErrorState(errCall, errCall.Status, errCall.Description, errCall.Message, w)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)
	}
}
