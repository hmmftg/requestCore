package libTracing_test

import (
	"testing"
	"time"

	"github.com/hmmftg/requestCore/libTracing"
)

func TestRecordHTTPClientCallNoPanic(t *testing.T) {
	// Should not panic even if called with various inputs
	libTracing.RecordHTTPClientCall("test-api", "GET", 200, 100*time.Millisecond, nil)
	libTracing.RecordHTTPClientCall("test-api", "POST", 500, 50*time.Millisecond, nil)
	libTracing.RecordHTTPClientCall("test-api", "GET", 0, 10*time.Millisecond, nil)
}

func TestRecordHTTPClientCallWithError(t *testing.T) {
	err := &libTracingTestError{msg: "connection refused"}
	libTracing.RecordHTTPClientCall("test-api", "GET", 0, 5*time.Millisecond, err)
}

type libTracingTestError struct{ msg string }

func (e *libTracingTestError) Error() string { return e.msg }
