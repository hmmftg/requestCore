package libContext

import (
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

// AddWebHandlerLogs initializes a context from the given framework context and returns a logging closure.
func AddWebHandlerLogs(c any, title, tag string) func(time.Time, int) {
	w := InitContextNoAuditTrail(c)
	return AddWebLogs(w, title, tag)
}

// AddWebLogs adds request metadata as log tags and returns a closure that logs elapsed time and status.
func AddWebLogs(w webFramework.WebFramework, title, tag string) func(time.Time, int) {
	webFramework.AddLogTag(w, tag, slog.String("title", title))
	webFramework.AddLogTag(w, tag, slog.String("method", w.Parser.GetMethod()))
	webFramework.AddLogTag(w, tag, slog.String("path", w.Parser.GetPath()))
	return func(start time.Time, status int) {
		elapsed := time.Since(start)
		webFramework.AddLogTag(w, tag, slog.String("elapsed", elapsed.String()))
		webFramework.AddLogTag(w, tag, slog.Int("status", status))
		webFramework.CollectLogTags(w, tag)
		webFramework.CollectLogArrays(w, tag)
	}
}

// AddMiddlewareLogs adds request metadata as direct logs and returns a closure that logs elapsed time and status.
func AddMiddlewareLogs(w webFramework.WebFramework, title, tag string) func(time.Time, int) {
	webFramework.AddLog(w, tag, slog.String("title", title))
	webFramework.AddLog(w, tag, slog.String("method", w.Parser.GetMethod()))
	webFramework.AddLog(w, tag, slog.String("path", w.Parser.GetPath()))
	return func(start time.Time, status int) {
		elapsed := time.Since(start)
		webFramework.AddLog(w, tag, slog.String("elapsed", elapsed.String()))
		webFramework.AddLog(w, tag, slog.Int("status", status))
		webFramework.CollectLogArrays(w, tag)
	}
}
