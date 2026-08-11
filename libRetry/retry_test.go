package libRetry_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRetry"
	"github.com/hmmftg/requestCore/status"
)

type testResp struct {
	Data       string `json:"data"`
	statusCode int
	errCode    int
	errKey     string
}

func (r *testResp) GetStatus() int      { return r.statusCode }
func (r *testResp) GetErrorCode() int   { return r.errCode }
func (r *testResp) GetErrorKey() string { return r.errKey }

func TestWithRetry_ImmediateSuccess(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{MaxRetries: 3}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
	assert.Equal(t, result.Response.Data, "ok")
}

func TestWithRetry_TimeoutThenSuccess(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     2,
		RetryOnTimeout: true,
		Backoff:        0,
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n == 1 {
			return nil, 408, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(2))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
	assert.Equal(t, result.Response.Data, "ok")
}

func TestWithRetry_ExhaustedTimeout(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     2,
		RetryOnTimeout: true,
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return nil, 408, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
	})

	assert.Equal(t, calls.Load(), int32(3))
	assert.Assert(t, result.Error != nil)
	assert.Equal(t, result.Attempts, 3)
}

func TestWithRetry_RetryableStatus(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:    2,
		RetryOnStatus: map[int]bool{503: true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n < 3 {
			return nil, 503, &libCallApi.RemoteCallError{Status: 503, Body: nil, Err: errors.New("503")}
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(3))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 3)
}

func TestWithRetry_NonRetryableStatus(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:    3,
		RetryOnStatus: map[int]bool{503: true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return nil, 404, &libCallApi.RemoteCallError{Status: 404, Body: nil, Err: errors.New("404")}
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.Assert(t, result.Error != nil)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_RetryableErrorCode(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:        2,
		RetryOnErrorCodes: map[int]bool{5001: true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n < 3 {
			return &testResp{Data: "", statusCode: 200, errCode: 5001}, 200, nil
		}
		return &testResp{Data: "ok", statusCode: 200, errCode: 0}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(3))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 3)
}

func TestWithRetry_NoRetries(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     0,
		RetryOnTimeout: true,
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return nil, 408, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.Assert(t, result.Error != nil)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_CancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	policy := &libRetry.RetryPolicy{
		MaxRetries:     3,
		RetryOnTimeout: true,
		Backoff:        100 * time.Millisecond,
		Context:        ctx,
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		return nil, 408, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
	})

	assert.Assert(t, result.Error != nil)
	assert.Assert(t, errors.Is(result.Error, context.Canceled))
}

func TestWithRetry_TitlesPerAttempt(t *testing.T) {
	var titles []string
	policy := &libRetry.RetryPolicy{
		MaxRetries:     2,
		RetryOnTimeout: true,
	}

	libRetry.WithRetry(policy, func(attempt int) (*testResp, int, error) {
		titles = append(titles, libRetry.FormatAttemptTitle("test-call", attempt))
		return nil, 408, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
	})

	assert.Equal(t, len(titles), 3)
	assert.Equal(t, titles[0], "test-call")
	assert.Equal(t, titles[1], "test-call-retry-1")
	assert.Equal(t, titles[2], "test-call-retry-2")
}

func TestWithRetry_CustomSleep(t *testing.T) {
	var sleepCalls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     2,
		RetryOnTimeout: true,
		Backoff:        1 * time.Second,
		Sleep: func(_ context.Context, _ time.Duration) bool {
			sleepCalls.Add(1)
			return true
		},
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, int, error) {
		if attempt < 3 {
			return nil, 408, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, sleepCalls.Load(), int32(2), "should sleep between attempts only")
}

