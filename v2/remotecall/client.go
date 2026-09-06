package remotecall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
)

// RequestBodyType selects how the request body is marshaled.
type RequestBodyType int

const (
	// BodyTypeJSON marshals Body via json.Marshal.
	BodyTypeJSON RequestBodyType = iota

	// BodyTypeForm requires Body to have dynamic type url.Values.
	// The client calls Encode() and sends the resulting bytes with
	// Content-Type: application/x-www-form-urlencoded.
	// If Body is not url.Values, Call returns a preflight error (no HTTP request).
	BodyTypeForm

	// BodyTypeEmpty ignores Body — no request body is sent.
	BodyTypeEmpty
)

// RemoteAPI configures a target remote API.
type RemoteAPI struct {
	// Name is the application-controlled API name used in OperationKey.
	Name string

	// BaseURL is the base URL for the API (e.g., "https://api.example.com").
	BaseURL string

	// SkipPatterns are per-API skip patterns for ShouldSkipCall.
	SkipPatterns []string
}

// CallConfig is the per-call configuration. Only what changes between calls
// belongs here; client-wide concerns are on RemoteClient via functional options.
type CallConfig[Req, Resp any] struct {
	API      RemoteAPI             // target API config
	Method   string                // HTTP method
	Path     string                // URL path (appended to BaseURL)
	Query    url.Values            // query parameters
	Headers  http.Header           // extra headers (merged over defaults)
	Body     Req                   // request body (marshaled based on BodyType)
	BodyType RequestBodyType       // JSON, Form, Empty
	Builder  ResponseBuilder[Resp] // response parser
	Timeout  time.Duration         // per-call timeout (0 = client default)
	Retry    *RetryPolicy          // overrides client default if non-nil; nil = client default or no retry
}

// Request is the internal adapter contract type. Callers never construct
// this directly — RemoteClient.Call builds it from CallConfig.
type Request struct {
	Method    string
	URL       string
	Headers   http.Header
	Body      []byte            // pre-marshaled by RemoteClient; immutable bytes replayed by Resty on retry
	Retry     *RetryPolicy      // immutable per-call retry policy; nil = no retry
	OnAttempt func(AttemptInfo) // optional hook, called after each HTTP attempt
}

// AttemptInfo is passed to the OnAttempt hook after each HTTP attempt.
type AttemptInfo struct {
	Attempt    int           // 1-based attempt number
	StatusCode int           // HTTP status code (0 if no response received)
	Duration   time.Duration // time for this attempt
	Err        error         // nil if attempt succeeded
}

// RawResponse is the raw HTTP response returned by the adapter.
// Body and Headers are owned copies — the adapter copies from Resty's
// internal buffers before returning. The caller may safely retain
// RawResponse without worrying about buffer reuse.
type RawResponse struct {
	StatusCode int
	Headers    http.Header // owned copy, not a reference to internal buffers
	Body       []byte      // owned copy, not a reference to internal buffers
}

// Adapter is the internal HTTP execution boundary. The adapter has no
// knowledge of auth, telemetry, circuit breaking, error classification,
// or response building. It sends bytes and receives bytes.
type Adapter interface {
	// Execute performs one logical HTTP exchange (which may include
	// multiple attempts if Retry is non-nil). No circuit breaker, no
	// error classification, no response building, no telemetry.
	//
	// Error semantics: the adapter returns (RawResponse, nil) for ALL
	// HTTP responses, including 4xx and 5xx. HTTP status codes are NOT
	// errors at the adapter level. The adapter returns (RawResponse{}, err)
	// only for transport-level failures: network errors, DNS, TLS, timeout,
	// context cancellation.
	//
	// The adapter copies all response data before returning.
	Execute(ctx context.Context, req Request) (RawResponse, error)
}

// RemoteClientOption configures a RemoteClient at construction time.
type RemoteClientOption func(*RemoteClient)

// WithHTTPClient sets a custom *http.Client. If injected, the caller owns
// transport instrumentation. requestCore does NOT add otelhttp.
func WithHTTPClient(c *http.Client) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.httpClient = c
	}
}

// WithTelemetry sets the telemetry sink for recording call lifecycle events.
func WithTelemetry(sink telemetry.Sink) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.sink = sink
	}
}

// WithMetrics sets the metrics recorder.
func WithMetrics(r MetricsRecorder) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.metrics = r
	}
}

