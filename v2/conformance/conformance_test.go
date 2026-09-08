package conformance_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/conformance"
	"github.com/hmmftg/requestCore/v2/httpsemantics"
	"github.com/hmmftg/requestCore/v2/idempotency"
	"github.com/hmmftg/requestCore/v2/response"
)

// TestBearerChallengeConformance verifies that v2 httpsemantics
// produces the expected Bearer challenge headers for all shared vectors.
func TestBearerChallengeConformance(t *testing.T) {
	for _, tc := range conformance.BearerChallengeVectors {
		t.Run(tc.Name, func(t *testing.T) {
			h := http.Header{}
			httpsemantics.ApplyBearerChallenge(h, httpsemantics.BearerChallenge{
				Realm:            tc.Realm,
				Error:            httpsemantics.BearerError(tc.Error),
				ErrorDescription: tc.Description,
				ErrorURI:         tc.URI,
				Scope:            tc.Scope,
			})
			got := h.Get("WWW-Authenticate")
			if got != tc.WantHeader {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tc.WantHeader)
			}
		})
	}
}

// TestRetryAfterConformance verifies that v2 httpsemantics parses
// Retry-After headers correctly for all shared vectors.
func TestRetryAfterConformance(t *testing.T) {
	for _, tc := range conformance.RetryAfterVectors {
		t.Run(tc.Name, func(t *testing.T) {
			ra, err := httpsemantics.ParseRetryAfter(tc.Header)
			if tc.WantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRetryAfter() error = %v", err)
			}
			if tc.WantIsDate {
				if !ra.IsDate {
					t.Error("expected IsDate = true")
				}
			} else {
				if ra.Delta != tc.WantDelta {
					t.Errorf("Delta = %d, want %d", ra.Delta, tc.WantDelta)
				}
			}
		})
	}
}

// TestETagConformance verifies that v2 httpsemantics parses ETags
// correctly for all shared vectors.
func TestETagConformance(t *testing.T) {
	for _, tc := range conformance.ETagVectors {
		t.Run(tc.Name, func(t *testing.T) {
			etag, err := httpsemantics.ParseETag(tc.Input)
			if tc.WantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseETag() error = %v", err)
			}
			if etag.Weak != tc.WantWeak {
				t.Errorf("Weak = %v, want %v", etag.Weak, tc.WantWeak)
			}
			if etag.Value != tc.WantValue {
				t.Errorf("Value = %q, want %q", etag.Value, tc.WantValue)
			}
		})
	}
}

// TestPreconditionConformance verifies that v2 httpsemantics evaluates
// preconditions correctly for all shared vectors.
func TestPreconditionConformance(t *testing.T) {
	for _, tc := range conformance.PreconditionVectors {
		t.Run(tc.Name, func(t *testing.T) {
			result := httpsemantics.EvaluatePreconditions(httpsemantics.PreconditionInput{
				IfMatch:            tc.IfMatch,
				IfNoneMatch:        tc.IfNoneMatch,
				IfModifiedSince:    tc.IfModifiedSince,
				IfUnmodifiedSince:  tc.IfUnmodifiedSince,
				ResourceETag:       tc.ResourceETag,
				ResourceModified:   tc.ResourceModified,
				IsSafeMethod:       tc.IsSafeMethod,
			})
			if int(result) != tc.WantResult {
				t.Errorf("result = %d, want %d", result, tc.WantResult)
			}
		})
	}
}

// TestIdempotencyKeyConformance verifies that v2 idempotency validates
// keys correctly for all shared vectors.
func TestIdempotencyKeyConformance(t *testing.T) {
	for _, tc := range conformance.IdempotencyKeyVectors {
		t.Run(tc.Name, func(t *testing.T) {
			err := idempotency.ValidateKey(tc.Key)
			if tc.WantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateKey() error = %v", err)
			}
		})
	}
}

// TestNoBodyStatusConformance verifies that v2 httpsemantics correctly
// identifies no-body statuses.
func TestNoBodyStatusConformance(t *testing.T) {
	for _, tc := range conformance.NoBodyStatusVectors {
		t.Run(tc.Name, func(t *testing.T) {
			got := httpsemantics.IsNoBodyStatus(tc.Status)
			if got != tc.WantNoBody {
				t.Errorf("IsNoBodyStatus(%d) = %v, want %v", tc.Status, got, tc.WantNoBody)
			}
		})
	}
}

