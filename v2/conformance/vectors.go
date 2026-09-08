// Package conformance provides shared data-only HTTP conformance vectors
// for cross-version testing of requestCore v1 and v2. The vectors are
// framework-neutral and test response status, headers, tracing, and
// security behavior without depending on either module's internal types.
//
// Usage:
//
//	for _, tc := range conformance.BearerChallengeVectors {
//	    // run tc against both v1 and v2 implementations
//	}
package conformance

import (
	"net/http"
	"time"
)

// BearerChallengeVector tests RFC 6750 WWW-Authenticate Bearer challenge
// formatting and header injection prevention.
type BearerChallengeVector struct {
	Name        string
	Realm       string
	Error       string
	Description string
	URI         string
	Scope       string
	WantHeader  string
}

// BearerChallengeVectors is the shared set of Bearer challenge test
// vectors for v1 and v2.
var BearerChallengeVectors = []BearerChallengeVector{
	{
		Name:       "default_realm",
		WantHeader: `Bearer realm="Protected"`,
	},
	{
		Name:       "custom_realm",
		Realm:      "My API",
		WantHeader: `Bearer realm="My API"`,
	},
	{
		Name:        "invalid_token",
		Error:       "invalid_token",
		Description: "The access token expired",
		URI:         "https://example.com/oauth/errors",
		WantHeader:  `Bearer realm="Protected", error="invalid_token", error_description="The access token expired", error_uri="https://example.com/oauth/errors"`,
	},
	{
		Name:       "insufficient_scope",
		Error:      "insufficient_scope",
		Scope:      "read write admin",
		WantHeader: `Bearer realm="Protected", error="insufficient_scope", scope="read write admin"`,
	},
	{
		Name:       "missing_token",
		Error:      "missing_token",
		WantHeader: `Bearer realm="Protected", error="missing_token"`,
	},
}

// RetryAfterVector tests RFC 9110 Retry-After parsing and formatting.
type RetryAfterVector struct {
	Name       string
	Header     string
	WantDelta  int
	WantIsDate bool
	WantError  bool
}

// RetryAfterVectors is the shared set of Retry-After test vectors.
var RetryAfterVectors = []RetryAfterVector{
	{Name: "delta_120", Header: "120", WantDelta: 120},
	{Name: "delta_zero", Header: "0", WantDelta: 0},
	{Name: "http_date", Header: "Tue, 21 Oct 2025 07:28:00 GMT", WantIsDate: true},
	{Name: "empty", Header: "", WantError: true},
	{Name: "negative", Header: "-1", WantError: true},
	{Name: "malformed", Header: "not a date or number", WantError: true},
}

// ETagVector tests RFC 9110 ETag parsing and comparison.
type ETagVector struct {
	Name    string
	Input   string
	WantWeak bool
	WantValue string
	WantError bool
}

// ETagVectors is the shared set of ETag test vectors.
var ETagVectors = []ETagVector{
	{Name: "strong", Input: `"abc123"`, WantValue: "abc123"},
	{Name: "weak", Input: `W/"abc123"`, WantWeak: true, WantValue: "abc123"},
	{Name: "empty", Input: "", WantError: true},
	{Name: "malformed", Input: "abc123", WantError: true},
}

// PreconditionVector tests RFC 9110 precondition evaluation.
type PreconditionVector struct {
	Name             string
	IfMatch          string
	IfNoneMatch      string
	IfModifiedSince  string
	IfUnmodifiedSince string
	ResourceETag     string
	ResourceModified time.Time
	IsSafeMethod     bool
	WantResult       int // 0=Proceed, 1=NotModified, 2=Failed
}

