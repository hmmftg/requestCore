package libLogger

import (
	"log/slog"
)

const (
	SlogRequestBody  = "slog.request.body"
	SlogResponseBody = "slog.response.body"
)

type JsonLogger struct {
	logger *slog.Logger
}

func (j JsonLogger) Write(p []byte) (n int, err error) {
	if p[len(p)-1] == '\n' {
		j.logger.Info(string(p[:len(p)-1]))
		return len(p), nil
	}
	j.logger.Info(string(p))
	return len(p), nil
}
