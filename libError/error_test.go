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
				status.BadRequest, "second", libError.Action{Status: 12, Description: "ee", Message: nil},
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

func TestNewWithDescription(t *testing.T) {
	tests := []struct {
		name        string
		status      status.StatusCode
		desc        string
		format      string
		args        []interface{}
		expectedMsg string
	}{
		{
			name:        "Simple string without format",
			status:      status.BadRequest,
			desc:        "VALIDATION_ERROR",
			format:      "User not found",
			args:        nil,
			expectedMsg: "User not found",
		},
		{
			name:        "String with single argument",
			status:      status.NotFound,
			desc:        "USER_NOT_FOUND",
			format:      "User with ID %d not found",
			args:        []interface{}{123},
			expectedMsg: "User with ID 123 not found",
		},
		{
			name:        "String with multiple arguments",
			status:      status.BadRequest,
			desc:        "INVALID_INPUT",
			format:      "Invalid input: %s, value: %d",
			args:        []interface{}{"age", 150},
			expectedMsg: "Invalid input: age, value: 150",
		},
		{
			name:        "String with string argument",
			status:      status.InternalServerError,
			desc:        "DATABASE_ERROR",
			format:      "Database error: %s",
			args:        []interface{}{"connection timeout"},
			expectedMsg: "Database error: connection timeout",
		},
		{
			name:        "String with float argument",
			status:      status.BadRequest,
			desc:        "INVALID_VALUE",
			format:      "Value %.2f is out of range",
			args:        []interface{}{3.14159},
			expectedMsg: "Value 3.14 is out of range",
		},
		{
			name:        "String with bool argument",
			status:      status.BadRequest,
			desc:        "INVALID_FLAG",
			format:      "Feature enabled: %t",
			args:        []interface{}{true},
			expectedMsg: "Feature enabled: true",
		},
		{
			name:        "String with multiple types",
			status:      status.BadRequest,
			desc:        "VALIDATION_FAILED",
			format:      "User %s (ID: %d, Age: %d) - Status: %t",
			args:        []interface{}{"John Doe", 1001, 25, true},
			expectedMsg: "User John Doe (ID: 1001, Age: 25) - Status: true",
		},
		{
			name:        "String with percent sign",
			status:      status.BadRequest,
			desc:        "PERCENTAGE_ERROR",
			format:      "Progress: %d%% completed",
			args:        []interface{}{75},
			expectedMsg: "Progress: 75% completed",
		},
		{
			name:        "String with no placeholders but args provided",
			status:      status.BadRequest,
			desc:        "SIMPLE_ERROR",
			format:      "Simple error message",
			args:        []interface{}{"extra", 123},
			expectedMsg: "Simple error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := libError.NewWithDescription(tt.status, tt.desc, tt.format, tt.args...)

			// Check that error is not nil
			if err == nil {
				t.Fatal("Expected error to be not nil")
			}

			// Get error string
			errStr := err.Error()

			// Check if error string contains the expected message
			if !strings.Contains(errStr, tt.expectedMsg) {
				t.Errorf("Expected error message to contain '%s', but got '%s'", tt.expectedMsg, errStr)
			}

			// Check that description is in the error
			if !strings.Contains(errStr, tt.desc) {
				t.Errorf("Expected error to contain description '%s', but got '%s'", tt.desc, errStr)
			}

			// Check action data
			action := err.Action()
			if action.Status != tt.status {
				t.Errorf("Expected status %d, but got %d", tt.status, action.Status)
			}

			if action.Description != tt.desc {
				t.Errorf("Expected description '%s', but got '%s'", tt.desc, action.Description)
			}

			// Check the message in action data
			msgStr := fmt.Sprintf("%v", action.Message)
			if !strings.Contains(msgStr, tt.expectedMsg) {
				t.Errorf("Expected action message to contain '%s', but got '%s'", tt.expectedMsg, msgStr)
			}

			// Check that source is set
			if err.Src() == nil {
				t.Error("Expected source to be set, but got nil")
			}
		})
	}
}

func TestNewWithDescription_EmptyFormat(t *testing.T) {
	err := libError.NewWithDescription(status.BadRequest, "TEST", "")

	if err == nil {
		t.Fatal("Expected error to be not nil")
	}

	errStr := err.Error()

	// Should still be a valid error
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}

	// Check description is present
	if !strings.Contains(errStr, "TEST") {
		t.Errorf("Expected error to contain 'TEST', but got '%s'", errStr)
	}
}
