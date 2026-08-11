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

// Initialize checks for duplicate requests, inserts the request, and builds the formatted path.
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
		params = append(params, w.Parser.GetURLParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

// InitializeNoLog builds the formatted path without duplicate checking or request insertion.
func (m RequestModel) InitializeNoLog(w webFramework.WebFramework, _, url string, _ RequestPtr, args ...any) (int, map[string]string, error) {
	var params []any
	for _, arg := range args {
		params = append(params, w.Parser.GetURLParam(arg.(string)))
	}
	path := fmt.Sprintf(url, params...)
	return http.StatusOK, map[string]string{"path": path}, nil
}

// InitRequest checks for duplicates and inserts the request from the parser's local storage.
func (m RequestModel) InitRequest(w webFramework.WebFramework, method, _ string) error {
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
			"duplicate request"))
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
