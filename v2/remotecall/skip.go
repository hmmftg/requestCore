package remotecall

import "strings"

// ShouldSkipCall reports whether an endpoint matches one of the supplied
// skip patterns. It is the v2 replacement for handlers.ShouldSkipAPICall.
//
// Pattern grammar:
//   - "endpoint"           matches any HTTP method
//   - "endpoint:METHOD"    matches only when the call's method equals METHOD
//
// Matching rules:
//   - Patterns are trimmed of surrounding whitespace; empty patterns are ignored.
//   - A pattern containing more than one ':' separator is malformed and is ignored.
//   - Method comparison is case-insensitive.
//   - Endpoint matching is either an exact match or a path-boundary prefix match.
//   - Leading and trailing slashes are normalized away from both the endpoint
//     and the pattern endpoint before comparison.
func ShouldSkipCall(endpoint, method string, skipPatterns []string) bool {
	normalizedEndpoint := normalizePath(endpoint)
	for _, raw := range skipPatterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
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
		if !methodConstrained {
			return true
		}
		if strings.EqualFold(patternMethod, method) {
			return true
		}
	}
	return false
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimRight(p, "/")
	return p
}

func endpointMatches(endpoint, patternEndpoint string) bool {
	if endpoint == patternEndpoint {
		return true
	}
	return strings.HasPrefix(endpoint, patternEndpoint+"/")
}
