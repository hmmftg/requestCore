package libCallApi

import (
	"net/url"
	"strings"
)

// ExtractTrackerID separates a URL from its trackerId query value.
// It returns the URL without its query string as cleanEndpoint and the first
// trackerId query value as trackerID. When the URL has no query string, no
// trackerId parameter is present, or the trackerId value is empty, trackerID
// is empty and cleanEndpoint is the original URL with its query string removed.
//
// The lookup is case-insensitive (trackerId, TrackerId, TRACKERID all match).
// The cleanEndpoint preserves the original scheme/path/host exactly; it is not
// normalized, because transaction-log or metric code may consume it verbatim.
//
// Malformed query encoding does not cause a panic: the clean endpoint is still
// returned and trackerID is empty.
func ExtractTrackerID(rawURL string) (cleanEndpoint, trackerID string) {
	qIdx := strings.Index(rawURL, "?")
	if qIdx < 0 {
		return rawURL, ""
	}
	cleanEndpoint = rawURL[:qIdx]
	if qIdx == len(rawURL)-1 {
		// trailing '?' with no query content
		return cleanEndpoint, ""
	}
	values, err := url.ParseQuery(rawURL[qIdx+1:])
	if err != nil {
		// Malformed query encoding: return the clean endpoint without panicking.
		return cleanEndpoint, ""
	}
	for key, vals := range values {
		if !strings.EqualFold(key, "trackerId") {
			continue
		}
		if len(vals) == 0 {
			return cleanEndpoint, ""
		}
		// First value wins when the parameter is repeated.
		return cleanEndpoint, vals[0]
	}
	return cleanEndpoint, ""
}
