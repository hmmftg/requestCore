package response

import (
	"errors"
	"testing"
)

func TestMapperRegistry_Freeze(t *testing.T) {
	r := NewMapperRegistry()
	r.Freeze()

	err := r.Register(func(err error) bool { return true }, func(err error) *Problem {
		return NewProblemWithCode(400, "test", "TEST")
	})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("expected ErrRegistryFrozen, got %v", err)
	}
}

func TestMapperRegistry_SetFallbackAfterFreeze(t *testing.T) {
	r := NewMapperRegistry()
	r.Freeze()

	err := r.SetFallback(func(err error) *Problem {
		return NewProblemWithCode(500, "custom", "CUSTOM")
	})
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("expected ErrRegistryFrozen, got %v", err)
	}
}

func TestMapperRegistry_FrozenReported(t *testing.T) {
	r := NewMapperRegistry()
	if r.Frozen() {
		t.Fatal("expected not frozen initially")
	}
	r.Freeze()
	if !r.Frozen() {
		t.Fatal("expected frozen after Freeze()")
	}
}

func TestMapperRegistry_RegisterBeforeFreeze(t *testing.T) {
	r := NewMapperRegistry()
	err := r.Register(func(err error) bool { return true }, func(err error) *Problem {
		return NewProblemWithCode(400, "test", "TEST")
	})
	if err != nil {
		t.Fatalf("Register before freeze failed: %v", err)
	}
	r.Freeze()
	// Map should still work after freeze.
	p := r.Map(errors.New("some error"))
	if p == nil {
		t.Fatal("expected non-nil problem")
	}
	if p.Status != 400 {
		t.Fatalf("expected 400 from registered mapper, got %d", p.Status)
	}
}
