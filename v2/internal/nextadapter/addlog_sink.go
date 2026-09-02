package nextadapter

import (
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
	legacy "github.com/hmmftg/requestCore/webFramework"
)

// addLogSink is a request-scoped telemetry.Sink that forwards lifecycle
// events to the legacy AddLog pipeline. It never includes raw request
// or response bodies. Each event uses one slog.Attr per AddLog call.
//
// The mandatory <operation>-req success entry (containing the typed
// response) is emitted by Wrap after Execute returns, not by this sink,
// because telemetry.Event does not carry the typed response.
type addLogSink struct {
	w         legacy.WebFramework
	operation string
}

// Record forwards a telemetry event to the AddLog pipeline under the
// HandlerLogTag for start/success and under <operation>-req-failed for
// failure.
func (s *addLogSink) Record(event telemetry.Event) {
	if s.w.Parser == nil {
		return
	}
	switch event.Type {
	case telemetry.EventStart:
		s.recordStart(event)
	case telemetry.EventSuccess:
		s.recordSuccess(event)
	case telemetry.EventFailure:
		s.recordFailure(event)
	case telemetry.EventBusiness:
		// Business events are forwarded under the operation tag.
		legacy.AddLog(s.w, s.operation, slog.String("event", "business"))
	}
}

func (s *addLogSink) recordStart(event telemetry.Event) {
	legacy.AddLogTag(s.w, legacy.HandlerLogTag, slog.String("operation", event.Operation))
	legacy.AddLogTag(s.w, legacy.HandlerLogTag, slog.String("method", event.Method))
	legacy.AddLogTag(s.w, legacy.HandlerLogTag, slog.String("route", event.RoutePattern))
}

func (s *addLogSink) recordSuccess(event telemetry.Event) {
	legacy.AddLogTag(s.w, legacy.HandlerLogTag, slog.Int("status", event.Status))
	legacy.AddLogTag(s.w, legacy.HandlerLogTag, slog.String("elapsed", event.Duration.String()))
}

func (s *addLogSink) recordFailure(event telemetry.Event) {
	errAttr := slog.Any("error", event.Err)
	legacy.AddLog(s.w, s.operation+"-req-failed", errAttr)
	if event.Status != 0 {
		legacy.AddLog(s.w, s.operation+"-req-failed", slog.Int("status", event.Status))
	}
}

// addWebLogs is a local equivalent of libContext.AddWebLogs that avoids
// importing root libContext (whose dependency surface includes every
// legacy framework). It records title/method/path as log tags and
// returns a completion closure that logs elapsed and status and
// collects the HandlerLogTag tags/arrays.
func addWebLogs(w legacy.WebFramework, title string) func(time.Time, int) {
	legacy.AddLogTag(w, legacy.HandlerLogTag, slog.String("title", title))
	legacy.AddLogTag(w, legacy.HandlerLogTag, slog.String("method", w.Parser.GetMethod()))
	legacy.AddLogTag(w, legacy.HandlerLogTag, slog.String("path", w.Parser.GetPath()))
	return func(start time.Time, status int) {
		elapsed := time.Since(start)
		legacy.AddLogTag(w, legacy.HandlerLogTag, slog.String("elapsed", elapsed.String()))
		legacy.AddLogTag(w, legacy.HandlerLogTag, slog.Int("status", status))
		legacy.CollectLogTags(w, legacy.HandlerLogTag)
		legacy.CollectLogArrays(w, legacy.HandlerLogTag)
	}
}