// WithCircuitBreaker enables circuit breaking with the given policy.
// Circuit breaking is opt-in. Zero values in the policy are normalized
// to defaults (MaxRequests=1, Interval=60s, OpenDuration=60s, FailureThreshold=5).
func WithCircuitBreaker(policy CircuitBreakerPolicy) RemoteClientOption {
	return func(rc *RemoteClient) {
		policy.normalize()
		rc.breakerPolicy = &policy
	}
}

// WithDefaultRetry sets a default retry policy for all calls that don't
// specify their own. Retry is opt-in.
func WithDefaultRetry(policy RetryPolicy) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.defaultRetry = &policy
	}
}

// WithAuthProvider sets the auth provider applied to every call's headers
// before adapter execution.
func WithAuthProvider(provider AuthProvider) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.auth = provider
	}
}

// WithDefaultHeaders sets default headers merged into every call.
func WithDefaultHeaders(headers http.Header) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.defaultHeaders = headers
	}
}

// WithAdapter sets a custom Adapter implementation. If not set, a default
// restyadapter is created from the httpClient. This option is primarily for
// testing — production code should use WithHTTPClient to inject a custom
// *http.Client and let the default adapter be created.
func WithAdapter(a Adapter) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.adapter = a
	}
}

// WithAppName sets the application name used in X-App-ID header.
func WithAppName(name string) RemoteClientOption {
	return func(rc *RemoteClient) {
		rc.appName = name
	}
}

// RemoteClient is the v2 remote call client. It owns all policy and
// semantics, delegating only raw HTTP mechanics to the internal adapter.
type RemoteClient struct {
	httpClient     *http.Client
	adapter        Adapter
	sink           telemetry.Sink
	metrics        MetricsRecorder
	breakerPolicy  *CircuitBreakerPolicy
	defaultRetry   *RetryPolicy
	auth           AuthProvider
	defaultHeaders http.Header
	appName        string

	breakersMu sync.RWMutex
	breakers   map[OperationKey]*CircuitBreaker
}

// defaultAdapterFactory creates a restyadapter from an *http.Client.
// This is set by an init() in internal/restyadapter to avoid a circular
// import. If nil (e.g., in tests that only use WithAdapter), the client
// panics on Call if no adapter is set.
var defaultAdapterFactory func(*http.Client) Adapter

// RegisterDefaultAdapterFactory is called by internal/restyadapter's
// init() to provide the default adapter factory. This breaks the circular
// dependency: remotecall defines the Adapter interface, restyadapter
// implements it and registers its constructor.
func RegisterDefaultAdapterFactory(fn func(*http.Client) Adapter) {
	defaultAdapterFactory = fn
}

// NewRemoteClient creates a new RemoteClient with the given options.
// If no HTTP client is injected, a standard instrumented client is created.
// If no adapter is injected via WithAdapter, a default restyadapter is
// created from the HTTP client.
func NewRemoteClient(opts ...RemoteClientOption) *RemoteClient {
	rc := &RemoteClient{
		sink:     telemetry.NopSink{},
		metrics:  NopRecorder{},
		breakers: make(map[OperationKey]*CircuitBreaker),
	}
	for _, opt := range opts {
		opt(rc)
	}
	if rc.httpClient == nil {
		rc.httpClient = NewInstrumentedClient()
	}
	if rc.adapter == nil {
		if defaultAdapterFactory == nil {
			panic("remotecall: no adapter registered; use WithAdapter or import internal/restyadapter")
		}
		rc.adapter = defaultAdapterFactory(rc.httpClient)
	}
	return rc
}

