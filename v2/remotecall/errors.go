// Package remotecall provides a clean remote-call API where requestCore owns
// all policy and semantics (retry eligibility, circuit breaking, error
// classification, telemetry, auth, response building) and delegates only raw
// HTTP mechanics to an internal adapter backed by go-resty/resty/v3.
//
// RemoteCallError is the error type returned by RemoteClient.Call. It wraps
// an ErrorKind classification, the HTTP status code (when applicable), the
// raw response body and headers (as owned copies), and the underlying error
// for errors.Is/errors.As chaining.
package remotecall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/v2/response"
)

// ErrorKind classifies the kind of failure that occurred during a remote call.
type ErrorKind int

const (
	// ErrorKindHTTP indicates a non-2xx HTTP response was received.
	// StatusCode and ResponseBody are populated.
	ErrorKindHTTP ErrorKind = iota

	// ErrorKindTransport indicates a network-level failure (DNS, connection,
	// TLS). No HTTP response was received.
	ErrorKindTransport

	// ErrorKindTimeout indicates a requestCore-owned timeout or client-side
	// per-call timeout. No HTTP response was received.
	ErrorKindTimeout

	// ErrorKindContext indicates the caller's context was canceled or the
	// caller's deadline was exceeded. No HTTP response was received.
	ErrorKindContext

	// ErrorKindDecode indicates the response body could not be parsed by the
	// ResponseBuilder. An HTTP response was received but the builder failed.
	ErrorKindDecode
)

// String returns a human-readable name for the error kind.
func (k ErrorKind) String() string {
	switch k {
	case ErrorKindHTTP:
		return "http"
	case ErrorKindTransport:
		return "transport"
	case ErrorKindTimeout:
		return "timeout"
	case ErrorKindContext:
		return "context"
	case ErrorKindDecode:
		return "decode"
	default:
		return "unknown"
	}
}

// RemoteCallError is the error type returned by RemoteClient.Call when a
// call fails. It preserves the HTTP status code, raw response body, and
// response headers as independent owned copies that may safely be retained
// by the caller. Ownership propagates from RawResponse through RemoteCallError.
type RemoteCallError struct {
	// Kind classifies the failure.
	Kind ErrorKind

	// StatusCode is the HTTP status code. Valid only for ErrorKindHTTP.
	StatusCode int

	// ResponseBody is the raw response body as an owned copy. Valid for
	// ErrorKindHTTP and ErrorKindDecode.
	ResponseBody []byte

	// Headers are the response headers as an owned copy. Valid for
	// ErrorKindHTTP and ErrorKindDecode.
	Headers http.Header

	// Err is the underlying error for errors.Unwrap chaining.
	Err error

	// OpKey is the canonical operation key for this call.
	OpKey string
}

// Error implements the error interface.
func (e *RemoteCallError) Error() string {
	switch e.Kind {
	case ErrorKindHTTP:
		return fmt.Sprintf("remotecall: http %d: %s", e.StatusCode, string(e.ResponseBody))
	case ErrorKindTransport:
		return fmt.Sprintf("remotecall: transport: %v", e.Err)
	case ErrorKindTimeout:
		return fmt.Sprintf("remotecall: timeout: %v", e.Err)
	case ErrorKindContext:
		return fmt.Sprintf("remotecall: context: %v", e.Err)
	case ErrorKindDecode:
		return fmt.Sprintf("remotecall: decode: %v", e.Err)
	default:
		return fmt.Sprintf("remotecall: unknown: %v", e.Err)
	}
}

// Unwrap returns the underlying error for errors.Is and errors.As support.
func (e *RemoteCallError) Unwrap() error {
	return e.Err
}

// IsHTTPError reports whether err is a RemoteCallError with Kind == ErrorKindHTTP.
func IsHTTPError(err error) bool {
	var rce *RemoteCallError
	return errors.As(err, &rce) && rce.Kind == ErrorKindHTTP
}

// IsTransportError reports whether err is a RemoteCallError with Kind == ErrorKindTransport.
func IsTransportError(err error) bool {
	var rce *RemoteCallError
	return errors.As(err, &rce) && rce.Kind == ErrorKindTransport
}

// IsTimeoutError reports whether err is a RemoteCallError with Kind == ErrorKindTimeout.
func IsTimeoutError(err error) bool {
	var rce *RemoteCallError
	return errors.As(err, &rce) && rce.Kind == ErrorKindTimeout
}

