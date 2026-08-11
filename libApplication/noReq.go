package initiator

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/webFramework"
)

// NoReq is a no-op implementation of RequestInterface that skips request tracking.
type NoReq struct {
}

// Initialize is a no-op that returns success without tracking the request.
func (n NoReq) Initialize(
	_ webFramework.WebFramework, _, _ string, _ libRequest.RequestPtr, _ ...any) (
	int, map[string]string, error) {
	return http.StatusOK, nil, nil
}

// InitRequest is a no-op that returns nil without initializing a request.
func (n NoReq) InitRequest(_ webFramework.WebFramework, _, _ string) error {
	return nil
}

// InitializeNoLog is a no-op that returns success without logging the request.
func (n NoReq) InitializeNoLog(
	_ webFramework.WebFramework, _, _ string, _ libRequest.RequestPtr, _ ...any) (
	int, map[string]string, error) {
	return http.StatusOK, nil, nil
}

// AddRequestLog is a no-op that does not add any request log.
func (n NoReq) AddRequestLog(_, _ string, _ libRequest.RequestPtr) {
}

// LogEnd is a no-op that does not log the request end.
func (n NoReq) LogEnd(_, _ string, _ libRequest.RequestPtr) {
}

// AddRequestEvent is a no-op that does not add a request event.
func (n NoReq) AddRequestEvent(_ webFramework.WebFramework, _, _, _ string, _ libRequest.RequestPtr) {
}

// LogStart is a no-op that returns nil without logging the request start.
func (n NoReq) LogStart(_ webFramework.WebFramework, _, _ string) libRequest.RequestPtr {
	return nil
}

// InsertRequest is a no-op that returns nil without inserting the request.
func (n NoReq) InsertRequest(_ libRequest.RequestPtr) error {
	return nil
}

// CheckDuplicateRequest is a no-op that returns nil without checking for duplicates.
func (n NoReq) CheckDuplicateRequest(_ libRequest.RequestPtr) error {
	return nil
}

// UpdateRequestWithContext is a no-op that returns nil without updating the request.
func (n NoReq) UpdateRequestWithContext(_ context.Context, _ libRequest.RequestPtr) error {
	return nil
}
