package httpsemantics

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ETag represents an RFC 9110 entity tag with optional weak indicator.
type ETag struct {
	// Weak indicates whether this is a weak entity tag (W/ prefix).
	Weak bool

	// Value is the opaque entity-tag value without the surrounding
	// double quotes or W/ prefix.
	Value string
}

// String formats the ETag as an RFC 9110 entity-tag value:
// `W/"value"` for weak, `"value"` for strong.
func (e ETag) String() string {
	if e.Weak {
		return fmt.Sprintf(`W/"%s"`, e.Value)
	}
	return fmt.Sprintf(`"%s"`, e.Value)
}

// StrongEqual reports whether two ETags are strongly equal per
// RFC 9110 §8.8.3.2: both must be strong and their values must match.
func (e ETag) StrongEqual(other ETag) bool {
	return !e.Weak && !other.Weak && e.Value == other.Value
}

// WeakEqual reports whether two ETags are weakly equal per
// RFC 9110 §8.8.3.2: their values must match, regardless of weakness.
func (e ETag) WeakEqual(other ETag) bool {
	return e.Value == other.Value
}

// ParseETag parses a single RFC 9110 entity-tag value. Supports both
// strong ("value") and weak (W/"value") forms. Returns an error for
// malformed input.
func ParseETag(s string) (ETag, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ETag{}, fmt.Errorf("httpsemantics: empty entity tag")
	}

	weak := false
	if strings.HasPrefix(s, "W/") {
		weak = true
		s = s[2:]
	}

	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return ETag{}, fmt.Errorf("httpsemantics: malformed entity tag: %q", s)
	}

	value := s[1 : len(s)-1]
	if strings.ContainsAny(value, `"`) {
		return ETag{}, fmt.Errorf("httpsemantics: unescaped quote in entity tag: %q", s)
	}

	return ETag{Weak: weak, Value: value}, nil
}

// ParseETagList parses a comma-separated list of entity tags from a
// header value (e.g. If-Match, If-None-Match). Returns the parsed
// ETags. The wildcard "*" is represented as ETag{Value: "*"}.
func ParseETagList(s string) ([]ETag, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	if s == "*" {
		return []ETag{{Value: "*"}}, nil
	}

	parts := strings.Split(s, ",")
	tags := make([]ETag, 0, len(parts))
	for _, part := range parts {
		tag, err := ParseETag(part)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// FormatETagList formats a slice of ETags as a comma-separated header
// value.
func FormatETagList(tags []ETag) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = t.String()
	}
	return strings.Join(parts, ", ")
}

// PreconditionResult indicates the outcome of precondition evaluation.
type PreconditionResult int

const (
	// PreconditionProceed means the request should proceed normally.
	PreconditionProceed PreconditionResult = iota

	// PreconditionNotModified means the server should respond with 304
	// Not Modified.
	PreconditionNotModified

	// PreconditionFailed means the server should respond with 412
	// Precondition Failed.
	PreconditionFailed

	// PreconditionRequired means the server should respond with 428
	// Precondition Required (the resource requires a precondition
	// that was not provided).
	PreconditionRequired
)

// PreconditionInput holds the request headers and resource state needed
// to evaluate RFC 9110 preconditions.
type PreconditionInput struct {
	// IfMatch is the value of the If-Match header. Empty if absent.
	IfMatch string

	// IfNoneMatch is the value of the If-None-Match header. Empty if absent.
	IfNoneMatch string

	// IfModifiedSince is the value of the If-Modified-Since header.
	// Empty if absent.
	IfModifiedSince string

	// IfUnmodifiedSince is the value of the If-Unmodified-Since header.
	// Empty if absent.
	IfUnmodifiedSince string

	// ResourceETag is the current entity tag of the resource.
	// Empty if the resource has no ETag.
	ResourceETag string

	// ResourceModified is the last modification time of the resource.
	// Zero if the resource has no Last-Modified.
	ResourceModified time.Time

	// IsSafeMethod is true for GET and HEAD (where If-None-Match can
	// produce 304 rather than 412).
	IsSafeMethod bool
}

// EvaluatePreconditions evaluates RFC 9110 §13.1.2 precondition headers
// in precedence order and returns the appropriate result.
//
// Precedence (RFC 9110 §13.2.2):
//  1. If-Match
//  2. If-Unmodified-Since
//  3. If-None-Match
//  4. If-Modified-Since
//
// If-Match and If-Unmodified-Since produce 412 on failure.
// If-None-Match and If-Modified-Since produce 304 on failure for safe
// methods, 412 for unsafe methods.
func EvaluatePreconditions(in PreconditionInput) PreconditionResult {
	// 1. If-Match
	if in.IfMatch != "" {
		tags, err := ParseETagList(in.IfMatch)
		if err != nil {
			return PreconditionFailed
		}
		if !matchETag(tags, in.ResourceETag) {
			return PreconditionFailed
		}
	}

	// 2. If-Unmodified-Since
	if in.IfUnmodifiedSince != "" {
		since, err := http.ParseTime(in.IfUnmodifiedSince)
		if err != nil {
			return PreconditionFailed
		}
		if !in.ResourceModified.IsZero() && in.ResourceModified.After(since) {
			return PreconditionFailed
		}
	}

	// 3. If-None-Match
	if in.IfNoneMatch != "" {
		tags, err := ParseETagList(in.IfNoneMatch)
		if err != nil {
			return PreconditionFailed
		}
		if matchETag(tags, in.ResourceETag) {
			if in.IsSafeMethod {
				return PreconditionNotModified
			}
			return PreconditionFailed
		}
	}

	// 4. If-Modified-Since
	if in.IfModifiedSince != "" {
		since, err := http.ParseTime(in.IfModifiedSince)
		if err != nil {
			return PreconditionFailed
		}
		if in.IsSafeMethod && !in.ResourceModified.IsZero() && !in.ResourceModified.After(since) {
			return PreconditionNotModified
		}
	}

	return PreconditionProceed
}

// matchETag checks whether the resource ETag matches any tag in the
// provided list. The wildcard "*" matches any non-empty resource ETag.
func matchETag(tags []ETag, resourceETag string) bool {
	if resourceETag == "" {
		return false
	}

	resourceTag, err := ParseETag(resourceETag)
	if err != nil {
		return false
	}

	for _, tag := range tags {
		if tag.Value == "*" {
			return true
		}
		if tag.WeakEqual(resourceTag) {
			return true
		}
	}
	return false
}

// IsNoBodyStatus reports whether the given HTTP status code should not
// have a response body per RFC 9110.
func IsNoBodyStatus(status int) bool {
	return status == http.StatusNoContent ||
		status == http.StatusResetContent ||
		status == http.StatusNotModified
}
