package libRequest

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/webFramework"
)

// LogRequest is a no-op RequestInterface implementation that logs requests via slog.
type LogRequest struct {
}

// Initialize returns the URL path without performing any request initialization.
func (l LogRequest) Initialize(_ webFramework.WebFramework, _, url string, _ RequestPtr, _ ...any) (int, map[string]string, error) {
	return http.StatusOK, map[string]string{"path": url}, nil
}

// InitRequest is a no-op that always returns nil.
func (l LogRequest) InitRequest(_ webFramework.WebFramework, _, _ string) error {
	return nil
}

// InitializeNoLog returns the URL path without performing any request initialization.
func (l LogRequest) InitializeNoLog(_ webFramework.WebFramework, _, url string, _ RequestPtr, _ ...any) (int, map[string]string, error) {
	return http.StatusOK, map[string]string{"path": url}, nil

}

// InsertRequest logs the request via slog.
func (l LogRequest) InsertRequest(req RequestPtr) error {
	slog.Info("LogStart",
		slog.Any("req", req),
	)
	return nil
}

// CheckDuplicateRequest is a no-op that always returns nil.
func (l LogRequest) CheckDuplicateRequest(_ RequestPtr) error {
	return nil
}

// UpdateRequestWithContext logs the request completion via slog.
func (l LogRequest) UpdateRequestWithContext(_ context.Context, req RequestPtr) error {
	slog.Info("LogEnd",
		slog.Any("req", req),
	)
	return nil
}
