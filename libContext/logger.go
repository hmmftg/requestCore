package libContext

import (
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

func AddWebHandlerLogs(c any, title string) func(time.Time, int) {
	w := InitContext(c)
	return AddWebLogs(w, title)
}

func AddWebLogs(w webFramework.WebFramework, title string) func(time.Time, int) {
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("title", title))
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("method", w.Parser.GetMethod()))
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("path", w.Parser.GetPath()))
	return func(start time.Time, status int) {
		elapsed := time.Since(start)
		webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("elapsed", elapsed.String()))
		webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.Int("status", status))
		webFramework.CollectLogTags(w, webFramework.HandlerLogTag)
		webFramework.CollectLogArrays(w, webFramework.HandlerLogTag)
	}
}
