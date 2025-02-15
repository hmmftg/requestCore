package webFramework

import (
	"context"
	"fmt"
	"log"
	"log/slog"
)

const (
	LogTagNameFormat   string = "LOG_TAG_%s"
	LogArrayNameFormat string = "LOG_ARRAY_%s"
	HandlerLogTag      string = "handler"
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

func AddLog(w WebFramework, title string, log slog.Attr) {
	name := fmt.Sprintf(LogArrayNameFormat, title)
	addLog(w, name, log)
}

func AddLogTag(w WebFramework, title string, log slog.Attr) {
	name := fmt.Sprintf(LogTagNameFormat, title)
	addLog(w, name, log)
}

func AddStartUpLog(log slog.Attr) {
	if startUpLogs == nil {
		startUpLogs = []slog.Attr{}
	}
	startUpLogs = append(startUpLogs, log)
}

func AddStartUpLogTag(title string, log slog.Attr) {
	if startUpLogs == nil {
		startUpLogs = []slog.Attr{}
	}
	startUpLogs = append(startUpLogs, slog.Any(title, log))
}

func CollectStartUpLogs() {
	slog.LogAttrs(context.Background(), slog.LevelInfo, "StartUp", startUpLogs...)
	startUpLogs = []slog.Attr{}
}

func AddServiceRegistrationLog(name string) {
	if serviceRegistrationLogs == nil {
		serviceRegistrationLogs = []any{}
	}
	serviceRegistrationLogs = append(serviceRegistrationLogs, name)
}

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

func CollectLogArrays(w WebFramework, title string) {
	name := fmt.Sprintf(LogArrayNameFormat, title)
	collectLogs(w, name, title, true)
}

func CollectLogTags(w WebFramework, title string) {
	name := fmt.Sprintf(LogTagNameFormat, title)
	collectLogs(w, name, title, false)
}
