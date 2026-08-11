package webFramework

import (
	"context"
	"fmt"
	"log"
	"log/slog"
)

const (
	// LogTagNameFormat is the format string for log tag local storage keys.
	LogTagNameFormat string = "LOG_TAG_%s"
	// LogArrayNameFormat is the format string for log array local storage keys.
	LogArrayNameFormat string = "LOG_ARRAY_%s"
	// HandlerLogTag is the log tag for handler lifecycle logs.
	HandlerLogTag string = "handler"
	// ErrorListLogTag is the log tag for error list logs.
	ErrorListLogTag string = "errors"
)

var startUpLogs []slog.Attr
var serviceRegistrationLogs []any

func addLog(w WebFramework, tag string, log slog.Attr) {
	v := w.Parser.GetLocal(tag)
	if v == nil {
		w.Parser.SetLocal(tag, []slog.Attr{log})
		return
	}
	if arr, ok := v.([]slog.Attr); ok {
		w.Parser.SetLocal(tag, append(arr, log))
	} else {
		slog.Error(fmt.Sprintf("log variable for %s is of wrong type %T", tag, arr), v)
	}
}

// AddLog appends a log attribute to the request's log array for the given title.
func AddLog(w WebFramework, title string, log slog.Attr) {
	name := fmt.Sprintf(LogArrayNameFormat, title)
	addLog(w, name, log)
}

// AddLogTag appends a log tag attribute to the request's log tags for the given title.
func AddLogTag(w WebFramework, title string, log slog.Attr) {
	name := fmt.Sprintf(LogTagNameFormat, title)
	addLog(w, name, log)
}

// AddStartUpLog appends a log attribute to the startup logs.
func AddStartUpLog(log slog.Attr) {
	if startUpLogs == nil {
		startUpLogs = []slog.Attr{}
	}
	startUpLogs = append(startUpLogs, log)
}

// AddStartUpLogTag appends a tagged log attribute to the startup logs.
func AddStartUpLogTag(title string, log slog.Attr) {
	if startUpLogs == nil {
		startUpLogs = []slog.Attr{}
	}
	startUpLogs = append(startUpLogs, slog.Any(title, log))
}

// CollectStartUpLogs emits all accumulated startup logs via slog and resets the buffer.
func CollectStartUpLogs() {
	slog.LogAttrs(context.Background(), slog.LevelInfo, "StartUp", startUpLogs...)
	startUpLogs = []slog.Attr{}
}

// AddServiceRegistrationLog records a service registration entry in the startup logs.
func AddServiceRegistrationLog(name string) {
	if serviceRegistrationLogs == nil {
		serviceRegistrationLogs = []any{}
	}
	serviceRegistrationLogs = append(serviceRegistrationLogs, slog.String(name, "registered"))
}

// CollectServiceRegistrationLogs emits accumulated service registration logs as a startup log group.
func CollectServiceRegistrationLogs() {
	if len(startUpLogs) == 0 {
		log.Fatal("unable to log service lregistration logs, start-up logs are empty")
	}
	logAttr := slog.Group("Service Registration Logs", serviceRegistrationLogs...)
	AddStartUpLog(logAttr)
	serviceRegistrationLogs = []any{}
}

func collectLogs(w WebFramework, tag, title string, isObject bool) {
	v := w.Parser.GetLocal(tag)
	if v == nil {
		return
	}
	if arr, ok := v.([]slog.Attr); ok {
		if isObject {
			w.Parser.AddCustomAttributes(slog.Any(title, arr))
		} else {
			for id := range arr {
				w.Parser.AddCustomAttributes(arr[id])
			}
		}
	} else {
		slog.Error(fmt.Sprintf("log variable for %s is of wrong type %T", title, arr), v)
	}
}

// CollectLogArrays collects log array attributes for the given title and adds them as custom attributes.
func CollectLogArrays(w WebFramework, title string) {
	name := fmt.Sprintf(LogArrayNameFormat, title)
	collectLogs(w, name, title, true)
}

// CollectLogTags collects log tag attributes for the given title and adds them as custom attributes.
func CollectLogTags(w WebFramework, title string) {
	name := fmt.Sprintf(LogTagNameFormat, title)
	collectLogs(w, name, title, false)
}