// Call performs a single logical remote call. It follows the lifecycle:
//
//  1. Validate / derive operation metadata
//  2. Skip check
//  3. Circuit Allow (if enabled)
//  4. Preflight (marshal body, build URL, build headers, auth)
//  5. Emit call-start telemetry
//  6. adapter.Execute (with retry if enabled)
//  7. Classify final result
//  8. Build typed response (2xx only; non-2xx → RemoteCallError)
//  9. Record ONE breaker outcome (if enabled)
//  10. Emit ONE terminal telemetry event
//  11. Record metrics
//  12. Return
func (rc *RemoteClient) Call[Req, Resp any](ctx context.Context, cfg CallConfig[Req, Resp]) (*Resp, error) {
	start := time.Now()

	// 1. Validate / derive operation metadata
	opKey := NewOperationKey(cfg.API.Name, cfg.Method)

	// 2. Skip check
	if ShouldSkipCall(cfg.Path, cfg.Method, cfg.API.SkipPatterns) {
		return nil, ErrSkipped
	}

	// 3. Circuit Allow (if enabled)
	var breaker *CircuitBreaker
	if rc.breakerPolicy != nil {
		breaker = rc.getOrCreateBreaker(opKey)
		if !breaker.Allow() {
			return nil, ErrCircuitOpen
		}
	}

	// 4. Preflight
	bodyBytes, contentType, err := rc.marshalBody(cfg.Body, cfg.BodyType)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal body: %v", ErrPreflight, err)
	}

	fullURL, err := buildURL(cfg.API.BaseURL, cfg.Path, cfg.Query)
	if err != nil {
		return nil, fmt.Errorf("%w: build url: %v", ErrPreflight, err)
	}

	headers, err := buildCallHeaders(ctx, rc.defaultHeaders, HeaderContext{
		AppName: rc.appName,
	}, cfg.Headers, rc.auth)
	if err != nil {
		return nil, fmt.Errorf("%w: auth: %v", ErrPreflight, err)
	}

	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}

	// Check caller context before proceeding
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: context cancelled before execute: %v", ErrPreflight, err)
	}

	// 5. Emit call-start telemetry
	rc.emitCallStart(opKey.String(), cfg.Method, cfg.Path)

	// callerHasDeadline tracks whether the caller's context has a deadline.
	// This distinguishes caller-imposed DeadlineExceeded (ErrorKindContext)
	// from requestCore-owned timeout (ErrorKindTimeout) in classifyError.
	callerHasDeadline := false

	// 6. adapter.Execute
	var retryPolicy *RetryPolicy
	if cfg.Retry != nil {
		retryPolicy = cfg.Retry
	} else if rc.defaultRetry != nil {
		retryPolicy = rc.defaultRetry
	}

	// Create derived timeout context if configured
	execCtx := ctx
	var cancel context.CancelFunc
	if cfg.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	} else {
		if _, ok := ctx.Deadline(); ok {
			callerHasDeadline = true
		}
	}

	// Wire OnAttempt to telemetry
	onAttempt := func(info AttemptInfo) {
		rc.emitAttemptComplete(opKey.String(), info)
	}

	req := Request{
		Method:    cfg.Method,
		URL:       fullURL,
		Headers:   headers,
		Body:      bodyBytes,
		Retry:     retryPolicy,
		OnAttempt: onAttempt,
	}

	raw, execErr := rc.adapter.Execute(execCtx, req)

	// 7. Classify final result
	var callErr *RemoteCallError
	var resp *Resp

	if execErr != nil {
		kind := classifyError(execErr, callerHasDeadline)
		callErr = &RemoteCallError{
			Kind:  kind,
			Err:   execErr,
			OpKey: opKey.String(),
		}
	} else if raw.StatusCode < 200 || raw.StatusCode >= 300 {
		// Non-2xx → RemoteCallError (builder NOT called)
		callErr = &RemoteCallError{
			Kind:         ErrorKindHTTP,
			StatusCode:   raw.StatusCode,
			ResponseBody: raw.Body,
			Headers:      raw.Headers,
			OpKey:        opKey.String(),
		}
	} else {
		// 2xx → ResponseBuilder
		resp, err = cfg.Builder.Build(raw)
		if err != nil {
			callErr = &RemoteCallError{
				Kind:         ErrorKindDecode,
				StatusCode:   raw.StatusCode,
				ResponseBody: raw.Body,
				Headers:      raw.Headers,
				Err:          err,
				OpKey:        opKey.String(),
			}
		}
	}

	// 8. Record breaker outcome (if enabled)
	// Breaker success/failure ≠ logical call success/failure.
	// Breaker answers dependency health, not application call success.
	if breaker != nil && callErr == nil {
		// Success: HTTP 2xx + successful build, or HTTP 4xx
		breaker.RecordSuccess()
	} else if breaker != nil && callErr != nil {
		switch callErr.Kind {
		case ErrorKindTransport, ErrorKindTimeout:
			// Failure: remote service unreachable or too slow
			breaker.RecordFailure()
		case ErrorKindHTTP:
			if callErr.StatusCode >= 500 {
				// Failure: remote service error
				breaker.RecordFailure()
			} else {
				// Success: 4xx is a valid response, service is healthy
				breaker.RecordSuccess()
			}
		case ErrorKindContext, ErrorKindDecode:
			// No outcome: caller behavior or client-side contract problem
		}
	}

	// 9. Emit terminal telemetry
	duration := time.Since(start)
	if callErr != nil {
		rc.emitCallFailure(opKey.String(), callErr)
	} else {
		rc.emitCallSuccess(opKey.String(), raw.StatusCode, duration)
	}

	// 10. Record metrics
	outcome := "success"
	if callErr != nil {
		outcome = "failure"
	}
	rc.metrics.RecordCall(opKey.String(), duration, outcome)

	// 11. Return
	if callErr != nil {
		return nil, callErr
	}
	return resp, nil
}

