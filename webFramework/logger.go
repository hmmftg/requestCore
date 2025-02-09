package webFramework

import (
	"fmt"
	"log/slog"
)

func AddLog(w WebFramework, title string, log slog.Attr) {
	name := fmt.Sprintf("LOG_%s", title)
	v := w.Parser.GetLocal(name)
	if v == nil {
		w.Parser.SetLocal(name, []slog.Attr{log})
		return
	}
	if arr, ok := v.([]slog.Attr); ok {
		w.Parser.SetLocal(name, append(arr, log))
	} else {
		slog.Error(fmt.Sprintf("log variable for %s is of wrong type %T", title, arr), v)
	}
}

func CollectLogs(w WebFramework, title string) {
	name := fmt.Sprintf("LOG_%s", title)
	v := w.Parser.GetLocal(name)
	if v == nil {
		return
	}
	if arr, ok := v.([]slog.Attr); ok {
		w.Parser.AddCustomAttributes(slog.Any(title, arr))
	} else {
		slog.Error(fmt.Sprintf("log variable for %s is of wrong type %T", title, arr), v)
	}
}
