package libRequest

import (
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

func (m RequestModel) LogStart(w webFramework.WebFramework, method, log string) RequestPtr {
	r := w.Parser.GetLocal("reqLog")
	if r != nil {
		reqLog := r.(RequestPtr)
		branch := w.Parser.GetLocal("branchId")
		if branch == nil {
			branch = ""
		}
		m.AddRequestEvent(w, branch.(string), method, log, reqLog)
		return reqLog
	}
	return &Request{ActionId: "NONE"}
}

func (m RequestModel) LogEnd(method, log string, r RequestPtr) {
	if r.ActionId != "NONE" {
		m.AddRequestLog(method, log, r)
	}
}

func (m RequestModel) AddRequestLog(method, log string, req RequestPtr) {
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

func (m RequestModel) AddLogEvent(method, log string, req RequestPtr) {
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

func (m RequestModel) AddRequestEvent(w webFramework.WebFramework, branch, method, log string, req RequestPtr) {
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
