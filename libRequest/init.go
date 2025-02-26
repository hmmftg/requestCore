package libRequest

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

func (m RequestModel) Initialize(w webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
	err := m.CheckDuplicateRequest(req)
	if err != nil {
		if ok, err := response.Unwrap(err); ok {
			src := err.GetInput().(string)
			if src == "DB" {
				return http.StatusBadRequest, map[string]string{"desc": "PWC_REGISTER", "message": "unable to CheckDuplicateRequest"}, err
			}
		}
		return http.StatusBadRequest, map[string]string{"desc": "DUPLICATE_REQUEST", "message": "Duplicate Request"}, err
	}
	prg, mdl := m.QueryInterface.GetModule()
	req.Header.SetProgram(prg)
	req.Header.SetModule(mdl)
	req.Header.SetUser(w.Parser.GetLocalString("userId"))
	req.Header.SetMethod(method)
	err = m.InsertRequestWithContext(w.Ctx, req)
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

func (m RequestModel) InitializeNoLog(w webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
	var params []any
	for _, arg := range args {
		params = append(params, w.Parser.GetUrlParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

func (m RequestModel) InitRequest(w webFramework.WebFramework, method, url string) error {
	reqL := w.Parser.GetLocal("reqLog")
	req := reqL.(RequestPtr)
	err := m.CheckDuplicateRequest(req)
	if err != nil {
		if ok, err := response.Unwrap(err); ok {
			src := err.GetInput().(string)
			if src == "DB" {
				return err
			}
		}
		return errors.Join(err, libError.NewWithDescription(
			status.BadRequest,
			"DUPLICATE_REQUEST",
			"dupicate request"))
	}
	prg, mdl := m.QueryInterface.GetModule()
	req.Header.SetProgram(prg)
	req.Header.SetModule(mdl)
	req.Header.SetUser(w.Parser.GetLocalString("userId"))
	req.Header.SetMethod(method)
	err = m.InsertRequestWithContext(w.Ctx, req)
	if err != nil {
		return errors.Join(err, libError.NewWithDescription(
			status.InternalServerError,
			"PWC_REGISTER",
			"unable To Register Request"))
	}
	return nil
}
