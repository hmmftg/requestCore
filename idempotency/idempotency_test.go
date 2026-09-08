package idempotency

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateKey_Valid(t *testing.T) {
	tests := []string{
		"abc123",
		"order-12345",
		"550e8400-e29b-41d4-a716-446655440000",
		"a",
	}
	for _, key := range tests {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) error = %v", key, err)
		}
	}
}

func TestValidateKey_Empty(t *testing.T) {
	if err := ValidateKey(""); err != ErrKeyEmpty {
		t.Errorf("ValidateKey(\"\") error = %v, want %v", err, ErrKeyEmpty)
	}
}

func TestValidateKey_TooLong(t *testing.T) {
	key := make([]byte, KeyMaxLength+1)
	for i := range key {
		key[i] = 'a'
	}
	if err := ValidateKey(string(key)); err != ErrKeyTooLong {
		t.Errorf("ValidateKey(too long) error = %v, want %v", err, ErrKeyTooLong)
	}
}

func TestValidateKey_NonPrintable(t *testing.T) {
	if err := ValidateKey("abc\x00def"); err == nil {
		t.Error("ValidateKey with non-printable should error")
	}
}

func TestValidateKey_WhitespaceOnly(t *testing.T) {
	if err := ValidateKey("   "); err == nil {
		t.Error("ValidateKey with whitespace-only should error")
	}
}

func TestFingerprintRequest_Deterministic(t *testing.T) {
	f1 := FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	f2 := FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	if f1.Hash() != f2.Hash() {
		t.Error("same request should produce same fingerprint")
	}
}

func TestFingerprintRequest_DifferentBody(t *testing.T) {
	f1 := FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	f2 := FingerprintRequest("POST", "/orders", []byte(`{"item":"pen"}`))
	if f1.Hash() == f2.Hash() {
		t.Error("different bodies should produce different fingerprints")
	}
}

func TestFingerprintRequest_DifferentMethod(t *testing.T) {
	f1 := FingerprintRequest("POST", "/orders", []byte(`{}`))
	f2 := FingerprintRequest("PUT", "/orders", []byte(`{}`))
	if f1.Hash() == f2.Hash() {
		t.Error("different methods should produce different fingerprints")
	}
}

func TestFingerprintRequest_DifferentPath(t *testing.T) {
	f1 := FingerprintRequest("POST", "/orders", []byte(`{}`))
	f2 := FingerprintRequest("POST", "/users", []byte(`{}`))
	if f1.Hash() == f2.Hash() {
		t.Error("different paths should produce different fingerprints")
	}
}

func TestFingerprintRequest_MethodCaseInsensitive(t *testing.T) {
	f1 := FingerprintRequest("post", "/orders", []byte(`{}`))
	f2 := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if f1.Hash() != f2.Hash() {
		t.Error("method should be case-insensitive")
	}
}

func TestFingerprintRequest_BodyHashOnly(t *testing.T) {
	f := FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	if f.BodyHash == "" {
		t.Error("BodyHash should not be empty")
	}
	if f.BodyHash == `{"item":"book"}` {
		t.Error("BodyHash should be a hash, not the raw body")
	}
}

func TestSafeHeaders_RemovesSensitive(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Set-Cookie", "session=abc123")
	h.Set("Authorization", "Bearer token123")
	h.Set("X-Custom", "custom-value")

	safe := SafeHeaders(h)

	if safe.Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be preserved")
	}
	if safe.Get("Set-Cookie") != "" {
		t.Error("Set-Cookie should be removed")
	}
	if safe.Get("Authorization") != "" {
		t.Error("Authorization should be removed")
	}
	if safe.Get("X-Custom") != "custom-value" {
		t.Error("X-Custom should be preserved")
	}
}

func TestIsSafeMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"HEAD", true},
		{"PUT", true},
		{"DELETE", true},
		{"POST", false},
		{"PATCH", false},
	}
	for _, tt := range tests {
		if got := IsSafeMethod(tt.method); got != tt.want {
			t.Errorf("IsSafeMethod(%q) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestRequiresIdempotencyKey(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"POST", true},
		{"PATCH", true},
		{"GET", false},
		{"PUT", false},
		{"DELETE", false},
	}
	for _, tt := range tests {
		if got := RequiresIdempotencyKey(tt.method); got != tt.want {
			t.Errorf("RequiresIdempotencyKey(%q) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestRecord_IsExpired(t *testing.T) {
	now := time.Now()
	r := &Record{
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if r.IsExpired(now) {
		t.Error("record should not be expired")
	}
	if !r.IsExpired(now.Add(2 * time.Hour)) {
		t.Error("record should be expired")
	}
}

func TestMemoryStore_Reserve_New(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	existing, err := s.Reserve("key1", fp, 1*time.Hour)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if existing != nil {
		t.Error("existing should be nil for new key")
	}
}

func TestMemoryStore_Reserve_Conflict(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	existing, err := s.Reserve("key1", fp, 1*time.Hour)
	if err != ErrConflict {
		t.Errorf("second Reserve() error = %v, want %v", err, ErrConflict)
	}
	if existing == nil {
		t.Error("existing should not be nil on conflict")
	}
}

func TestMemoryStore_Reserve_ReplayAvailable(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := s.Complete("key1", 201, http.Header{"Content-Type": []string{"application/json"}}, []byte(`{"id":1}`)); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	existing, err := s.Reserve("key1", fp, 1*time.Hour)
	if err != ErrReplayAvailable {
		t.Errorf("Reserve() error = %v, want %v", err, ErrReplayAvailable)
	}
	if existing == nil {
		t.Error("existing should not be nil on replay")
	}
	if existing.ResponseStatus != 201 {
		t.Errorf("ResponseStatus = %d, want 201", existing.ResponseStatus)
	}
}

func TestMemoryStore_Reserve_FingerprintMismatch(t *testing.T) {
	s := NewMemoryStore()
	fp1 := FingerprintRequest("POST", "/orders", []byte(`{"item":"book"}`))
	if _, err := s.Reserve("key1", fp1, 1*time.Hour); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	fp2 := FingerprintRequest("POST", "/orders", []byte(`{"item":"pen"}`))
	_, err := s.Reserve("key1", fp2, 1*time.Hour)
	if err != ErrFingerprintMismatch {
		t.Errorf("Reserve() error = %v, want %v", err, ErrFingerprintMismatch)
	}
}

func TestMemoryStore_Complete(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	h := http.Header{"Content-Type": []string{"application/json"}}
	if err := s.Complete("key1", 201, h, []byte(`{"id":1}`)); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	record, err := s.Get("key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record.State != StateCompleted {
		t.Errorf("State = %v, want StateCompleted", record.State)
	}
	if record.ResponseStatus != 201 {
		t.Errorf("ResponseStatus = %d, want 201", record.ResponseStatus)
	}
	if string(record.ResponseBody) != `{"id":1}` {
		t.Errorf("ResponseBody = %q, want %q", string(record.ResponseBody), `{"id":1}`)
	}
}

func TestMemoryStore_Fail(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := s.Fail("key1"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	record, err := s.Get("key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record.State != StateFailed {
		t.Errorf("State = %v, want StateFailed", record.State)
	}
}

func TestMemoryStore_Reserve_AfterFail(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	if err := s.Fail("key1"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	// Should be able to reserve again after failure
	existing, err := s.Reserve("key1", fp, 1*time.Hour)
	if err != nil {
		t.Errorf("Reserve after Fail() error = %v, want nil", err)
	}
	if existing != nil {
		t.Error("existing should be nil for re-reservation after failure")
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	s := NewMemoryStore()
	record, err := s.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record != nil {
		t.Error("record should be nil for nonexistent key")
	}
}

func TestMemoryStore_Complete_NotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.Complete("nonexistent", 200, nil, nil)
	if err != ErrNotFound {
		t.Errorf("Complete() error = %v, want %v", err, ErrNotFound)
	}
}

func TestMemoryStore_Complete_NotInProgress(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Hour); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := s.Complete("key1", 201, nil, nil); err != nil {
		t.Fatalf("first Complete() error = %v", err)
	}
	// Second complete should fail
	err := s.Complete("key1", 200, nil, nil)
	if err != ErrNotInProgress {
		t.Errorf("second Complete() error = %v, want %v", err, ErrNotInProgress)
	}
}

func TestMemoryStore_ExpiredRecord(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	// Reserve with very short TTL
	if _, err := s.Reserve("key1", fp, 1*time.Millisecond); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	// Wait for expiration
	time.Sleep(10 * time.Millisecond)
	// Should be able to reserve again
	existing, err := s.Reserve("key1", fp, 1*time.Hour)
	if err != nil {
		t.Errorf("Reserve() after expiry error = %v, want nil", err)
	}
	if existing != nil {
		t.Error("existing should be nil for re-reservation after expiry")
	}
}

func TestMemoryStore_ConcurrentReserve(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))

	var wg sync.WaitGroup
	var conflicts atomic.Int32
	var successes atomic.Int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Reserve("concurrent-key", fp, 1*time.Hour)
			if err == nil {
				successes.Add(1)
			} else if err == ErrConflict {
				conflicts.Add(1)
			}
		}()
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("expected 1 success, got %d", successes.Load())
	}
	if conflicts.Load() != 9 {
		t.Errorf("expected 9 conflicts, got %d", conflicts.Load())
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	s := NewMemoryStore()
	fp := FingerprintRequest("POST", "/orders", []byte(`{}`))
	if _, err := s.Reserve("key1", fp, 1*time.Millisecond); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if _, err := s.Reserve("key2", fp, 1*time.Hour); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	s.Cleanup()
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1 after cleanup", s.Len())
	}
}
