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

// ExtractValue extracts a value from source and stores it in dest, supporting header#alias syntax.
func ExtractValue(name string, source func(string) string, dest map[string]string) {
	if strings.Contains(name, "#") {
		headerSplit := strings.Split(name, "#")
		dest[headerSplit[1]] = source(headerSplit[0])
	} else {
		dest[name] = source(name)
	}
}

// ExtractHeaders builds a map of headers and locals from the web framework parser.
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

// CallArgs holds the arguments for a remote call handler.
type CallArgs[Req any, Resp any] struct {
	Title, Path, API, Method string
	HasQuery, IsJSON         bool
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

// Parameters returns the handler parameters for the CallArgs.
func (c CallArgs[Req, Resp]) Parameters() HandlerParameters[Req, Resp] {
	var mode libRequest.Type
	if c.IsJSON {
		mode = libRequest.JSON
	} else {
		mode = libRequest.Query
	}
	return HandlerParameters[Req, Resp]{
		Title:           c.Title,
		Body:            mode,
		ValidateHeader:  false,
		Persistence:     nil,
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
	// HeadersMap is the local key for the extracted headers map.
	HeadersMap = "headersMap"
	// FinalPath is the local key for the computed final request path.
	FinalPath = "finalPath"
)

// Initializer prepares headers and the final path for the remote call.
func (c CallArgs[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	if c.ForwardAuth {
		c.Headers = append(c.Headers, "Authorization")
	}
	headersMap := ExtractHeaders(req.W, c.Headers, c.Locals)
	if !c.ForwardAuth {
		remoteAPI := req.Core.Params().GetRemoteAPI(c.API)
		headersMap["Authorization"] = "Basic " + libCallApi.BasicAuth(remoteAPI.AuthData.User, remoteAPI.AuthData.Password)
	}
	req.W.Parser.SetLocal(HeadersMap, headersMap)

	finalPath := c.Path
	for _, value := range req.W.Parser.GetURLParams() {
		//normalized := strings.ReplaceAll(param.Value, "*", "/")
		finalPath += "/" + value //normalized
	}
	req.W.Parser.SetLocal(FinalPath, finalPath)
	return nil
}

// Handler performs the remote call for the CallArgs handler.
func (c CallArgs[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	finalPath := req.W.Parser.GetLocalString(FinalPath)
	headers, ok := req.W.Parser.GetLocal(HeadersMap).(map[string]string)
	if !ok {
		return req.Response, libError.NewWithDescription(
			http.StatusInternalServerError,
			"BAD_LOCAL_HEADERS",
			"unable to get headers, wrong data type: %T", req.W.Parser.GetLocal(HeadersMap))
	}

	resp, err := libCallApi.RemoteCall(req.W,
		&libCallApi.RemoteCallParamData[Req, Resp]{
			Headers:  headers,
			JSONBody: *req.Request,
			API:      *req.Core.Params().GetRemoteAPI(c.API),
			Method:   c.Method,
			Path:     finalPath,
			Parser:   req.W.Parser, // Pass parser for distributed tracing
		},
	)
	if err != nil {
		return req.Response, err
	}
	return *resp, nil
}

// Simulation returns the default response for simulation mode.
func (c CallArgs[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}

// Finalizer is a no-op finalizer for the CallArgs handler.
func (c CallArgs[Req, Resp]) Finalizer(_ HandlerRequest[Req, Resp]) {}

// CallRemote returns a base handler that forwards requests to a remote service.
func CallRemote[Req any, Resp any](
	core requestCore.RequestCoreInterface,
	callArg CallArgs[Req, Resp],
	simulation bool,
	args ...string,
) any {
	return BaseHandler(core, callArg, simulation, args)
}

// CallRemoteWithRespParser returns a base handler that forwards requests with a response parser.
func CallRemoteWithRespParser[Req any, Resp any](
	core requestCore.RequestCoreInterface,
	callArgs CallArgs[Req, Resp],
	simulation bool,
	args ...string,
) any {
	return BaseHandler(core, callArgs, simulation, args)
}

// InitPostRequest validates and persists a request, then builds the formatted path.
func InitPostRequest(
	w webFramework.WebFramework,
	reqLog libRequest.RequestPtr,
	_, url string,
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
		params = append(params, w.Parser.GetURLParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

// ConsumeHandlerType holds configuration for a consume handler that proxies remote API calls.
type ConsumeHandlerType[Req, Resp any] struct {
	Title           string
	Params          libCallApi.RemoteCallParamData[Req, Resp]
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	HasReceipt      bool
	Headers         []string
	API             string
	Method          string
	Query           string
	RecoveryHandler func(any)
}

// Parameters returns the handler parameters for the ConsumeHandlerType.
func (h *ConsumeHandlerType[Req, Resp]) Parameters() HandlerParameters[Req, Resp] {
	return HandlerParameters[Req, Resp]{
		Title:           h.Title,
		Body:            h.Mode,
		ValidateHeader:  h.VerifyHeader,
		Persistence:     nil,
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

// Initializer appends URL parameters to the handler path.
func (h *ConsumeHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	for _, value := range req.W.Parser.GetURLParams() {
		//normalized := strings.ReplaceAll(param.Value, "*", "/")
		h.Path += "/" + value //normalized
	}
	return nil
}

// Handler performs the remote JSON call for the ConsumeHandlerType.
func (h *ConsumeHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	headersMap := ExtractHeaders(req.W, h.Headers, nil)
	resp, errCall := CallAPIJSON(req.W, req.Core, h.Title,
		&libCallApi.RemoteCallParamData[Req, Resp]{
			API:         *req.Core.Params().GetRemoteAPI(h.API),
			Method:      h.Method,
			Path:        h.Path,
			Query:       h.Query,
			JSONBody:    *req.Request,
			ValidateTLS: false,
			EnableLog:   false,
			Headers:     headersMap,
			Builder:     req.Builder,
			Parser:      req.W.Parser, // Pass parser for distributed tracing
		})
	if errCall != nil {
		return req.Response, errCall
	}
	return resp, nil
}

// Simulation returns the default response for simulation mode.
func (h *ConsumeHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}

// Finalizer is a no-op finalizer for the ConsumeHandlerType.
func (h *ConsumeHandlerType[Req, Resp]) Finalizer(_ HandlerRequest[Req, Resp]) {}

// ConsumeHandler returns a base handler that consumes a remote API endpoint.
func ConsumeHandler[Req, Resp any](
	core requestCore.RequestCoreInterface,
	params *ConsumeHandlerType[Req, Resp],
	simulation bool,
) any {
	return BaseHandler(core, params, simulation)
}
