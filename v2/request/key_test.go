package request

import (
	"testing"
)

func TestKey_UniqueIdentity(t *testing.T) {
	k1 := NewKey[string]("session")
	k2 := NewKey[string]("session")
	if k1.id == k2.id {
		t.Fatal("keys with same name must have different identities")
	}
}

func TestKey_DifferentTypes(t *testing.T) {
	kStr := NewKey[string]("value")
	kInt := NewKey[int]("value")
	if kStr.id == kInt.id {
		t.Fatal("keys of different types with same name must have different identities")
	}
}

func TestKey_String(t *testing.T) {
	k := NewKey[string]("my-key")
	if k.String() != "my-key" {
		t.Fatalf("expected name %q, got %q", "my-key", k.String())
	}
}

func TestKey_SetGet(t *testing.T) {
	ctx := NewContext(nil)
	k := NewKey[string]("greeting")
	Set(ctx, k, "hello")
	got, ok := Get(ctx, k)
	if !ok {
		t.Fatal("expected value to be present")
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestKey_GetMissing(t *testing.T) {
	ctx := NewContext(nil)
	k := NewKey[int]("missing")
	got, ok := Get(ctx, k)
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if got != 0 {
		t.Fatalf("expected zero value 0, got %d", got)
	}
}

func TestKey_TypeSafety(t *testing.T) {
	ctx := NewContext(nil)
	kStr := NewKey[string]("value")
	kInt := NewKey[int]("value")
	Set(ctx, kStr, "hello")
	// Getting with a different type key should return false, not panic.
	got, ok := Get(ctx, kInt)
	if ok {
		t.Fatalf("expected ok=false for type mismatch, got %v", got)
	}
}

func TestKey_NilContext(t *testing.T) {
	var ctx *Context
	k := NewKey[string]("test")
	// Set and Get on nil context should not panic.
	Set(ctx, k, "value")
	_, ok := Get(ctx, k)
	if ok {
		t.Fatal("expected ok=false on nil context")
	}
}

func TestKey_MultipleKeys(t *testing.T) {
	ctx := NewContext(nil)
	k1 := NewKey[string]("a")
	k2 := NewKey[int]("b")
	k3 := NewKey[bool]("c")
	Set(ctx, k1, "alpha")
	Set(ctx, k2, 42)
	Set(ctx, k3, true)
	if v, ok := Get(ctx, k1); !ok || v != "alpha" {
		t.Fatalf("k1: expected alpha, got %v (ok=%v)", v, ok)
	}
	if v, ok := Get(ctx, k2); !ok || v != 42 {
		t.Fatalf("k2: expected 42, got %d (ok=%v)", v, ok)
	}
	if v, ok := Get(ctx, k3); !ok || v != true {
		t.Fatalf("k3: expected true, got %v (ok=%v)", v, ok)
	}
}

func TestKey_OverwriteValue(t *testing.T) {
	ctx := NewContext(nil)
	k := NewKey[string]("counter")
	Set(ctx, k, "first")
	Set(ctx, k, "second")
	got, ok := Get(ctx, k)
	if !ok || got != "second" {
		t.Fatalf("expected second, got %q (ok=%v)", got, ok)
	}
}
