package libCallApi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"strconv"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/hmmftg/requestCore/response"
)

func (m RemoteApiModel) ConsumeRestBasicAuthApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", err
	}
	req.SetBasicAuth(m.RemoteApiList[apiName].AuthData.User, m.RemoteApiList[apiName].AuthData.Password)
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
		return nil, errorDesc, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, nil
}

func (m RemoteApiModel) GetApi(apiName string) RemoteApi {
	return m.RemoteApiList[apiName]
}

func (m RemoteApiModel) ConsumeRestApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error) {
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		httpClient.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", http.StatusInternalServerError, err
	}
	if _, ok := headers["Authorization"]; !ok {
		req.SetBasicAuth(m.RemoteApiList[apiName].AuthData.User, m.RemoteApiList[apiName].AuthData.Password)
	}
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_READ_TIMED_OUT#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_READ#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
	}

	if resp.StatusCode != http.StatusOK {
		var respJson response.WsRemoteResponse
		if json.Unmarshal(responseData, &respJson) == nil {
			return responseData, resp.Status, resp.StatusCode, nil
		}
		errorDesc := fmt.Sprintf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
		return nil, errorDesc, resp.StatusCode, fmt.Errorf("API_NOK#%s#%s#%s#", apiName, m.RemoteApiList[apiName].Name, resp.Status)
	}

	return responseData, resp.Status, resp.StatusCode, nil
}

type RequestBodyType int

const (
	JSON RequestBodyType = iota
	Form
	Empty
)

type CallData[Resp any] struct {
	httpClient *http.Client
	Api        RemoteApi
	Path       string
	Method     string
	Headers    map[string]string
	Req        any
	SslVerify  bool
	BodyType   RequestBodyType
	Timeout    time.Duration
	EnableLog  bool
	LogLevel   int
	Builder    func(int, []byte, map[string]string) (*Resp, response.ErrorState)
}

type CallResp struct {
	Headers map[string]string
	Status  int
}

func GetResp[Resp any, Error any](api RemoteApi, resp *http.Response) (*Resp, *Error, *CallResp, response.ErrorState) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_READ_TIMED_OUT", api.Name, err).Input("GetResp.ReadAll")
		}
		return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_READ", api.Name, err).Input("GetResp.ReadAll")
	}
	var respJson Resp
	var errJson Error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.Unmarshal(responseData, &respJson)
		if err != nil {
			return nil, nil, nil, response.Error(http.StatusInternalServerError, "API_OK_RESP_JSON", api.Name, err).Input(fmt.Sprintf("GetResp.Unmarshal:%s", string(responseData)))
		}
	default:
		err = json.Unmarshal(responseData, &errJson)
		if err != nil {
			return nil, nil, nil, response.Error(resp.StatusCode, "API_NOK_RESP_JSON", api.Name, err).Input(fmt.Sprintf("GetResp.Unmarshal:%s", string(responseData)))
		}
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	return &respJson, &errJson, &CallResp{Status: resp.StatusCode, Headers: headerMap}, nil
}

func GetJSONResp[Resp any](api RemoteApi, resp *http.Response, Builder func(int, []byte, map[string]string) (*Resp, response.ErrorState)) (*Resp, response.ErrorState) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, response.Error(http.StatusRequestTimeout, "API_READ_TIMED_OUT", api.Name, err).Input("GetResp.ReadAll")
		}
		return nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_READ", api.Name, err).Input("GetResp.ReadAll")
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	if Builder != nil {
		return Builder(resp.StatusCode, responseData, headerMap)
	}

	var jsonResp Resp
	err = json.Unmarshal(responseData, &jsonResp)
	if err != nil {
		return nil, response.Error(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", responseData, err).Input("GetResp.json.Unmarshal")
	}
	return &jsonResp, nil
}

func PrepareCall[Resp any](c CallData[Resp]) (*http.Request, response.ErrorState) {
	var to time.Duration
	if timeOutString, ok := c.Headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		to = time.Duration(timeoutSeconds * int(time.Second))
	} else if c.Timeout > 0 {
		to = c.Timeout
	} else {
		to = defaultTimeOut
	}
	if c.httpClient == nil {
		httpClient.Timeout = to
	} else {
		c.httpClient.Timeout = to
	}
	var buffer *bytes.Buffer
	switch c.BodyType {
	case JSON:
		jString, err := json.Marshal(c.Req)
		if err != nil {
			return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", c.Req, err).Input(fmt.Sprintf("PrepareCall.Marshal:%v", c.Req))
		}
		buffer = bytes.NewBuffer(jString)
	case Form:
		form, err := query.Values(c.Req)
		if err != nil {
			return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", c.Req, err).Input(fmt.Sprintf("PrepareCall.Marshal:%v", c.Req))
		}
		buffer = bytes.NewBuffer([]byte(form.Encode()))
	case Empty:
		buffer = bytes.NewBuffer([]byte(""))
	}
	if buffer == nil {
		return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", c.Req, fmt.Errorf("type is not defined"))
	}
	req, err := http.NewRequest(c.Method, c.Api.Domain+"/"+c.Path, buffer)
	if err != nil {
		return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", fmt.Sprintf("M=%s,Url:%s,json:%s", c.Method, c.Api.Domain+"/"+c.Path, buffer.String()), err).Input(fmt.Sprintf("PrepareCall.NewRequest:%v", c))
	}
	if _, ok := c.Headers["Authorization"]; !ok {
		req.SetBasicAuth(c.Api.AuthData.User, c.Api.AuthData.Password)
	}
	switch c.BodyType {
	case JSON:
		req.Header.Add("Content-Type", "application/json")
	case Form:
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Add("Accept", "application/json")
	for header, value := range c.Headers {
		req.Header.Add(header, value)
	}

	return req, nil
}

