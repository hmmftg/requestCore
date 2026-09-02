package routing

import (
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
)

func TestValidatePattern_Valid(t *testing.T) {
	patterns := []string{
		"/users",
		"/users/{id}",
		"/users/{id}/posts",
		"/users/{id}/posts/{postId}",
		"/",
		"/users/new",
	}
	for _, p := range patterns {
		if err := ValidatePattern(p); err != nil {
			t.Fatalf("expected valid pattern %q, got error: %v", p, err)
		}
	}
}

func TestValidatePattern_Invalid(t *testing.T) {
	patterns := []string{
		"/users/{id",
		"/users/id}",
		"/users/{{id}}",
		"/users/}",
	}
	for _, p := range patterns {
		if err := ValidatePattern(p); err == nil {
			t.Fatalf("expected error for invalid pattern %q", p)
		}
	}
}

func TestTranslatePattern_Gin(t *testing.T) {
	result := TranslatePattern("/users/{id}/posts/{postId}", "gin")
	if result != "/users/:id/posts/:postId" {
		t.Fatalf("expected /users/:id/posts/:postId, got %s", result)
	}
}

func TestTranslatePattern_Chi(t *testing.T) {
	result := TranslatePattern("/users/{id}", "chi")
	if result != "/users/{id}" {
		t.Fatalf("expected /users/{id}, got %s", result)
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		base, rel, expected string
	}{
		{"", "/users", "/users"},
		{"/api", "", "/api"},
		{"/api", "/users", "/api/users"},
		{"/api/", "/users", "/api/users"},
		{"/api", "users", "/api/users"},
		{"/api/", "users", "/api/users"},
	}
	for _, tt := range tests {
		result := JoinPath(tt.base, tt.rel)
		if result != tt.expected {
			t.Fatalf("JoinPath(%q, %q) = %q, expected %q", tt.base, tt.rel, result, tt.expected)
		}
	}
}

func TestChain_Empty(t *testing.T) {
	result := Chain()
	if result != nil {
		t.Fatal("expected nil for empty chain")
	}
}

func TestChain_Single(t *testing.T) {
	mw := func(next Handler) Handler {
		return func(ctx *request.Context, transport Transport) error {
			return next(ctx, transport)
		}
	}
	result := Chain(mw)
	if result == nil {
		t.Fatal("expected non-nil for single middleware")
	}
}

func TestChain_Multiple(t *testing.T) {
	order := []string{}
	mw1 := func(next Handler) Handler {
		return func(ctx *request.Context, transport Transport) error {
			order = append(order, "mw1-before")
			err := next(ctx, transport)
			order = append(order, "mw1-after")
			return err
		}
	}
	mw2 := func(next Handler) Handler {
		return func(ctx *request.Context, transport Transport) error {
			order = append(order, "mw2-before")
			err := next(ctx, transport)
			order = append(order, "mw2-after")
			return err
		}
	}
	chain := Chain(mw1, mw2)
	if chain == nil {
		t.Fatal("expected non-nil chain")
	}

	handler := chain(func(ctx *request.Context, transport Transport) error {
		order = append(order, "handler")
		return nil
	})

	if err := handler(nil, nil); err != nil {
		t.Fatalf("handler: %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}
