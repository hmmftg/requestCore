package libError_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

func fakeErrorCaller(data string, depth int, err error, child libError.Error) libError.Error {
	if depth <= 0 {
		if child != nil {
			return libError.Add(child, http.StatusNotFound, data, err)
		} else {
			return libError.Convert(err, http.StatusNotFound, data, err)
		}
	}
	return fakeErrorCaller(data, depth-1, errors.Join(err, fmt.Errorf("sub %d", depth)), child)
}

type TestCase struct {
	Name          string
	Depth         int
	DesiredSrc    string
	NotDesiredSrc string
	Error         error
	Child         libError.Error
}

var testCases = []TestCase{
	{
		Name:          "LevelOne",
		Depth:         0,
		DesiredSrc:    "error_test.go",
		NotDesiredSrc: "error.go",
		Error:         fmt.Errorf("error: %s", "One"),
	},
	{
		Name:          "LevelTwo",
		Depth:         1,
		DesiredSrc:    "error_test.go",
		NotDesiredSrc: "error.go",
		Error:         fmt.Errorf("error: %s", "Two"),
	},
	{
		Name:          "LevelTen",
		Depth:         10,
		DesiredSrc:    "error_test.go",
		NotDesiredSrc: "error.go",
		Error:         fmt.Errorf("error: %s", "Ten"),
	},
	{
		Name:          "LevelTen",
		Depth:         10,
		DesiredSrc:    "error_test.go",
		NotDesiredSrc: "error.go",
		Error:         fmt.Errorf("error: %s", "Ten"),
		Child: libError.Add(
			libError.Add(
				libError.Add(
					libError.Add(
						libError.New(status.InternalServerError, "fifth", "grand child"),
						status.BadRequest, "forth", *libError.GetStack(),
					),
					status.NotFound, "third", nil,
				),
				status.BadRequest, "second", libError.Action{12, "ee", nil},
			),
			status.InternalServerError, "first", "dd",
		),
	},
}

func TestErrorLog(t *testing.T) {
	for _, tst := range testCases {
		err := fakeErrorCaller(tst.Name, tst.Depth, tst.Error, tst.Child)
		result := err.Error()
		t.Log(result)
		if !strings.Contains(result, tst.DesiredSrc) {
			t.Fatal(result, " does not contain ", tst.DesiredSrc)
		}
		if strings.Contains(result, tst.NotDesiredSrc) {
			t.Fatal(result, " contains ", tst.NotDesiredSrc)
		}
	}
}

func TestErrorSlogJson(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	for _, tst := range testCases {
		rLog := logger.With(slog.String("name", tst.Name), slog.Int("depth", tst.Depth))
		err := fakeErrorCaller(tst.Name, tst.Depth, tst.Error, tst.Child)
		result := err.Error()
		rLog.Log(context.Background(), slog.LevelError, "result", slog.Any("error", err))
		if !strings.Contains(result, tst.DesiredSrc) {
			t.Fatal(result, " does not contain ", tst.DesiredSrc)
		}
		if strings.Contains(result, tst.NotDesiredSrc) {
			t.Fatal(result, " contains ", tst.NotDesiredSrc)
		}
	}
}

func TestErrorSlogText(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	for _, tst := range testCases {
		rLog := logger.With(slog.String("name", tst.Name), slog.Int("depth", tst.Depth))
		err := fakeErrorCaller(tst.Name, tst.Depth, tst.Error, tst.Child)
		result := err.Error()
		rLog.Log(context.Background(), slog.LevelError, "result", slog.Any("error", err))
		if !strings.Contains(result, tst.DesiredSrc) {
			t.Fatal(result, " does not contain ", tst.DesiredSrc)
		}
		if strings.Contains(result, tst.NotDesiredSrc) {
			t.Fatal(result, " contains ", tst.NotDesiredSrc)
		}
	}
}
