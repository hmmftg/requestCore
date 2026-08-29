package response

import (
	"errors"
	"net/http"
	"testing"
)

// customErrA and customErrB are test error types for mapper tests.
type customErrA struct{ msg string }

func (e *customErrA) Error() string { return e.msg }

type customErrB struct{ msg string }

func (e *customErrB) Error() string { return e.msg }

// matchA is a matcher that matches *customErrA via errors.As.
func matchA(err error) bool {
	var e *customErrA
	return errors.As(err, &e)
}

// matchB is a matcher that matches *customErrB via errors.As.
func matchB(err error) bool {
	var e *customErrB
	return errors.As(err, &e)
}

func TestMapperRegistry_DefaultSanitizer(t *testing.T) {
	r := DefaultMapperRegistry()
	p := r.Map(errors.New("some unknown error"))
	if p == nil {
		t.Fatal("expected non-nil problem")
	}
	if p.Status != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", p.Status)
	}
	if p.Code != "INTERNAL" {
		t.Fatalf("expected code INTERNAL, got %q", p.Code)
	}
	// Detail should not expose the raw error.
	if p.Detail != "" {
		t.Fatalf("expected empty detail, got %q", p.Detail)
	}
}

func TestMapperRegistry_NilError(t *testing.T) {
	r := DefaultMapperRegistry()
	if p := r.Map(nil); p != nil {
		t.Fatalf("expected nil for nil error, got %v", p)
	}
}

func TestMapperRegistry_AlreadyProblem(t *testing.T) {
	r := DefaultMapperRegistry()
	original := NewProblem(422, "Validation Failed").WithCode("VALIDATION")
	p := r.Map(original)
	if p != original {
		t.Fatal("expected original problem returned as-is")
	}
}

func TestMapperRegistry_RegisteredMapper(t *testing.T) {
	r := DefaultMapperRegistry()
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusConflict, "Conflict", "CUSTOM_A")
	})

	p := r.Map(&customErrA{msg: "conflict occurred"})
	if p == nil {
		t.Fatal("expected non-nil problem")
	}
	if p.Status != http.StatusConflict {
		t.Fatalf("expected 409, got %d", p.Status)
	}
	if p.Code != "CUSTOM_A" {
		t.Fatalf("expected CUSTOM_A, got %q", p.Code)
	}
}

func TestMapperRegistry_FirstMatchingMapperWins(t *testing.T) {
	r := DefaultMapperRegistry()
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusConflict, "First", "FIRST")
	})
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusBadRequest, "Second", "SECOND")
	})

	p := r.Map(&customErrA{msg: "test"})
	if p.Code != "FIRST" {
		t.Fatalf("expected FIRST mapper to win, got %q", p.Code)
	}
}

func TestMapperRegistry_NoMatchUsesFallback(t *testing.T) {
	r := DefaultMapperRegistry()
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusConflict, "Conflict", "CUSTOM_A")
	})

	// customErrB is not registered; should fall back to 500.
	p := r.Map(&customErrB{msg: "unknown"})
	if p.Status != http.StatusInternalServerError {
		t.Fatalf("expected 500 fallback, got %d", p.Status)
	}
}

func TestMapperRegistry_CustomFallback(t *testing.T) {
	r := DefaultMapperRegistry()
	r.SetFallback(func(err error) *Problem {
		return NewProblemWithCode(http.StatusBadGateway, "Bad Gateway", "CUSTOM_FALLBACK")
	})

	p := r.Map(errors.New("unknown"))
	if p.Status != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", p.Status)
	}
	if p.Code != "CUSTOM_FALLBACK" {
		t.Fatalf("expected CUSTOM_FALLBACK, got %q", p.Code)
	}
}

func TestMapperRegistry_NilFallbackRestoresDefault(t *testing.T) {
	r := DefaultMapperRegistry()
	r.SetFallback(func(err error) *Problem {
		return NewProblemWithCode(http.StatusBadGateway, "Bad Gateway", "CUSTOM")
	})
	r.SetFallback(nil)

	p := r.Map(errors.New("unknown"))
	if p.Status != http.StatusInternalServerError {
		t.Fatalf("expected 500 after nil fallback, got %d", p.Status)
	}
}

func TestMapperRegistry_RegisterNilMatcher(t *testing.T) {
	r := DefaultMapperRegistry()
	err := r.Register(nil, func(err error) *Problem { return nil })
	if err == nil {
		t.Fatal("expected error for nil matcher")
	}
}

func TestMapperRegistry_RegisterNilMapper(t *testing.T) {
	r := DefaultMapperRegistry()
	err := r.Register(matchA, nil)
	if err == nil {
		t.Fatal("expected error for nil mapper")
	}
}

func TestMapperRegistry_WrappedError(t *testing.T) {
	r := DefaultMapperRegistry()
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusConflict, "Conflict", "WRAPPED_A")
	})

	// Wrap customErrA in another error.
	wrapped := errors.Join(&customErrA{msg: "inner"})
	p := r.Map(wrapped)
	if p == nil {
		t.Fatal("expected non-nil problem")
	}
	if p.Code != "WRAPPED_A" {
		t.Fatalf("expected WRAPPED_A via errors.As, got %q", p.Code)
	}
}

func TestMapperRegistry_MapperReturnsNilFallsThrough(t *testing.T) {
	r := DefaultMapperRegistry()
	_ = r.Register(matchA, func(err error) *Problem {
		return nil // this mapper declines
	})
	_ = r.Register(matchA, func(err error) *Problem {
		return NewProblemWithCode(http.StatusConflict, "Second", "SECOND")
	})

	p := r.Map(&customErrA{msg: "test"})
	if p == nil {
		t.Fatal("expected non-nil problem")
	}
	if p.Code != "SECOND" {
		t.Fatalf("expected SECOND (first mapper declined), got %q", p.Code)
	}
}
