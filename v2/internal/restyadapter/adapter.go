// Package restyadapter implements the remotecall.Adapter interface using
// go-resty/resty/v3 as the underlying HTTP engine.
//
// The adapter has no knowledge of auth, telemetry, circuit breaking, error
// classification, or response building. It receives pre-marshaled bytes and
// returns raw bytes. The only Resty import in the entire codebase lives in
// this package.
//
// Replacing Resty with another engine (or pure stdlib) would only require
// rewriting this package — no public API or policy changes needed.
package restyadapter

import (
	"context"
	"net/http"
	"sync"

	"github.com/hmmftg/requestCore/v2/remotecall"
	"resty.dev/v3"
)

// Adapter implements remotecall.Adapter using Resty v3.
type Adapter struct {
	client *resty.Client
}

// New creates a new Adapter backed by the given *http.Client.
// The adapter never mutates the shared Resty client's retry config —
// per-call retry is applied to each resty.Request, not the client.
func New(httpClient *http.Client) *Adapter {
	client := resty.NewWithClient(httpClient)
	// Disable Resty's default retry conditions so they don't cause
	// unauthorized retries. v2 RetryPolicy is the sole authority.
	client.SetRetryDefaultConditions(false)
	return &Adapter{client: client}
}

func init() {
	remotecall.RegisterDefaultAdapterFactory(func(hc *http.Client) remotecall.Adapter {
		return New(hc)
	})
}

// Execute performs one logical HTTP exchange (which may include retries
// if req.Retry is non-nil). It returns (RawResponse, nil) for ALL HTTP
// responses including 4xx/5xx. It returns (RawResponse{}, err) only for
// transport-level failures.
//
// The adapter copies all response data before returning. The caller may
// safely retain RawResponse without worrying about buffer reuse.
func (a *Adapter) Execute(ctx context.Context, req remotecall.Request) (remotecall.RawResponse, error) {
	r := a.client.R().
		SetContext(ctx).
		SetMethod(req.Method).
		SetURL(req.URL)

	// Set headers
	for k, vs := range req.Headers {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}

	// Set body as raw bytes — immutable-byte replay on retry
	if len(req.Body) > 0 {
		r.SetBody(req.Body)
	}

	// Apply per-call retry policy
	if req.Retry != nil {
		applyRetryPolicy(r, req.Retry)
	}

	// Wire OnAttempt hook via retry hooks (fires after each failed attempt
	// before the next retry, with per-attempt timing).
	if req.OnAttempt != nil {
		tracker := &attemptTrackerType{
			onAttempt: req.OnAttempt,
		}
		r.AddRetryHooks(func(resp *resty.Response, err error) {
			tracker.recordAttempt(resp, err)
		})
	}

	// Execute
	resp, err := r.Send()

	// Fire OnAttempt for the final attempt. Resty retry hooks only fire
	// between attempts (not after the last one), so we emit the final
	// attempt here. Duration is approximate (total send time divided by
	// attempt count is not per-attempt, but Resty doesn't expose per-attempt
	// timing in the final response).
	if req.OnAttempt != nil {
		attempt := 1
		if resp != nil && resp.Request != nil {
			attempt = resp.Request.Attempt
		}
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode()
		}
		req.OnAttempt(remotecall.AttemptInfo{
			Attempt:    attempt,
			StatusCode: statusCode,
			Err:        err,
		})
	}

	if err != nil {
		return remotecall.RawResponse{}, err
	}

	// Copy response data — owned by the caller
	raw := remotecall.RawResponse{
		StatusCode: resp.StatusCode(),
		Headers:    copyHeaders(resp.Header()),
		Body:       copyBytes(resp.Bytes()),
	}

	return raw, nil
}

// attemptTrackerType tracks attempt info for the OnAttempt hook.
type attemptTrackerType struct {
	mu        sync.Mutex
	onAttempt func(remotecall.AttemptInfo)
}

func (t *attemptTrackerType) recordAttempt(resp *resty.Response, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.onAttempt == nil {
		return
	}
	attempt := 1
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode()
		if resp.Request != nil {
			attempt = resp.Request.Attempt
		}
	}
	t.onAttempt(remotecall.AttemptInfo{
		Attempt:    attempt,
		StatusCode: statusCode,
		Err:        err,
	})
}

// copyHeaders returns a deep copy of the http.Header.
func copyHeaders(h http.Header) http.Header {
	if len(h) == 0 {
		return http.Header{}
	}
	copied := make(http.Header, len(h))
	for k, vs := range h {
		copied[k] = append([]string(nil), vs...)
	}
	return copied
}

// copyBytes returns a copy of the byte slice.
func copyBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	copied := make([]byte, len(b))
	copy(copied, b)
	return copied
}
