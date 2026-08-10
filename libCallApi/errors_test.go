package libCallApi_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/hmmftg/requestCore/libCallApi"
)

func TestRemoteCallErrorError(t *testing.T) {
	inner := errors.New("inner error")
	rce := &libCallApi.RemoteCallError{Status: 403, Body: []byte("forbidden body"), Err: inner}
	assert.Equal(t, rce.Error(), "remote call failed (status 403): inner error")
}

func TestRemoteCallErrorErrorNoInner(t *testing.T) {
	rce := &libCallApi.RemoteCallError{Status: 500, Body: []byte("server error")}
	assert.Equal(t, rce.Error(), "remote call failed (status 500)")
}

func TestRemoteCallErrorUnwrap(t *testing.T) {
	inner := errors.New("inner error")
	rce := &libCallApi.RemoteCallError{Status: 403, Body: nil, Err: inner}
	assert.Equal(t, rce.Unwrap(), inner)
}

func TestRemoteCallErrorUnwrapNil(t *testing.T) {
	rce := &libCallApi.RemoteCallError{Status: 403, Body: nil}
	assert.Assert(t, rce.Unwrap() == nil)
}

func TestIsForbidden403(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusForbidden, Body: []byte("forbidden")}
	assert.Equal(t, libCallApi.IsForbidden(err), true)
}

func TestIsForbidden401(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusUnauthorized, Body: []byte("unauthorized")}
	assert.Equal(t, libCallApi.IsForbidden(err), true)
}

func TestIsForbidden500(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusInternalServerError, Body: []byte("server error")}
	assert.Equal(t, libCallApi.IsForbidden(err), false)
}

func TestIsForbiddenNil(t *testing.T) {
	assert.Equal(t, libCallApi.IsForbidden(nil), false)
}

func TestIsForbiddenNonRemoteCallError(t *testing.T) {
	assert.Equal(t, libCallApi.IsForbidden(errors.New("some other error")), false)
}

func TestIsClientError4xx(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusNotFound, Body: []byte("not found")}
	assert.Equal(t, libCallApi.IsClientError(err), true)
}

func TestIsClientError5xx(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusInternalServerError, Body: []byte("server error")}
	assert.Equal(t, libCallApi.IsClientError(err), false)
}

func TestIsServerError5xx(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusBadGateway, Body: []byte("bad gateway")}
	assert.Equal(t, libCallApi.IsServerError(err), true)
}

func TestIsServerError4xx(t *testing.T) {
	err := &libCallApi.RemoteCallError{Status: http.StatusBadRequest, Body: []byte("bad request")}
	assert.Equal(t, libCallApi.IsServerError(err), false)
}

func TestRemoteCallErrorErrorsAs(t *testing.T) {
	inner := errors.New("inner error")
	rce := &libCallApi.RemoteCallError{Status: 403, Body: []byte("forbidden"), Err: inner}
	wrapped := fmt.Errorf("wrapped: %w", rce)

	var target *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(wrapped, &target), "errors.As should find RemoteCallError")
	assert.Equal(t, target.Status, 403)
	assert.DeepEqual(t, target.Body, []byte("forbidden"))
}

func TestStatusPreservingBuilder2xx(t *testing.T) {
	resp, err := libCallApi.StatusPreservingBuilder[map[string]any](
		200,
		[]byte(`{"key":"value"}`),
		nil,
	)
	assert.NilError(t, err)
	assert.Equal(t, (*resp)["key"], "value")
}

func TestStatusPreservingBuilder201(t *testing.T) {
	resp, err := libCallApi.StatusPreservingBuilder[map[string]any](
		201,
		[]byte(`{"status":"created"}`),
		nil,
	)
	assert.NilError(t, err, "201 should not be an error")
	assert.Equal(t, (*resp)["status"], "created")
}

func TestStatusPreservingBuilder204(t *testing.T) {
	resp, err := libCallApi.StatusPreservingBuilder[map[string]any](
		204,
		nil,
		nil,
	)
	assert.NilError(t, err, "204 should not be an error")
	assert.Assert(t, resp != nil, "resp should not be nil for 204")
}

func TestStatusPreservingBuilder403(t *testing.T) {
	resp, err := libCallApi.StatusPreservingBuilder[map[string]any](
		403,
		[]byte(`{"error":"forbidden"}`),
		nil,
	)
	assert.Assert(t, resp == nil, "resp should be nil on non-2xx")
	assert.Assert(t, err != nil, "err should be non-nil on non-2xx")

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "err should be a RemoteCallError")
	assert.Equal(t, rce.Status, 403)
	assert.DeepEqual(t, rce.Body, []byte(`{"error":"forbidden"}`))
}

func TestStatusPreservingBuilderMalformed2xx(t *testing.T) {
	resp, err := libCallApi.StatusPreservingBuilder[map[string]any](
		200,
		[]byte(`{invalid json`),
		nil,
	)
	assert.Assert(t, resp == nil, "resp should be nil on parse error")
	assert.Assert(t, err != nil, "err should be non-nil on parse error")

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, !errors.As(err, &rce), "err should NOT be a RemoteCallError for malformed 2xx JSON")

	assert.Assert(t, strings.Contains(err.Error(), "parse response"), "error should contain 'parse response', got: %s", err.Error())
}
