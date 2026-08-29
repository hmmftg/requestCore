package operation

import (
	"errors"
	"sync"
	"testing"
)

func TestNewOperation(t *testing.T) {
	op := NewOperation("createUser", "POST", "/users")
	if op.ID != "createUser" {
		t.Fatalf("expected ID createUser, got %q", op.ID)
	}
	if op.Method != "POST" {
		t.Fatalf("expected POST, got %q", op.Method)
	}
	if op.Pattern != "/users" {
		t.Fatalf("expected /users, got %q", op.Pattern)
	}
}

func TestOperation_BuilderMethods(t *testing.T) {
	op := NewOperation("getUser", "GET", "/users/{id}").
		WithName("Get User").
		WithSummary("Retrieve a user by ID").
		WithDescription("Returns a single user resource").
		WithTags("users", "read").
		WithDeprecated()

	if op.Name != "Get User" {
		t.Fatalf("expected Name 'Get User', got %q", op.Name)
	}
	if op.Summary != "Retrieve a user by ID" {
		t.Fatalf("expected Summary, got %q", op.Summary)
	}
	if op.Description != "Returns a single user resource" {
		t.Fatalf("expected Description, got %q", op.Description)
	}
	if len(op.Tags) != 2 || op.Tags[0] != "users" || op.Tags[1] != "read" {
		t.Fatalf("expected [users, read], got %v", op.Tags)
	}
	if !op.Deprecated {
		t.Fatal("expected Deprecated=true")
	}
}

func TestOperation_String(t *testing.T) {
	op := NewOperation("listUsers", "GET", "/users")
	if op.String() != "GET /users [listUsers]" {
		t.Fatalf("expected 'GET /users [listUsers]', got %q", op.String())
	}
}

func TestOperation_Clone(t *testing.T) {
	op := NewOperation("updateUser", "PUT", "/users/{id}").
		WithTags("users", "write")
	clone := op.Clone()
	if clone.ID != op.ID || clone.Method != op.Method || clone.Pattern != op.Pattern {
		t.Fatal("clone should have same core fields")
	}
	if len(clone.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(clone.Tags))
	}
	// Mutating clone tags should not affect original.
	clone.Tags[0] = "mutated"
	if op.Tags[0] != "users" {
		t.Fatalf("expected original unchanged, got %q", op.Tags[0])
	}
}

func TestOperation_Clone_NilTags(t *testing.T) {
	op := NewOperation("deleteUser", "DELETE", "/users/{id}")
	clone := op.Clone()
	if clone.Tags != nil {
		t.Fatalf("expected nil tags, got %v", clone.Tags)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	op := NewOperation("createUser", "POST", "/users")
	if err := r.Register(op); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := r.Get("createUser")
	if !ok {
		t.Fatal("expected to find operation")
	}
	if got.ID != "createUser" {
		t.Fatalf("expected createUser, got %q", got.ID)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected ok=false for missing operation")
	}
}

func TestRegistry_DuplicateID(t *testing.T) {
	r := NewRegistry()
	op := NewOperation("createUser", "POST", "/users")
	if err := r.Register(op); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register(op)
	if !errors.Is(err, ErrDuplicateOperationID) {
		t.Fatalf("expected ErrDuplicateOperationID, got %v", err)
	}
}

func TestRegistry_InvalidOperation(t *testing.T) {
	r := NewRegistry()
	tests := []struct {
		name string
		op   Operation
	}{
		{"empty ID", NewOperation("", "POST", "/users")},
		{"empty method", NewOperation("createUser", "", "/users")},
		{"empty pattern", NewOperation("createUser", "POST", "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Register(tt.op)
			if !errors.Is(err, ErrInvalidOperation) {
				t.Fatalf("expected ErrInvalidOperation, got %v", err)
			}
		})
	}
}

func TestRegistry_Freeze(t *testing.T) {
	r := NewRegistry()
	r.Freeze()
	if !r.Frozen() {
		t.Fatal("expected frozen=true")
	}
	err := r.Register(NewOperation("createUser", "POST", "/users"))
	if !errors.Is(err, ErrRegistryFrozen) {
		t.Fatalf("expected ErrRegistryFrozen, got %v", err)
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(NewOperation("createUser", "POST", "/users"))
	_ = r.Register(NewOperation("listUsers", "GET", "/users"))
	_ = r.Register(NewOperation("getUser", "GET", "/users/{id}"))

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(all))
	}
	// Should be sorted by ID.
	if all[0].ID != "createUser" {
		t.Fatalf("expected first createUser, got %q", all[0].ID)
	}
	if all[1].ID != "getUser" {
		t.Fatalf("expected second getUser, got %q", all[1].ID)
	}
	if all[2].ID != "listUsers" {
		t.Fatalf("expected third listUsers, got %q", all[2].ID)
	}
}

func TestRegistry_AllEmpty(t *testing.T) {
	r := NewRegistry()
	all := r.All()
	if all != nil && len(all) != 0 {
		t.Fatalf("expected empty slice, got %v", all)
	}
}

func TestRegistry_AllReturnsCopies(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(NewOperation("createUser", "POST", "/users").
		WithTags("users"))
	all := r.All()
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
	// Mutate the returned copy.
	all[0].ID = "mutated"
	all[0].Tags[0] = "mutated"
	// Original should be unaffected.
	got, ok := r.Get("createUser")
	if !ok {
		t.Fatal("expected to find original")
	}
	if got.ID != "createUser" {
		t.Fatalf("expected original ID unchanged, got %q", got.ID)
	}
	if got.Tags[0] != "users" {
		t.Fatalf("expected original tags unchanged, got %q", got.Tags[0])
	}
}

func TestRegistry_GetReturnsCopy(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(NewOperation("createUser", "POST", "/users").
		WithTags("users"))
	got, _ := r.Get("createUser")
	got.Tags[0] = "mutated"
	// Second get should return original.
	got2, _ := r.Get("createUser")
	if got2.Tags[0] != "users" {
		t.Fatalf("expected original tags, got %q", got2.Tags[0])
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	// Concurrent writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = r.Register(NewOperation(
				"op"+string(rune('a'+n)),
				"GET",
				"/path",
			))
		}(i)
	}
	// Concurrent readers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.All()
		}()
	}
	wg.Wait()
}
