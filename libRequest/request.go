package libRequest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

func (m RequestModel) Initialize(c webFramework.WebFramework, method, url string, req *Request, args ...any) (int, map[string]string, error) {
	err := m.CheckDuplicateRequest(*req)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	m.AddRequestEvent(c, req.BranchId, method, "start", req)
	prg, mdl := m.QueryInterface.GetModule()
	req.Header.SetProgram(prg)
	req.Header.SetModule(mdl)
	req.Header.SetUser(c.Parser.GetLocalString("userId"))
	req.Header.SetMethod(method)
	m.InsertRequest(*req)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]string{"desc": "PWC_REGISTER", "message": "Unable To Register Request"}, err
	}
	var params []any
	for _, arg := range args {
		params = append(params, c.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

func (m RequestModel) InitializeNoLog(c webFramework.WebFramework, method, url string, req *Request, args ...any) (int, map[string]string, error) {
	m.AddRequestEvent(c, req.BranchId, method, "start", req)
	var params []any
	for _, arg := range args {
		params = append(params, c.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

func (m RequestModel) LogStart(w webFramework.WebFramework, method, log string) *Request {
	r := w.Parser.GetLocal("reqLog")
	if r != nil {
		reqLog := r.(*Request)
		branch := w.Parser.GetLocal("branchId").(string)
		m.AddRequestEvent(w, branch, method, log, reqLog)
		return reqLog
	}
	return &Request{ActionId: "NONE"}
}

func (m RequestModel) LogEnd(method, log string, r *Request) {
	if r.ActionId != "NONE" {
		m.AddRequestLog(method, log, r)
	}
}

func (m RequestModel) AddRequestLog(method, log string, req *Request) {
	programName, moduleName := m.QueryInterface.GetModule()
	logData := LogData{
		Time:    time.Now(),
		Program: programName,
		Module:  moduleName,
		Method:  method,
		LogText: log,
	}
	if len(req.Events) == 0 {
		m.AddLogEvent(method, log, req)
		return
	}
	lastEventId := len(req.Events) - 1
	if lastEventId < 0 {
		lastEventId = 0
	}
	req.Events[lastEventId].Logs = append(req.Events[lastEventId].Logs, logData)
}

func (m RequestModel) AddLogEvent(method, log string, req *Request) {
	programName, moduleName := m.QueryInterface.GetModule()
	event := EventData{
		Time: time.Now(),
		Logs: []LogData{
			{
				Time:    time.Now(),
				Program: programName,
				Module:  moduleName,
				Method:  method,
				LogText: log,
			},
		},
	}
	req.Events = append(req.Events, event)
}

func (m RequestModel) AddRequestEvent(w webFramework.WebFramework, branch, method, log string, req *Request) {
	programName, moduleName := m.QueryInterface.GetModule()
	event := EventData{
		Time:     time.Now(),
		ActionId: w.Parser.GetLocalString("action"),
		BranchId: branch,
		UserId:   w.Parser.GetLocalString("userId"),
		Logs: []LogData{
			{
				Time:    time.Now(),
				Program: programName,
				Module:  moduleName,
				Method:  method,
				LogText: log,
			},
		},
	}
	req.Events = append(req.Events, event)
}

func (m RequestModel) InsertRequest(request Request) error {
	rowByte, err := json.Marshal(request)
	if err != nil {
		return err
	}
	row := string(rowByte)
	args := make([]any, 5)
	args[0] = request.Header.GetUser()
	args[1] = request.Header.GetProgram()
	args[2] = request.Header.GetModule()
	args[3] = request.Header.GetMethod()
	args[4] = row
	if strings.Contains(m.InsertInDb, "$6") {
		args = append(args, request.Req)
	}
	ret, msg, err := m.QueryInterface.CallDbFunction(
		m.InsertInDb,
		args...,
	)
	if err != nil {
		log.Println("InsertNewRequest(", row, ")=>", ret, msg, err)
		return err
	}
	return nil
}

func (m RequestModel) CheckDuplicateRequest(request Request) error {
	ret, result, err := m.QueryInterface.QueryRunner(m.QueryInDb, request.Header.GetId())
	if err != nil {
		return err
	}
	if ret != 0 {
		if len(result) > 0 {
			return fmt.Errorf("query(%s)=>%d", request.Header.GetId(), ret)
		}
		return fmt.Errorf("query(%s)=>%d,%s", request.Header.GetId(), ret, result[0])
	}
	if len(result) > 0 {
		return fmt.Errorf("duplicate Request: id: %s", request.Header.GetId())
	}
	return nil
}

func (m RequestModel) UpdateRequest(request Request) error {
	programName, moduleName := m.QueryInterface.GetModule()
	requestBytes, _ := json.Marshal(request)
	args := make([]any, 6)
	args[0] = request.UserId
	args[1] = programName
	args[2] = moduleName
	args[3] = "UpdateRequest"
	args[4] = string(requestBytes)
	args[5] = request.Id
	if strings.Contains(m.UpdateInDb, "$7") {
		args = append(args, request.Resp)
	}
	ret, msg, err := m.QueryInterface.CallDbFunction(
		m.UpdateInDb,
		args...,
	)
	if err != nil {
		log.Println("UpdateRequest(", request.Id, string(requestBytes), ")=>", ret, msg, err)
		return err
	}
	return nil
}
