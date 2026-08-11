package handlers

import (
	"strings"
)

// ShouldSkipAPICall reports whether an endpoint matches one of the supplied
// skip patterns. It is an application-layer decision helper only: it must not
// be invoked by CallAPIJSONWithOpts to control webFramework.AddLog, and a
// match must never be used to suppress the mandatory request, success, or
// failure framework logs.
//
// Pattern grammar:
//   - "endpoint"           matches any HTTP method
//   - "endpoint:METHOD"    matches only when the call's method equals METHOD
//
// Matching rules:
//   - Patterns are trimmed of surrounding whitespace; empty patterns are
//     ignored.
//   - A pattern containing more than one ':' separator is malformed and is
//     ignored (it never matches), rather than being treated as a broad match.
//   - Method comparison is case-insensitive.
//   - Endpoint matching is either an exact match or a path-boundary prefix
//     match: the endpoint equals the pattern endpoint, or the endpoint begins
//     with patternEndpoint + "/". Substring matching is never used, so
//     "api/card" does not match "api/cardholder".
//   - Leading and trailing slashes are normalized away from both the endpoint
//     and the pattern endpoint before comparison, so "/api/card/",
//     "api/card", and "/api/card" are equivalent.
func ShouldSkipAPICall(endpoint, method string, skipPatterns []string) bool {
	normalizedEndpoint := normalizePath(endpoint)
	for _, raw := range skipPatterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		// Reject patterns with more than one ':' separator as malformed.
		if strings.Count(pattern, ":") > 1 {
			continue
		}
		var patternEndpoint, patternMethod string
		methodConstrained := false
		if idx := strings.Index(pattern, ":"); idx >= 0 {
			patternEndpoint = pattern[:idx]
			patternMethod = pattern[idx+1:]
			methodConstrained = true
		} else {
			patternEndpoint = pattern
		}
		patternEndpoint = normalizePath(patternEndpoint)
		if patternEndpoint == "" {
			continue
		}
		if !endpointMatches(normalizedEndpoint, patternEndpoint) {
			continue
		}
		// A pattern without a method constraint matches any method.
		if !methodConstrained {
			return true
		}
		if strings.EqualFold(patternMethod, method) {
			return true
		}
	}
	return false
}

// normalizePath strips a single leading and all trailing slashes so that
// "/api/card/", "api/card/", and "/api/card" collapse to "api/card".
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimRight(p, "/")
	return p
}

// endpointMatches reports whether endpoint equals patternEndpoint or begins
// with patternEndpoint + "/" at a path boundary. Both inputs must already be
// normalized via normalizePath.
func endpointMatches(endpoint, patternEndpoint string) bool {
	if endpoint == patternEndpoint {
		return true
	}
	return strings.HasPrefix(endpoint, patternEndpoint+"/")
}
