package handlers_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/hmmftg/requestCore/handlers"
)

func TestShouldSkipAPICall(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		method   string
		patterns []string
		want     bool
	}{
		{
			name:     "empty patterns",
			endpoint: "api/card",
			method:   "POST",
			patterns: nil,
			want:     false,
		},
		{
			name:     "exact match",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"api/card"},
			want:     true,
		},
		{
			name:     "path-boundary prefix match",
			endpoint: "api/card/123",
			method:   "GET",
			patterns: []string{"api/card"},
			want:     true,
		},
		{
			name:     "non-boundary near match must not match",
			endpoint: "api/cardholder",
			method:   "GET",
			patterns: []string{"api/card"},
			want:     false,
		},
		{
			name:     "method-qualified match",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"api/card:POST"},
			want:     true,
		},
		{
			name:     "method-qualified mismatch",
			endpoint: "api/card",
			method:   "GET",
			patterns: []string{"api/card:POST"},
			want:     false,
		},
		{
			name:     "method case differences match",
			endpoint: "api/card",
			method:   "post",
			patterns: []string{"api/card:POST"},
			want:     true,
		},
		{
			name:     "unqualified pattern matches any method",
			endpoint: "api/card",
			method:   "DELETE",
			patterns: []string{"api/card"},
			want:     true,
		},
		{
			name:     "whitespace in pattern is trimmed",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"  api/card  "},
			want:     true,
		},
		{
			name:     "empty pattern is ignored",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"", "   "},
			want:     false,
		},
		{
			name:     "malformed multi-colon pattern is ignored",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"api/card:POST:extra"},
			want:     false,
		},
		{
			name:     "leading trailing slash normalization endpoint",
			endpoint: "/api/card/",
			method:   "POST",
			patterns: []string{"api/card"},
			want:     true,
		},
		{
			name:     "leading trailing slash normalization pattern",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"/api/card/"},
			want:     true,
		},
		{
			name:     "prefix match with normalized slashes",
			endpoint: "/api/card/123/",
			method:   "GET",
			patterns: []string{"/api/card/"},
			want:     true,
		},
		{
			name:     "first matching pattern wins among several",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{"api/other", "api/card"},
			want:     true,
		},
		{
			name:     "method-qualified prefix match",
			endpoint: "api/card/123",
			method:   "PUT",
			patterns: []string{"api/card:PUT"},
			want:     true,
		},
		{
			name:     "method-qualified prefix mismatch",
			endpoint: "api/card/123",
			method:   "GET",
			patterns: []string{"api/card:PUT"},
			want:     false,
		},
		{
			name:     "empty endpoint never matches non-empty pattern",
			endpoint: "",
			method:   "POST",
			patterns: []string{"api/card"},
			want:     false,
		},
		{
			name:     "pattern with empty endpoint after colon separator ignored",
			endpoint: "api/card",
			method:   "POST",
			patterns: []string{":POST"},
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := handlers.ShouldSkipAPICall(tc.endpoint, tc.method, tc.patterns)
			assert.Equal(t, got, tc.want)
		})
	}
}
