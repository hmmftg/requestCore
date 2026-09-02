package operation

import (
	"testing"
)

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	op := Operation{ID: "getUser", Method: "GET", Pattern: "/users/{id}"}
	if err := r.Register(op); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := r.Unregister("getUser"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	_, ok := r.Get("getUser")
	if ok {
		t.Fatal("expected operation to be removed after Unregister")
	}
}

func TestRegistry_UnregisterNonExistent(t *testing.T) {
	r := NewRegistry()
	// Unregistering a non-existent ID should be a no-op.
	if err := r.Unregister("nonexistent"); err != nil {
		t.Fatalf("expected nil for non-existent ID, got %v", err)
	}
}

func TestRegistry_UnregisterAfterFreeze(t *testing.T) {
	r := NewRegistry()
	op := Operation{ID: "getUser", Method: "GET", Pattern: "/users/{id}"}
	_ = r.Register(op)
	r.Freeze()

	err := r.Unregister("getUser")
	if err != ErrRegistryFrozen {
		t.Fatalf("expected ErrRegistryFrozen, got %v", err)
	}
}

func TestRegistry_RollbackScenario(t *testing.T) {
	r := NewRegistry()
	op1 := Operation{ID: "createUser", Method: "POST", Pattern: "/users"}
	op2 := Operation{ID: "getUser", Method: "GET", Pattern: "/users/{id}"}

	_ = r.Register(op1)
	_ = r.Register(op2)

	// Simulate a router registration failure that requires rollback
	// of op2.
	if err := r.Unregister("getUser"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 operation after rollback, got %d", len(all))
	}
	if all[0].ID != "createUser" {
		t.Fatalf("expected createUser to remain, got %s", all[0].ID)
	}
}