// PreconditionVectors is the shared set of precondition test vectors.
var PreconditionVectors = []PreconditionVector{
	{
		Name:         "no_preconditions",
		IsSafeMethod: true,
		WantResult:   0,
	},
	{
		Name:         "if_match_match",
		IfMatch:      `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: true,
		WantResult:   0,
	},
	{
		Name:         "if_match_no_match",
		IfMatch:      `"abc"`,
		ResourceETag: `"def"`,
		IsSafeMethod: true,
		WantResult:   2,
	},
	{
		Name:         "if_match_wildcard",
		IfMatch:      `*`,
		ResourceETag: `"anything"`,
		IsSafeMethod: true,
		WantResult:   0,
	},
	{
		Name:         "if_none_match_safe_304",
		IfNoneMatch:  `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: true,
		WantResult:   1,
	},
	{
		Name:         "if_none_match_unsafe_412",
		IfNoneMatch:  `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: false,
		WantResult:   2,
	},
}

// IdempotencyKeyVector tests idempotency key validation.
type IdempotencyKeyVector struct {
	Name     string
	Key      string
	WantError bool
}

// IdempotencyKeyVectors is the shared set of idempotency key test vectors.
var IdempotencyKeyVectors = []IdempotencyKeyVector{
	{Name: "valid_simple", Key: "abc123"},
	{Name: "valid_uuid", Key: "550e8400-e29b-41d4-a716-446655440000"},
	{Name: "valid_single_char", Key: "a"},
	{Name: "empty", Key: "", WantError: true},
	{Name: "whitespace_only", Key: "   ", WantError: true},
	{Name: "non_printable", Key: "abc\x00def", WantError: true},
}

// SecurityVector tests that sensitive data is not leaked in responses
// or telemetry.
type SecurityVector struct {
	Name           string
	SensitiveData  string
	CheckInJSON    bool
}

// SecurityVectors is the shared set of security test vectors.
var SecurityVectors = []SecurityVector{
	{Name: "password", SensitiveData: "password=hunter2", CheckInJSON: true},
	{Name: "token", SensitiveData: "token=abc123", CheckInJSON: true},
	{Name: "connection_string", SensitiveData: "postgres://user:pass@host:5432/db", CheckInJSON: true},
	{Name: "internal_error", SensitiveData: "database password is secret123", CheckInJSON: true},
}

// NoBodyStatusVector tests that no-body statuses suppress the response body.
type NoBodyStatusVector struct {
	Name   string
	Status int
	WantNoBody bool
}

// NoBodyStatusVectors is the shared set of no-body status test vectors.
var NoBodyStatusVectors = []NoBodyStatusVector{
	{Name: "200", Status: http.StatusOK, WantNoBody: false},
	{Name: "201", Status: http.StatusCreated, WantNoBody: false},
	{Name: "204", Status: http.StatusNoContent, WantNoBody: true},
	{Name: "304", Status: http.StatusNotModified, WantNoBody: true},
}

// PaginationLinkVector tests RFC 8288 pagination link generation.
type PaginationLinkVector struct {
	Name           string
	Page           int
	PageSize       int
	TotalItems     int
	WantRels       []string // expected rel types
	WantNoRels     []string // rel types that should NOT be present
}

// PaginationLinkVectors is the shared set of pagination link test vectors.
var PaginationLinkVectors = []PaginationLinkVector{
	{
		Name:       "middle_page",
		Page:       3,
		PageSize:   10,
		TotalItems: 50,
		WantRels:   []string{"first", "prev", "next", "last"},
		WantNoRels: []string{},
	},
	{
		Name:       "first_page",
		Page:       1,
		PageSize:   10,
		TotalItems: 50,
		WantRels:   []string{"first", "next", "last"},
		WantNoRels: []string{"prev"},
	},
	{
		Name:       "last_page",
		Page:       5,
		PageSize:   10,
		TotalItems: 50,
		WantRels:   []string{"first", "prev", "last"},
		WantNoRels: []string{"next"},
	},
	{
		Name:       "unknown_total",
		Page:       1,
		PageSize:   10,
		TotalItems: 0,
		WantRels:   []string{"first", "next"},
		WantNoRels: []string{"last"},
	},
}