func TestWithRetry_NilPolicy(t *testing.T) {
	result := libRetry.WithRetry(nil, func(_ int) (*testResp, int, error) {
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_ElapsedDuration(t *testing.T) {
	policy := &libRetry.RetryPolicy{
		MaxRetries:     1,
		RetryOnTimeout: true,
		Sleep: func(_ context.Context, _ time.Duration) bool {
			time.Sleep(5 * time.Millisecond)
			return true
		},
		Backoff: 5 * time.Millisecond,
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, int, error) {
		if attempt == 1 {
			return nil, 408, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout")
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.NilError(t, result.Error)
	assert.Assert(t, result.Elapsed > 0, "elapsed should be positive")
}

func TestFormatAttemptTitle(t *testing.T) {
	assert.Equal(t, libRetry.FormatAttemptTitle("call", 1), "call")
	assert.Equal(t, libRetry.FormatAttemptTitle("call", 2), "call-retry-1")
	assert.Equal(t, libRetry.FormatAttemptTitle("call", 3), "call-retry-2")
}

func TestWithRetry_ConnectTimedOut(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     1,
		RetryOnTimeout: true,
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n == 1 {
			return nil, 408, libError.NewWithDescription(
				status.StatusCode(408), "API_CONNECT_TIMED_OUT", "connect timeout")
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(2))
	assert.NilError(t, result.Error)
}

func TestWithRetry_CustomTimeoutPredicate(t *testing.T) {
	customErr := errors.New("custom transport timeout")
	policy := &libRetry.RetryPolicy{
		MaxRetries:     1,
		RetryOnTimeout: true,
		IsTimeoutError: func(err error) bool {
			return err == customErr
		},
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, int, error) {
		if attempt == 1 {
			return nil, 0, customErr
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
}

func TestWithRetry_DefaultTimeoutPredicateSentinel(t *testing.T) {
	sentinelErr := errors.New("server-side timeout exceeded")
	policy := &libRetry.RetryPolicy{
		MaxRetries:     1,
		RetryOnTimeout: true,
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, int, error) {
		if attempt == 1 {
			return nil, 200, fmt.Errorf("api.example.com: elapsed 5s exceeds timeout 1s: %w", sentinelErr)
		}
		return &testResp{Data: "ok"}, 200, nil
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
}

func TestWithRetry_RetryableErrorKey(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:       2,
		RetryOnErrorKeys: map[string]bool{"SERVICE_UNAVAILABLE": true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n == 1 {
			return &testResp{Data: "", statusCode: 200, errKey: "SERVICE_UNAVAILABLE"}, 200, nil
		}
		return &testResp{Data: "ok", statusCode: 200, errKey: ""}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(2))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
	assert.Equal(t, result.Response.Data, "ok")
}

func TestWithRetry_NonRetryableErrorKey(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:       2,
		RetryOnErrorKeys: map[string]bool{"SERVICE_UNAVAILABLE": true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return &testResp{Data: "bad", statusCode: 200, errKey: "BAD_REQUEST"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_ErrorKeyAndCodeBothChecked(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:        2,
		RetryOnErrorCodes: map[int]bool{5001: true},
		RetryOnErrorKeys:  map[string]bool{"SERVICE_UNAVAILABLE": true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		n := calls.Add(1)
		if n == 1 {
			// errCode matches but errKey does not — retry should trigger by code.
			return &testResp{Data: "", statusCode: 200, errCode: 5001, errKey: "OTHER"}, 200, nil
		}
		return &testResp{Data: "ok", statusCode: 200, errCode: 0, errKey: ""}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(2))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
}

func TestWithRetry_ErrorKeyEmptyString(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:       2,
		RetryOnErrorKeys: map[string]bool{"SERVICE_UNAVAILABLE": true},
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return &testResp{Data: "ok", statusCode: 200, errKey: ""}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_NilRetryOnErrorKeys(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries: 2,
		// RetryOnErrorKeys is nil.
	}

	result := libRetry.WithRetry(policy, func(_ int) (*testResp, int, error) {
		calls.Add(1)
		return &testResp{Data: "ok", statusCode: 200, errKey: "SERVICE_UNAVAILABLE"}, 200, nil
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
}

// nonStatusType is a test type that does NOT implement StatusProvider.
type nonStatusType struct {
	val int
}

func TestExtractStatusCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		resp *testResp
		want int
	}{
		{name: "status_200", resp: &testResp{statusCode: 200}, want: 200},
		{name: "status_zero", resp: &testResp{statusCode: 0}, want: 0},
		{name: "nil_pointer", resp: (*testResp)(nil), want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := libRetry.ExtractStatusCode(tc.resp)
			assert.Equal(t, got, tc.want)
		})
	}

	t.Run("non_status_provider", func(t *testing.T) {
		t.Parallel()
		got := libRetry.ExtractStatusCode(&nonStatusType{val: 42})
		assert.Equal(t, got, 0)
	})
}

func TestDeriveStatusCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		resp *testResp
		err  error
		want int
	}{
		{name: "with_error", resp: &testResp{statusCode: 200}, err: errors.New("fail"), want: http.StatusInternalServerError},
		{name: "nil_resp_nil_err", resp: nil, err: nil, want: http.StatusInternalServerError},
		{name: "status_406", resp: &testResp{statusCode: 406}, err: nil, want: 406},
		{name: "status_zero", resp: &testResp{statusCode: 0}, err: nil, want: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := libRetry.DeriveStatusCode(tc.resp, tc.err)
			assert.Equal(t, got, tc.want)
		})
	}
}
