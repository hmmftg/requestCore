package libCallApi

import (
	"bytes"
	"crypto/tls"
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

	"github.com/hmmftg/requestCore/response"
)

func (m RemoteApiModel) ConsumeRestBasicAuthApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error) {
	timeout := time.Duration(30 * time.Second)
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", err
	}
	req.SetBasicAuth(m.RemoteApiList[apiName].User, m.RemoteApiList[apiName].Password)
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "#" + m.RemoteApiList[apiName].Name + "#", err
	}

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
		return nil, errorDesc, fmt.Errorf(errorDesc)
	}

	return responseData, resp.Status, nil
}

func (m RemoteApiModel) GetApi(apiName string) RemoteApi {
	return m.RemoteApiList[apiName]
}

func (m RemoteApiModel) ConsumeRestApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error) {
	timeout := time.Duration(30 * time.Second)
	if timeOutString, ok := headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	req, err := http.NewRequest(method, m.RemoteApiList[apiName].Domain+"/"+path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, "Generate Request Failed", http.StatusInternalServerError, err
	}
	if _, ok := headers["Authorization"]; !ok {
		req.SetBasicAuth(m.RemoteApiList[apiName].User, m.RemoteApiList[apiName].Password)
	}
	req.Header.Add("Content-Type", contentType)
	for header, value := range headers {
		req.Header.Add(header, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "API_CONNECT_TIMED_OUT#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
		}
		return nil, "API_UNABLE_TO_CALL#" + apiName + "# " + m.RemoteApiList[apiName].Name + "#", http.StatusRequestTimeout, err
	}

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
		return nil, errorDesc, resp.StatusCode, fmt.Errorf(errorDesc)
	}

	return responseData, resp.Status, resp.StatusCode, nil
}

type CallData struct {
	Api       RemoteApi
	Path      string
	Method    string
	Headers   map[string]string
	Req       any
	SslVerify bool
	Timeout   time.Duration
	EnableLog bool
	LogLevel  int
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

func GetJSONResp[Resp ApiResp](api RemoteApi, resp *http.Response) (*Resp, response.ErrorState) {
	respJsonP := new(Resp)
	respJson := *respJsonP
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, response.Error(http.StatusRequestTimeout, "API_READ_TIMED_OUT", api.Name, err).Input("GetResp.ReadAll")
		}
		return nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_READ", api.Name, err).Input("GetResp.ReadAll")
	}
	respJson.SetStatus(resp.StatusCode)
	err = json.Unmarshal(responseData, respJsonP)
	if err != nil {
		return nil, response.Error(resp.StatusCode, "API_OK_RESP_JSON", api.Name, err).Input(fmt.Sprintf("GetResp.Unmarshal:%s", string(responseData)))
	}
	headerMap := make(map[string]string, 0)
	for key, header := range resp.Header {
		headerMap[key] = header[0]
	}
	respJson.SetHeaders(headerMap)
	return respJsonP, nil
}

func PrepareCall(c CallData) (*http.Request, response.ErrorState) {
	if timeOutString, ok := c.Headers["Time-Out"]; ok {
		timeoutSeconds, _ := strconv.Atoi(timeOutString)
		c.Timeout = time.Duration(timeoutSeconds * int(time.Second))
	}
	requestJson, err := json.Marshal(c.Req)
	if err != nil {
		return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", c.Req, err).Input(fmt.Sprintf("PrepareCall.Marshal:%v", c.Req))
	}
	req, err := http.NewRequest(c.Method, c.Api.Domain+"/"+c.Path, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, response.Error(http.StatusInternalServerError, "Generate Request Failed", fmt.Sprintf("M=%s,Url:%s,json:%s", c.Method, c.Api.Domain+"/"+c.Path, string(requestJson)), err).Input(fmt.Sprintf("PrepareCall.NewRequest:%v", c))
	}
	if _, ok := c.Headers["Authorization"]; !ok {
		req.SetBasicAuth(c.Api.User, c.Api.Password)
	}
	req.Header.Add("Content-Type", "application/json")
	for header, value := range c.Headers {
		req.Header.Add(header, value)
	}

	return req, nil
}

func (c CallData) SetLogs(req *http.Request) *http.Request {
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

func ConsumeRest[Resp any](c CallData) (*Resp, *response.WsRemoteResponse, *CallResp, response.ErrorState) {
	if c.Timeout == 0 {
		c.Timeout = time.Duration(30 * time.Second)
	}

	req, errPrepare := PrepareCall(c)
	if errPrepare != nil {
		return nil, nil, nil, errPrepare.Input(c)
	}

	client := &http.Client{
		Timeout: c.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.SslVerify},
		}}

	if c.EnableLog {
		req = c.SetLogs(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
		}
		return nil, nil, nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
	}
	var respJson *Resp
	var errResp *response.WsRemoteResponse

	respJson, errResp, callResp, errParse := GetResp[Resp, response.WsRemoteResponse](c.Api, resp)
	if errParse != nil {
		return nil, nil, callResp, errParse.Input(resp)
	}

	return respJson, errResp, callResp, nil
}

func ConsumeRestJSON[Resp ApiResp](c *CallData) (*Resp, response.ErrorState) {
	if c.Timeout == 0 {
		c.Timeout = time.Duration(30 * time.Second)
	}

	req, errPrepare := PrepareCall(*c)
	if errPrepare != nil {
		return nil, errPrepare.Input(c)
	}

	client := &http.Client{
		Timeout: c.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.SslVerify},
		}}

	if c.EnableLog {
		req = c.SetLogs(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, response.Error(http.StatusRequestTimeout, "API_CONNECT_TIMED_OUT", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
		}
		return nil, response.Error(http.StatusRequestTimeout, "API_UNABLE_TO_CALL", c, err).Input(fmt.Sprintf("ConsumeRest.ClientDo:%v", req))
	}

	respJson, errParse := GetJSONResp[Resp](c.Api, resp)
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
	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(url, "text/xml", bytes.NewBuffer(requestBytes))
	if err != nil {
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