// IsContextCancelled reports whether err is a RemoteCallError with Kind == ErrorKindContext.
func IsContextCancelled(err error) bool {
	var rce *RemoteCallError
	return errors.As(err, &rce) && rce.Kind == ErrorKindContext
}

// IsDecodeError reports whether err is a RemoteCallError with Kind == ErrorKindDecode.
func IsDecodeError(err error) bool {
	var rce *RemoteCallError
	return errors.As(err, &rce) && rce.Kind == ErrorKindDecode
}

// ToProblem converts the RemoteCallError to an RFC 9457 Problem. It is a
// converter method, not an automatic normalization — the caller decides when
// to call it.
//
// If the response Content-Type is application/problem+json (parameters like
// charset ignored), ToProblem parses the upstream problem body and preserves
// its type, title, detail, instance, and extensions.
//
// If the response is not problem+json, ToProblem constructs a minimal problem:
//   - type: empty string
//   - title: HTTP status text for StatusCode
//   - status: StatusCode
//   - detail: response body only when safely representable (text, reasonable length); otherwise empty
//   - instance: empty
//
// If Kind != ErrorKindHTTP, ToProblem returns an error (problem details are
// HTTP-specific). Content-Type matching ignores parameters (e.g.,
// application/problem+json; charset=utf-8 matches).
func (e *RemoteCallError) ToProblem() (*response.Problem, error) {
	if e.Kind != ErrorKindHTTP {
		return nil, errors.New("remotecall: ToProblem requires ErrorKindHTTP")
	}

	problem := response.NewProblem(e.StatusCode, http.StatusText(e.StatusCode))

	if isProblemJSON(e.Headers) {
		// Parse the upstream problem body.
		parsed, err := parseProblemBody(e.ResponseBody, e.StatusCode)
		if err == nil {
			return parsed, nil
		}
		// Fall through to minimal problem on parse failure.
	}

	// Minimal fallback: include body as detail only when safely representable.
	if detail := safeDetail(e.ResponseBody); detail != "" {
		problem = problem.WithDetail(detail)
	}

	return problem, nil
}

// isProblemJSON checks whether the Content-Type header is
// application/problem+json, ignoring parameters like charset.
func isProblemJSON(headers http.Header) bool {
	ct := headers.Get("Content-Type")
	if ct == "" {
		return false
	}
	// Strip parameters.
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return strings.EqualFold(ct, response.ProblemContentType)
}

// parseProblemBody parses an RFC 9457 problem+json body.
func parseProblemBody(body []byte, status int) (*response.Problem, error) {
	if len(body) == 0 {
		return nil, errors.New("empty body")
	}
	p := &response.Problem{}
	if err := json.Unmarshal(body, p); err != nil {
		return nil, fmt.Errorf("remotecall: parse problem+json: %w", err)
	}
	if p.Status == 0 {
		p.Status = status
	}
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = http.StatusText(status)
	}
	return p, nil
}

// safeDetail returns the body as a string only when it is safely
// representable (text, reasonable length).
func safeDetail(body []byte) string {
	if len(body) == 0 || len(body) > 4096 {
		return ""
	}
	// Check for binary content by looking for null bytes.
	for _, b := range body {
		if b == 0 {
			return ""
		}
	}
	return string(body)
}

// ErrSkipped is returned when a call is skipped due to a skip pattern match.
var ErrSkipped = errors.New("remotecall: call skipped")

// ErrCircuitOpen is returned when the circuit breaker rejects a call.
var ErrCircuitOpen = errors.New("remotecall: circuit open")

// ErrPreflight is returned when a preflight failure occurs (body marshal,
// header build, auth). It is not exported as a sentinel; callers check
// errors.Is for specific causes.
var ErrPreflight = errors.New("remotecall: preflight failure")

// classifyError maps an adapter error into an ErrorKind.
// The callerDeadlineExceeded parameter distinguishes caller-imposed
// context.DeadlineExceeded from requestCore-owned timeout.
func classifyError(err error, callerDeadlineExceeded bool) ErrorKind {
	if err == nil {
		return ErrorKindHTTP // shouldn't happen, but default
	}
	if errors.Is(err, context.Canceled) {
		return ErrorKindContext
	}
	if errors.Is(err, context.DeadlineExceeded) {
		if callerDeadlineExceeded {
			return ErrorKindContext
		}
		return ErrorKindTimeout
	}
	return ErrorKindTransport
}
