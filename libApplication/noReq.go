package initiator

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/webFramework"
)

type NoReq struct {
}

func (n NoReq) Initialize(
	_ webFramework.WebFramework, _, _ string, _ libRequest.RequestPtr, _ ...any) (
	int, map[string]string, error) {
	return http.StatusOK, nil, nil
}
func (n NoReq) InitRequest(_ webFramework.WebFramework, _, _ string) error {
	return nil
}
func (n NoReq) InitializeNoLog(
	_ webFramework.WebFramework, _, _ string, _ libRequest.RequestPtr, _ ...any) (
	int, map[string]string, error) {
	return http.StatusOK, nil, nil
}
func (n NoReq) AddRequestLog(_, _ string, _ libRequest.RequestPtr) {
}
func (n NoReq) LogEnd(_, _ string, _ libRequest.RequestPtr) {
}
func (n NoReq) AddRequestEvent(_ webFramework.WebFramework, _, _, _ string, _ libRequest.RequestPtr) {
}
func (n NoReq) LogStart(_ webFramework.WebFramework, _, _ string) libRequest.RequestPtr {
	return nil
}
func (n NoReq) InsertRequest(_ libRequest.RequestPtr) error {
	return nil
}
func (n NoReq) CheckDuplicateRequest(_ libRequest.RequestPtr) error {
	return nil
}
func (n NoReq) UpdateRequestWithContext(_ context.Context, _ libRequest.RequestPtr) error {
	return nil
}
