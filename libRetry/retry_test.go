package libRetry_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRetry"
	"github.com/hmmftg/requestCore/status"
	"gotest.tools/v3/assert"
)

type testResp struct {
	Data       string `json:"data"`
	statusCode int
	errCode    int
}

func (r *testResp) GetStatus() int   { return r.statusCode }
func (r *testResp) GetErrorCode() int { return r.errCode }

func TestWithRetry_ImmediateSuccess(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{MaxRetries: 3}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		calls.Add(1)
		return &testResp{Data: "ok"}, nil, 200
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
	assert.Equal(t, result.Response.Data, "ok")
}

func TestWithRetry_TimeoutThenSuccess(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:      2,
		RetryOnTimeout:  true,
		Backoff:         0,
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		n := calls.Add(1)
		if n == 1 {
			return nil, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
		}
		return &testResp{Data: "ok"}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		calls.Add(1)
		return nil, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
	})

	assert.Equal(t, calls.Load(), int32(3))
	assert.Assert(t, result.Error != nil)
	assert.Equal(t, result.Attempts, 3)
}

func TestWithRetry_RetryableStatus(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:     2,
		RetryOnStatus:  map[int]bool{503: true},
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		n := calls.Add(1)
		if n < 3 {
			return nil, &libCallApi.RemoteCallError{Status: 503, Body: nil, Err: errors.New("503")}, 503
		}
		return &testResp{Data: "ok"}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		calls.Add(1)
		return nil, &libCallApi.RemoteCallError{Status: 404, Body: nil, Err: errors.New("404")}, 404
	})

	assert.Equal(t, calls.Load(), int32(1))
	assert.Assert(t, result.Error != nil)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_RetryableErrorCode(t *testing.T) {
	var calls atomic.Int32
	policy := &libRetry.RetryPolicy{
		MaxRetries:       2,
		RetryOnErrorCodes: map[int]bool{5001: true},
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		n := calls.Add(1)
		if n < 3 {
			return &testResp{Data: "", statusCode: 200, errCode: 5001}, nil, 200
		}
		return &testResp{Data: "ok", statusCode: 200, errCode: 0}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		calls.Add(1)
		return nil, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		return nil, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
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

	libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		titles = append(titles, libRetry.FormatAttemptTitle("test-call", attempt))
		return nil, libError.NewWithDescription(
			status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
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
		Sleep: func(ctx context.Context, d time.Duration) bool {
			sleepCalls.Add(1)
			return true
		},
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		if attempt < 3 {
			return nil, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
		}
		return &testResp{Data: "ok"}, nil, 200
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, sleepCalls.Load(), int32(2), "should sleep between attempts only")
}

func TestWithRetry_NilPolicy(t *testing.T) {
	result := libRetry.WithRetry(nil, func(attempt int) (*testResp, error, int) {
		return &testResp{Data: "ok"}, nil, 200
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 1)
}

func TestWithRetry_ElapsedDuration(t *testing.T) {
	policy := &libRetry.RetryPolicy{
		MaxRetries:     1,
		RetryOnTimeout: true,
		Sleep: func(ctx context.Context, d time.Duration) bool {
			time.Sleep(5 * time.Millisecond)
			return true
		},
		Backoff: 5 * time.Millisecond,
	}

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		if attempt == 1 {
			return nil, libError.NewWithDescription(
				status.StatusCode(408), "API_CALL_TIME_OUT", "timeout"), 408
		}
		return &testResp{Data: "ok"}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		n := calls.Add(1)
		if n == 1 {
			return nil, libError.NewWithDescription(
				status.StatusCode(408), "API_CONNECT_TIMED_OUT", "connect timeout"), 408
		}
		return &testResp{Data: "ok"}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		if attempt == 1 {
			return nil, customErr, 0
		}
		return &testResp{Data: "ok"}, nil, 200
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

	result := libRetry.WithRetry(policy, func(attempt int) (*testResp, error, int) {
		if attempt == 1 {
			return nil, fmt.Errorf("api.example.com: elapsed 5s exceeds timeout 1s: %w", sentinelErr), 200
		}
		return &testResp{Data: "ok"}, nil, 200
	})

	assert.NilError(t, result.Error)
	assert.Equal(t, result.Attempts, 2)
}
