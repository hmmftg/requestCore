package request

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestBodySource_LazyRead(t *testing.T) {
	data := `{"name":"alice"}`
	bs := NewBodySource(strings.NewReader(data))
	got, err := bs.Read(0)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(got) != data {
		t.Fatalf("expected %q, got %q", data, string(got))
	}
}

func TestBodySource_CachedOnSecondRead(t *testing.T) {
	// Use a fail-on-second-read reader to verify caching.
	bs := NewBodySource(&onceReader{data: []byte("hello")})
	first, err := bs.Read(0)
	if err != nil {
		t.Fatalf("first Read failed: %v", err)
	}
	if string(first) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(first))
	}
	// Second read should return cached result without calling reader.
	second, err := bs.Read(0)
	if err != nil {
		t.Fatalf("second Read failed: %v", err)
	}
	if string(second) != "hello" {
		t.Fatalf("expected cached 'hello', got %q", string(second))
	}
}

func TestBodySource_BoundedRead(t *testing.T) {
	bs := NewBodySource(strings.NewReader("this is a long body"))
	_, err := bs.Read(10)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestBodySource_BoundedReadExactLimit(t *testing.T) {
	bs := NewBodySource(strings.NewReader("exactly10"))
	got, err := bs.Read(10)
	if err != nil {
		t.Fatalf("expected no error at exact limit, got %v", err)
	}
	if string(got) != "exactly10" {
		t.Fatalf("expected 'exactly10', got %q", string(got))
	}
}

func TestBodySource_NilReader(t *testing.T) {
	bs := NewBodySource(nil)
	got, err := bs.Read(0)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestBodySource_EmptyReader(t *testing.T) {
	bs := NewBodySource(strings.NewReader(""))
	got, err := bs.Read(0)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestStringBodySource(t *testing.T) {
	bs := NewStringBodySource("test body")
	got, err := bs.Read(0)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(got) != "test body" {
		t.Fatalf("expected 'test body', got %q", string(got))
	}
}

func TestStringBodySource_Bounded(t *testing.T) {
	bs := NewStringBodySource("too long body")
	_, err := bs.Read(5)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestContext_BodyBytesFromString(t *testing.T) {
	ctx := NewContext(nil, WithBody("hello"))
	got, err := ctx.BodyBytes(0)
	if err != nil {
		t.Fatalf("BodyBytes failed: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(got))
	}
}

func TestContext_BodyBytesFromSource(t *testing.T) {
	ctx := NewContext(nil, WithBodySource(NewBodySource(strings.NewReader("from source"))))
	got, err := ctx.BodyBytes(0)
	if err != nil {
		t.Fatalf("BodyBytes failed: %v", err)
	}
	if string(got) != "from source" {
		t.Fatalf("expected 'from source', got %q", string(got))
	}
}

func TestContext_BodyBytesBounded(t *testing.T) {
	ctx := NewContext(nil, WithBody("this is too long"))
	_, err := ctx.BodyBytes(5)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestContext_BodyFromSource(t *testing.T) {
	ctx := NewContext(nil, WithBodySource(NewStringBodySource("source body")))
	if ctx.Body() != "source body" {
		t.Fatalf("expected 'source body', got %q", ctx.Body())
	}
}

func TestContext_BodySourcePrecedence(t *testing.T) {
	// When both body string and bodySource are set, source takes precedence.
	ctx := NewContext(nil,
		WithBody("string body"),
		WithBodySource(NewStringBodySource("source body")),
	)
	if ctx.Body() != "source body" {
		t.Fatalf("expected source to take precedence, got %q", ctx.Body())
	}
}

// TestContext_WithContextSharesState verifies that WithContext-derived
// contexts share typed values and hooks without data races.
func TestContext_WithContextSharesState(t *testing.T) {
	ctx := NewContext(nil)
	derived := ctx.WithContext(nil)

	ctx.setTyped(1, "value-from-original")
	v, ok := derived.getTyped(1)
	if !ok || v != "value-from-original" {
		t.Fatalf("derived context should see value from original, got %v (ok=%v)", v, ok)
	}

	derived.setTyped(2, "value-from-derived")
	v, ok = ctx.getTyped(2)
	if !ok || v != "value-from-derived" {
		t.Fatalf("original context should see value from derived, got %v (ok=%v)", v, ok)
	}
}

// TestContext_WithContextSharesHooks verifies that hooks registered on
// the original context are visible to derived contexts and run once.
func TestContext_WithContextSharesHooks(t *testing.T) {
	ctx := NewContext(nil)
	derived := ctx.WithContext(nil)

	hookRan := false
	ctx.AddBeforeCommitHook(func() error {
		hookRan = true
		return nil
	})

	// Running hooks on the derived context should run the hook
	// registered on the original.
	if err := derived.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("RunBeforeCommitHooks failed: %v", err)
	}
	if !hookRan {
		t.Fatal("hook registered on original did not run via derived")
	}

	// Running again on the original should be a no-op (idempotent).
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("second RunBeforeCommitHooks failed: %v", err)
	}
}

// TestContext_WithContextPreservesBody verifies that body and bodySource
// are preserved across WithContext.
func TestContext_WithContextPreservesBody(t *testing.T) {
	ctx := NewContext(nil, WithBody("preserved body"))
	derived := ctx.WithContext(nil)
	if derived.Body() != "preserved body" {
		t.Fatalf("expected body preserved, got %q", derived.Body())
	}

	ctx2 := NewContext(nil, WithBodySource(NewStringBodySource("source preserved")))
	derived2 := ctx2.WithContext(nil)
	if derived2.Body() != "source preserved" {
		t.Fatalf("expected bodySource preserved, got %q", derived2.Body())
	}
}

// TestContext_ConcurrentSharedState verifies no data race when
// concurrent goroutines access shared state via derived contexts.
func TestContext_ConcurrentSharedState(t *testing.T) {
	ctx := NewContext(nil)
	derived := ctx.WithContext(nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			ctx.setTyped(uint64(n), n)
		}(i)
		go func(n int) {
			defer wg.Done()
			derived.setTyped(uint64(n+1000), n)
		}(i)
	}
	wg.Wait()

	// Verify cross-visibility.
	for i := 0; i < 100; i++ {
		v, ok := derived.getTyped(uint64(i))
		if !ok || v != i {
			t.Fatalf("derived should see value %d from original, got %v (ok=%v)", i, v, ok)
		}
	}
}

// onceReader returns all data on the first Read call and EOF on
// subsequent calls. It panics if Read is called after the first call
// (to verify caching).
type onceReader struct {
	data []byte
	read bool
}

func (r *onceReader) Read(p []byte) (int, error) {
	if r.read {
		panic("onceReader: Read called after first call — caching failed")
	}
	r.read = true
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

// TestBodySource_ConcurrentRead verifies no data race on concurrent
// Read calls (only one goroutine should actually read; others get
// cached result).
func TestBodySource_ConcurrentRead(t *testing.T) {
	bs := NewBodySource(&onceReader{data: []byte("concurrent body")})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := bs.Read(0)
			if err != nil {
				t.Errorf("Read failed: %v", err)
			}
			if string(got) != "concurrent body" {
				t.Errorf("expected 'concurrent body', got %q", string(got))
			}
		}()
	}
	wg.Wait()
}
