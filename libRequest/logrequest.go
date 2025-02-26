package libRequest

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/webFramework"
)

type LogRequest struct {
}

func (l LogRequest) Initialize(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
	return http.StatusOK, map[string]string{"path": url}, nil
}
func (l LogRequest) InitRequest(c webFramework.WebFramework, method, url string) error {
	return nil
}
func (l LogRequest) InitializeNoLog(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error) {
	return http.StatusOK, map[string]string{"path": url}, nil

}
func (l LogRequest) InsertRequest(req RequestPtr) error {
	slog.Info("LogStart",
		slog.Any("req", req),
	)
	return nil
}
func (l LogRequest) CheckDuplicateRequest(request RequestPtr) error {
	return nil
}
func (l LogRequest) UpdateRequestWithContext(ctx context.Context, req RequestPtr) error {
	slog.Info("LogEnd",
		slog.Any("req", req),
	)
	return nil
}
