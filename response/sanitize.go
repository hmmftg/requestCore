package response

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxDescriptionLength is the maximum length of error description sent to API clients.
const MaxDescriptionLength = 512

// Sensitive pattern: long base64-like or token-like strings (redact with placeholder).
var sensitiveTokenPattern = regexp.MustCompile(`[A-Za-z0-9+/]{80,}={0,2}`)

// SanitizeForClient turns a candidate description into a safe string for API output.
// - If candidate is a string: trims, applies max length, and optionally redacts sensitive patterns.
// - If candidate is not a string (map, struct, etc.): returns SYSTEM_FAULT_DESC without dumping the value.
func SanitizeForClient(candidate any, maxLen int) string {
	if maxLen <= 0 {
		maxLen = MaxDescriptionLength
	}
	if candidate == nil {
		return SYSTEM_FAULT_DESC
	}
	s, ok := candidate.(string)
	if !ok {
		return SYSTEM_FAULT_DESC
	}
	s = strings.TrimSpace(s)
	// Redact long token-like substrings
	s = sensitiveTokenPattern.ReplaceAllString(s, "[REDACTED]")
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	// Truncate by runes to avoid cutting multi-byte characters
	runes := []rune(s)
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen])
}