// TestPaginationLinkConformance verifies that v2 httpsemantics builds
// pagination links correctly for all shared vectors.
func TestPaginationLinkConformance(t *testing.T) {
	for _, tc := range conformance.PaginationLinkVectors {
		t.Run(tc.Name, func(t *testing.T) {
			links := httpsemantics.BuildPaginationLinks(httpsemantics.PaginationConfig{
				BaseURL:    "/api/items",
				Page:       tc.Page,
				PageSize:   tc.PageSize,
				TotalItems: tc.TotalItems,
			})

			rels := make(map[string]bool)
			for _, l := range links {
				rels[l.Rel] = true
			}

			for _, want := range tc.WantRels {
				if !rels[want] {
					t.Errorf("missing rel %q", want)
				}
			}
			for _, notWant := range tc.WantNoRels {
				if rels[notWant] {
					t.Errorf("unexpected rel %q", notWant)
				}
			}
		})
	}
}

// TestSecurityConformance_ProblemSanitization verifies that v2 Problem
// responses do not leak sensitive data.
func TestSecurityConformance_ProblemSanitization(t *testing.T) {
	for _, tc := range conformance.SecurityVectors {
		t.Run(tc.Name, func(t *testing.T) {
			p := response.NewProblem(http.StatusInternalServerError, "Internal Server Error").
				WithCause(errors.New(tc.SensitiveData))

			body, err := json.Marshal(p)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}

			str := string(body)
			if strings.Contains(str, tc.SensitiveData) {
				t.Errorf("sensitive data leaked into JSON: %s", str)
			}
		})
	}
}

// TestSecurityConformance_SafeHeaders verifies that v2 idempotency
// SafeHeaders removes sensitive headers.
func TestSecurityConformance_SafeHeaders(t *testing.T) {
	sensitiveHeaders := []string{
		"Set-Cookie",
		"Authorization",
		"Cookie",
		"WWW-Authenticate",
		"Proxy-Authenticate",
		"Proxy-Authorization",
	}

	for _, hdr := range sensitiveHeaders {
		t.Run(hdr, func(t *testing.T) {
			h := http.Header{}
			h.Set(hdr, "sensitive-value")
			h.Set("Content-Type", "application/json")

			safe := idempotency.SafeHeaders(h)
			if safe.Get(hdr) != "" {
				t.Errorf("%s should be removed, got %q", hdr, safe.Get(hdr))
			}
			if safe.Get("Content-Type") != "application/json" {
				t.Error("Content-Type should be preserved")
			}
		})
	}
}

// TestTokenResponseNoStoreConformance verifies that the OAuth no-store
// headers are applied correctly.
func TestTokenResponseNoStoreConformance(t *testing.T) {
	h := http.Header{}
	httpsemantics.ApplyTokenResponseNoStore(h)

	if h.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", h.Get("Cache-Control"), "no-store")
	}
	if h.Get("Pragma") != "no-cache" {
		t.Errorf("Pragma = %q, want %q", h.Get("Pragma"), "no-cache")
	}
}

// TestFingerprintDeterminism verifies that request fingerprinting is
// deterministic across calls.
func TestFingerprintDeterminism(t *testing.T) {
	fp1 := idempotency.FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	fp2 := idempotency.FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	if fp1.Hash() != fp2.Hash() {
		t.Error("fingerprint should be deterministic")
	}
}

// TestFingerprintBodyNotRetained verifies that the raw body is not
// retained in the fingerprint.
func TestFingerprintBodyNotRetained(t *testing.T) {
	body := []byte(`{"secret":"password123"}`)
	fp := idempotency.FingerprintRequest("POST", "/orders", body)
	if strings.Contains(fp.BodyHash, "password123") {
		t.Error("fingerprint should not contain raw body")
	}
	if fp.BodyHash == string(body) {
		t.Error("BodyHash should be a hash, not the raw body")
	}
}

// TestMemoryStoreRaceConformance verifies that the MemoryStore handles
// concurrent reservations safely.
func TestMemoryStoreRaceConformance(t *testing.T) {
	store := idempotency.NewMemoryStore()
	fp := idempotency.FingerprintRequest("POST", "/orders", []byte(`{}`))

	var wg sync.WaitGroup
	var successes atomic.Int32
	var conflicts atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Reserve("race-key", fp, 1*time.Hour)
			if err == nil {
				successes.Add(1)
			} else if err == idempotency.ErrConflict {
				conflicts.Add(1)
			}
		}()
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("expected 1 success, got %d", successes.Load())
	}
	if conflicts.Load() != 19 {
		t.Errorf("expected 19 conflicts, got %d", conflicts.Load())
	}
}