func (c CallData[Resp]) SetLogs(req *http.Request) *http.Request {
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			log.Println("Got Conn:", connInfo)
		},
		ConnectStart: func(network, addr string) {
			log.Println("Dial start:", network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			log.Println("Dial done:", network, addr, err)
		},
		GotFirstResponseByte: func() {
			log.Println("First response byte!")
		},
		WroteHeaders: func() {
			log.Println("Wrote headers")
		},
		WroteRequest: func(wr httptrace.WroteRequestInfo) {
			log.Println("Wrote request", wr)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	log.Println("Starting request!")
	return req
}

func ConsumeRest[Resp any](c CallData[Resp]) (*Resp, *response.WsRemoteResponse, *CallResp, response.ErrorState) {
	req, errPrepare := PrepareCall(c)
	if errPrepare != nil {
		return nil, nil, nil, errPrepare.Input(c)
	}

	cl := httpClient
	if c.httpClient != nil {
		cl = c.httpClient
	}

	if c.EnableLog {
		req = c.SetLogs(req)
	}
	resp, err := cl.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
		}
		return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
	}
	defer resp.Body.Close()

	var respJson *Resp
	var errResp *response.WsRemoteResponse

	respJson, errResp, callResp, errParse := GetResp[Resp, response.WsRemoteResponse](c.Api, resp)
	if errParse != nil {
		return nil, nil, callResp, errParse.Input(resp)
	}

	return respJson, errResp, callResp, nil
}

func DefaultBuilderfunc[Resp any](status int, rawResp []byte, headers map[string]string) (*Resp, response.ErrorState) {
	if status != http.StatusOK {
		return nil, response.Error(status, "API_RESP_NOK", rawResp, fmt.Errorf("request failed, status %d", status))
	}
	var resp Resp
	err := json.Unmarshal(rawResp, &resp)
	if err != nil {
		return nil, response.Error(http.StatusBadRequest, "API_UNABLE_PARSE_RESP", rawResp, err).Input("GetResp.json.Unmarshal")
	}
	return &resp, nil
}

func ConsumeRestJSON[Resp any](c *CallData[Resp]) (*Resp, response.ErrorState) {
	req, errPrepare := PrepareCall(*c)
	if errPrepare != nil {
		return nil, errPrepare.Input(c)
	}

	cl := httpClient
	if c.httpClient != nil {
		cl = c.httpClient
	}

	if c.EnableLog {
		req = c.SetLogs(req)
	}
	resp, err := cl.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, response.Error(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
		}
		return nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
	}
	defer resp.Body.Close()

	if c.Builder == nil {
		c.Builder = DefaultBuilderfunc[Resp]
	}

	respJson, errParse := GetJSONResp(c.Api, resp, c.Builder)
	if errParse != nil {
		return nil, errParse.Input(resp)
	}

	return respJson, nil
}

func TransmitRequestWithAuth(
	path, api, method string,
	requestByte []byte,
	headers map[string]string,
	parseRemoteResp func([]byte, string, int) (int, map[string]string, any, error),
	consumeHandler func([]byte, string, string, string, string, map[string]string) ([]byte, string, int, error),
) (int, map[string]string, any, error) {
	var resp response.WsRemoteResponse
	respBytes, desc, status, err := consumeHandler(requestByte, api, path, "application/json", method, headers)
	if err != nil {
		return status, map[string]string{"desc": desc, "message": desc}, resp, err
	}
	status, result, respApi, err := parseRemoteResp(respBytes, desc, status)
	if err != nil || status != http.StatusOK {
		return status, result, respApi, err
	}
	return http.StatusOK, nil, respApi, nil
}

func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func TransmitSoap[Resp any](request any, url string, debug bool, timeout time.Duration) (*Resp, error) {
	requestBytes, _ := xml.MarshalIndent(&request, " ", "  ")
	req, requestErr := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewBuffer(requestBytes),
	)
	if requestErr != nil {
		return nil, requestErr
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, err
		}
		return nil, err
	}
	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debug {
		log.Println(string(result))
	}
	var respXml Resp
	err = xml.Unmarshal(result, &respXml)
	if err != nil {
		return nil, err
	}
	return &respXml, nil
}
