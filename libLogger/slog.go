package libLogger

import (
	"log/slog"
)

const (
	// SlogRequestBody is the slog attribute key for request body logging.
	SlogRequestBody = "slog.request.body"
	// SlogResponseBody is the slog attribute key for response body logging.
	SlogResponseBody = "slog.response.body"
)

// JsonLogger is a logger that writes structured JSON log entries via slog.
type JsonLogger struct {
	logger *slog.Logger
}

// Write logs the given bytes as an info-level slog message, implementing io.Writer.
func (j JsonLogger) Write(p []byte) (n int, err error) {
	if p[len(p)-1] == '\n' {
		j.logger.Info(string(p[:len(p)-1]))
		return len(p), nil
	}
	j.logger.Info(string(p))
	return len(p), nil
}
