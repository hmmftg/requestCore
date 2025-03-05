package libContext

import (
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

func AddWebHandlerLogs(c any, title, tag string) func(time.Time, int) {
	w := InitContextNoAuditTrail(c)
	return AddWebLogs(w, title, tag)
}

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
