package webFramework

import (
	"fmt"
	"log/slog"
)

const (
	LogTagNameFormat   string = "LOG_TAG_%s"
	LogArrayNameFormat string = "LOG_ARRAY_%s"
	HandlerLogTag      string = "handler"
)

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

func collectLogs(w WebFramework, tag string, isObject bool) {
	v := w.Parser.GetLocal(tag)
	if v == nil {
		return
	}
	if arr, ok := v.([]slog.Attr); ok {
		if isObject {
			w.Parser.AddCustomAttributes(slog.Any(tag, arr))
		} else {
			for id := range arr {
				w.Parser.AddCustomAttributes(arr[id])
			}
		}
	} else {
		slog.Error(fmt.Sprintf("log variable for %s is of wrong type %T", tag, arr), v)
	}
}

func CollectLogArrays(w WebFramework, title string) {
	name := fmt.Sprintf(LogArrayNameFormat, title)
	collectLogs(w, name, true)
}

func CollectLogTags(w WebFramework, title string) {
	name := fmt.Sprintf(LogTagNameFormat, title)
	collectLogs(w, name, false)
}
