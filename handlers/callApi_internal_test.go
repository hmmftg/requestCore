package handlers

import "testing"

func TestFormatRetryKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		attempt int
		want    string
	}{
		// -failed suffix (new behavior)
		{name: "req-failed attempt 2", key: "keyhan-title-req-failed", attempt: 2, want: "keyhan-title-req-retry-1-failed"},
		{name: "req-failed attempt 3", key: "keyhan-title-req-failed", attempt: 3, want: "keyhan-title-req-retry-2-failed"},

		// -resp suffix (regression)
		{name: "resp attempt 2", key: "svc-call-resp", attempt: 2, want: "svc-call-retry-1-resp"},

		// -error suffix (regression)
		{name: "error attempt 2", key: "svc-call-error", attempt: 2, want: "svc-call-retry-1-error"},

		// no suffix (regression)
		{name: "base attempt 2", key: "svc-call", attempt: 2, want: "svc-call-retry-1"},

		// attempt 1 → unchanged (all variants)
		{name: "req-failed attempt 1", key: "keyhan-title-req-failed", attempt: 1, want: "keyhan-title-req-failed"},
		{name: "resp attempt 1", key: "svc-call-resp", attempt: 1, want: "svc-call-resp"},
		{name: "error attempt 1", key: "svc-call-error", attempt: 1, want: "svc-call-error"},
		{name: "base attempt 1", key: "svc-call", attempt: 1, want: "svc-call"},
		{name: "attempt 0", key: "svc-call", attempt: 0, want: "svc-call"},
		{name: "attempt negative", key: "svc-call", attempt: -1, want: "svc-call"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatRetryKey(tc.key, tc.attempt)
			if got != tc.want {
				t.Fatalf("formatRetryKey(%q, %d) = %q, want %q", tc.key, tc.attempt, got, tc.want)
			}
		})
	}
}
