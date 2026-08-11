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
func (l LogRequest) Initialize(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
	return http.StatusOK, map[string]string{"path": url}, nil
}

// InitRequest is a no-op that always returns nil.
func (l LogRequest) InitRequest(c webFramework.WebFramework, method, url string) error {
	return nil
}

// InitializeNoLog returns the URL path without performing any request initialization.
func (l LogRequest) InitializeNoLog(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
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
func (l LogRequest) CheckDuplicateRequest(request RequestPtr) error {
	return nil
}

// UpdateRequestWithContext logs the request completion via slog.
func (l LogRequest) UpdateRequestWithContext(ctx context.Context, req RequestPtr) error {
	slog.Info("LogEnd",
		slog.Any("req", req),
	)
	return nil
}
