package httpsemantics

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAfter represents a parsed RFC 9110 Retry-After header value.
// It is either a delta-seconds (non-negative integer) or an HTTP-date.
type RetryAfter struct {
	// Delta is the delay in seconds. Valid only when IsDate is false.
	Delta int

	// Date is the retry time. Valid only when IsDate is true.
	Date time.Time

	// IsDate indicates whether the value is an HTTP-date form.
	IsDate bool
}

// Duration returns the effective delay duration. For delta-seconds,
// it returns Delta seconds. For HTTP-date, it returns the duration
// from now until the date (clamped to non-negative). The clock
// parameter allows injecting a time source for deterministic tests.
func (r RetryAfter) Duration(now time.Time) time.Duration {
	if r.IsDate {
		d := r.Date.Sub(now)
		if d < 0 {
			return 0
		}
		return d
	}
	return time.Duration(r.Delta) * time.Second
}

// ParseRetryAfter parses an RFC 9110 Retry-After header value. Supports
// both delta-seconds (integer) and HTTP-date (RFC 7231 IMF-fixdate)
// forms. Returns an error for malformed input.
func ParseRetryAfter(s string) (RetryAfter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return RetryAfter{}, fmt.Errorf("httpsemantics: empty Retry-After")
	}

	// Try delta-seconds first (most common)
	if delta, err := strconv.Atoi(s); err == nil {
		if delta < 0 {
			return RetryAfter{}, fmt.Errorf("httpsemantics: negative Retry-After: %d", delta)
		}
		return RetryAfter{Delta: delta}, nil
	}

	// Try HTTP-date
	t, err := http.ParseTime(s)
	if err != nil {
		return RetryAfter{}, fmt.Errorf("httpsemantics: invalid Retry-After: %q", s)
	}

	return RetryAfter{Date: t, IsDate: true}, nil
}

// FormatRetryAfterDelta formats a delta-seconds value as an RFC 9110
// Retry-After header string.
func FormatRetryAfterDelta(seconds int) string {
	return strconv.Itoa(seconds)
}

// FormatRetryAfterDate formats a time.Time as an RFC 9110 Retry-After
// HTTP-date header string (RFC 7231 IMF-fixdate format).
func FormatRetryAfterDate(t time.Time) string {
	return t.UTC().Format(http.TimeFormat)
}

// ClampRetryAfter caps a parsed Retry-After duration to a maximum
// delay. This prevents denial-of-service via excessively large
// Retry-After values. Returns the clamped duration in seconds.
func ClampRetryAfter(ra RetryAfter, now time.Time, maxDelay time.Duration) int {
	d := ra.Duration(now)
	if maxDelay > 0 && d > maxDelay {
		d = maxDelay
	}
	return int(d.Seconds())
}

// FormatRetryAfter formats a parsed RetryAfter as a header string,
// clamped to maxDelay seconds. If maxDelay is 0, no clamping is applied.
func FormatRetryAfter(ra RetryAfter, now time.Time, maxDelay time.Duration) string {
	if ra.IsDate {
		d := ra.Duration(now)
		if maxDelay > 0 && d > maxDelay {
			return FormatRetryAfterDelta(int(maxDelay.Seconds()))
		}
		return FormatRetryAfterDate(ra.Date)
	}
	if maxDelay > 0 && time.Duration(ra.Delta)*time.Second > maxDelay {
		return FormatRetryAfterDelta(int(maxDelay.Seconds()))
	}
	return FormatRetryAfterDelta(ra.Delta)
}
