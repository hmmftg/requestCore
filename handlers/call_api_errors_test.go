package handlers_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/handlers"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
	"gotest.tools/v3/assert"
)

func TestNormalizeCallError_Nil(t *testing.T) {
	assert.NilError(t, handlers.NormalizeCallError(nil))
}

func TestNormalizeCallError_NonLibError(t *testing.T) {
	original := errors.New("plain network error")
	result := handlers.NormalizeCallError(original)
	assert.Equal(t, result, original, "non-libError errors should be returned unchanged")
}

func TestNormalizeCallError_ConnectTimedOut(t *testing.T) {
	original := libError.NewWithDescription(
		status.StatusCode(http.StatusRequestTimeout),
		"API_CONNECT_TIMED_OUT",
		"connection timed out",
	)
	result := handlers.NormalizeCallError(original)
	assert.Equal(t, result, original, "API_CONNECT_TIMED_OUT should be preserved unchanged")
}

func TestNormalizeCallError_UnableParseResp(t *testing.T) {
	child := libError.NewWithDescription(
		status.BadRequest,
		"API_UNABLE_PARSE_RESP",
		"error in GetResp.json.Unmarshal: invalid char",
	)
	wrapped := libError.Add(child, status.InternalServerError, "parent", "context")

	result := handlers.NormalizeCallError(wrapped)
	assert.Assert(t, result != nil)

	ok, libErr := libError.Unwrap(result)
	assert.Assert(t, ok, "result should be a libError")
	assert.Equal(t, libErr.Action().Description, "API_CALL_ERROR")
}

func TestNormalizeCallError_OtherLibError(t *testing.T) {
	original := libError.NewWithDescription(
		status.InternalServerError,
		"API_UNABLE_TO_CALL",
		"connection refused",
	)
	result := handlers.NormalizeCallError(original)
	assert.Assert(t, result != nil)

	ok, libErr := libError.Unwrap(result)
	assert.Assert(t, ok, "result should be a libError")
	assert.Equal(t, libErr.Action().Description, "API_CALL_ERROR")
}

func TestBuildTimeoutError(t *testing.T) {
	err := handlers.BuildTimeoutError("https://api.example.com")
	assert.Assert(t, err != nil)

	ok, libErr := libError.Unwrap(err)
	assert.Assert(t, ok, "should be a libError")
	assert.Equal(t, libErr.Action().Description, "API_CALL_TIME_OUT")

	errStr := err.Error()
	assert.Assert(t, strings.Contains(errStr, "https://api.example.com"),
		"timeout error should contain domain, got: %s", errStr)
}

func TestBuildTimeoutError_ErrorsIs(t *testing.T) {
	err := handlers.BuildTimeoutError("https://api.example.com")
	assert.Assert(t, errors.Is(err, handlers.ErrServerTimeout),
		"errors.Is should detect ErrServerTimeout sentinel")
}

func TestBuildTimeoutError_ErrorsIsNotRegularError(t *testing.T) {
	regularErr := errors.New("some other error")
	assert.Assert(t, !errors.Is(regularErr, handlers.ErrServerTimeout),
		"regular errors should not match ErrServerTimeout")
}