// getOrCreateBreaker lazily creates a breaker for an operation key.
func (rc *RemoteClient) getOrCreateBreaker(opKey OperationKey) *CircuitBreaker {
	rc.breakersMu.RLock()
	cb, ok := rc.breakers[opKey]
	rc.breakersMu.RUnlock()
	if ok {
		return cb
	}
	rc.breakersMu.Lock()
	defer rc.breakersMu.Unlock()
	// Double-check after acquiring write lock
	if cb, ok := rc.breakers[opKey]; ok {
		return cb
	}
	cb = NewCircuitBreaker(*rc.breakerPolicy)
	rc.breakers[opKey] = cb
	return cb
}

// marshalBody converts the typed Body into []byte based on BodyType.
func (rc *RemoteClient) marshalBody(body any, bodyType RequestBodyType) ([]byte, string, error) {
	switch bodyType {
	case BodyTypeJSON:
		b, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		return b, "application/json", nil

	case BodyTypeForm:
		values, ok := body.(url.Values)
		if !ok {
			return nil, "", fmt.Errorf("remotecall: form body must be url.Values, got %T", body)
		}
		return []byte(values.Encode()), "application/x-www-form-urlencoded", nil

	case BodyTypeEmpty:
		return nil, "", nil

	default:
		return nil, "", fmt.Errorf("remotecall: unknown body type %d", bodyType)
	}
}

// buildURL constructs the full URL from base, path, and query parameters.
func buildURL(baseURL, path string, query url.Values) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if path != "" {
		u = u.JoinPath(path)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

// emitCallStart records a call-start telemetry event.
func (rc *RemoteClient) emitCallStart(opKey, method, path string) {
	rc.sink.Record(telemetry.Event{
		Type:         telemetry.EventStart,
		Operation:    opKey + "-req",
		Method:       method,
		RoutePattern: path,
	})
}

// emitCallSuccess records a call-success telemetry event.
func (rc *RemoteClient) emitCallSuccess(opKey string, status int, duration time.Duration) {
	rc.sink.Record(telemetry.Event{
		Type:      telemetry.EventSuccess,
		Operation: opKey + "-req",
		Status:    status,
		Duration:  duration,
	})
}

// emitCallFailure records a call-failure telemetry event.
func (rc *RemoteClient) emitCallFailure(opKey string, callErr *RemoteCallError) {
	rc.sink.Record(telemetry.Event{
		Type:      telemetry.EventFailure,
		Operation: opKey + "-req-failed",
		Err:       callErr,
	})
}

// emitAttemptComplete records an attempt-complete telemetry event.
// Telemetry may omit the first attempt (N=1) since call-start covers it.
func (rc *RemoteClient) emitAttemptComplete(opKey string, info AttemptInfo) {
	if info.Attempt <= 1 {
		return // omit first attempt
	}
	rc.sink.Record(telemetry.Event{
		Type:      telemetry.EventBusiness,
		Operation: fmt.Sprintf("%s-attempt-%d", opKey, info.Attempt),
		Status:    info.StatusCode,
		Duration:  info.Duration,
		Err:       info.Err,
	})
}

// BuildTimeoutError creates a standardized error for server-side timeout
// conditions. The domain is included for context.
// This is the v2 replacement for handlers.BuildTimeoutError.
func BuildTimeoutError(domain string, statusCode int) error {
	if statusCode == 0 {
		statusCode = http.StatusRequestTimeout
	}
	return fmt.Errorf("remotecall: timeout: %s: elapsed time exceeds timeout (status %d)", domain, statusCode)
}

// NormalizeCallError converts a raw remote-call error into a standardized
// error with a consistent description. This is the v2 replacement for
// handlers.NormalizeCallError.
func NormalizeCallError(err error) error {
	if err == nil {
		return nil
	}
	var rce *RemoteCallError
	if errors.As(err, &rce) {
		return fmt.Errorf("remotecall: API_CALL_ERROR: HTTP %d: %s", rce.StatusCode, string(rce.ResponseBody))
	}
	return err
}
