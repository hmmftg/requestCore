package libRequest

import (
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/webFramework"
)

func (m RequestModel) Initialize(w webFramework.WebFramework, method, url string, req *Request, args ...any) (int, map[string]string, error) {
	err := m.CheckDuplicateRequest(*req)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	m.AddRequestEvent(w, req.BranchId, method, "start", req)
	prg, mdl := m.QueryInterface.GetModule()
	req.Header.SetProgram(prg)
	req.Header.SetModule(mdl)
	req.Header.SetUser(w.Parser.GetLocalString("userId"))
	req.Header.SetMethod(method)
	err = m.InsertRequestWithContext(w.Ctx, *req)
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

func (m RequestModel) InitializeNoLog(c webFramework.WebFramework, method, url string, req *Request, args ...any) (int, map[string]string, error) {
	m.AddRequestEvent(c, req.BranchId, method, "start", req)
	var params []any
	for _, arg := range args {
		params = append(params, c.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}
