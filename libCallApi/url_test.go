package libCallApi_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/hmmftg/requestCore/libCallApi"
)

func TestExtractTrackerID(t *testing.T) {
	cases := []struct {
		name         string
		rawURL       string
		wantEndpoint string
		wantTracker  string
	}{
		{
			name:         "trackerId present",
			rawURL:       "https://api.example.com/v1/cards?trackerId=abc-123",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "abc-123",
		},
		{
			name:         "no query",
			rawURL:       "https://api.example.com/v1/cards",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "",
		},
		{
			name:         "query without trackerId",
			rawURL:       "https://api.example.com/v1/cards?page=1&size=10",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "",
		},
		{
			name:         "TrackerId variant",
			rawURL:       "https://api.example.com/v1/cards?TrackerId=abc-456",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "abc-456",
		},
		{
			name:         "TRACKERID variant",
			rawURL:       "https://api.example.com/v1/cards?TRACKERID=abc-789",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "abc-789",
		},
		{
			name:         "trackerId after other parameters",
			rawURL:       "https://api.example.com/v1/cards?page=2&size=20&trackerId=xyz",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "xyz",
		},
		{
			name:         "percent-encoded tracker ID",
			rawURL:       "https://api.example.com/v1/cards?trackerId=ab%2Fcd%3Def",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "ab/cd=ef",
		},
		{
			name:         "multiple trackerId values first wins",
			rawURL:       "https://api.example.com/v1/cards?trackerId=first&trackerId=second",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "first",
		},
		{
			name:         "empty URL",
			rawURL:       "",
			wantEndpoint: "",
			wantTracker:  "",
		},
		{
			name:         "empty trackerId value",
			rawURL:       "https://api.example.com/v1/cards?trackerId=",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "",
		},
		{
			name:         "trailing question mark only",
			rawURL:       "https://api.example.com/v1/cards?",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "",
		},
		{
			name:         "malformed query encoding returns clean endpoint",
			rawURL:       "https://api.example.com/v1/cards?trackerId=ab%2&bad=%ZZ",
			wantEndpoint: "https://api.example.com/v1/cards",
			wantTracker:  "",
		},
		{
			name:         "endpoint with path only no scheme",
			rawURL:       "/v1/cards?trackerId=local-1",
			wantEndpoint: "/v1/cards",
			wantTracker:  "local-1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, tracker := libCallApi.ExtractTrackerID(tc.rawURL)
			assert.Equal(t, endpoint, tc.wantEndpoint)
			assert.Equal(t, tracker, tc.wantTracker)
		})
	}
}
