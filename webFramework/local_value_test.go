package webFramework_test

import (
	"testing"

	"github.com/hmmftg/requestCore/webFramework"
	"gotest.tools/v3/assert"
)

func TestGetLocalOrDefault_MatchingType(t *testing.T) {
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{"tenant": "acme"}},
	}
	got := webFramework.GetLocalOrDefault(w, "tenant", "fallback")
	assert.Equal(t, got, "acme")
}

func TestGetLocalOrDefault_MissingKey(t *testing.T) {
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{}},
	}
	got := webFramework.GetLocalOrDefault(w, "missing", "fallback")
	assert.Equal(t, got, "fallback")
}

func TestGetLocalOrDefault_WrongType(t *testing.T) {
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{"count": 42}},
	}
	got := webFramework.GetLocalOrDefault(w, "count", "fallback")
	assert.Equal(t, got, "fallback")
}

func TestGetLocalOrDefault_NilParser(t *testing.T) {
	w := webFramework.WebFramework{}
	got := webFramework.GetLocalOrDefault(w, "anything", "fallback")
	assert.Equal(t, got, "fallback")
}

func TestGetLocalOrDefault_TypedNilPointer(t *testing.T) {
	type custom struct{ Name string }
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{"ptr": (*custom)(nil)}},
	}
	// A typed nil pointer still satisfies the type assertion, so it is returned
	// as-is rather than falling back to the default. This documents standard Go
	// type-assertion semantics for callers that store optional pointers.
	got := webFramework.GetLocalOrDefault(w, "ptr", &custom{Name: "default"})
	assert.Assert(t, got == nil, "typed nil pointer should be returned unchanged")
}

func TestGetLocalOrDefault_NonNilPointerValue(t *testing.T) {
	type custom struct{ Name string }
	stored := &custom{Name: "stored"}
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{"ptr": stored}},
	}
	got := webFramework.GetLocalOrDefault(w, "ptr", &custom{Name: "default"})
	assert.Equal(t, got.Name, "stored")
}

func TestGetLocalOrDefault_DistinctDefaults(t *testing.T) {
	cases := []struct {
		name         string
		defaultValue int
		want         int
	}{
		{name: "default-zero", defaultValue: 0, want: 0},
		{name: "default-positive", defaultValue: 7, want: 7},
		{name: "default-negative", defaultValue: -3, want: -3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := webFramework.WebFramework{
				Parser: webFramework.FakeParser{Locals: map[string]any{}},
			}
			got := webFramework.GetLocalOrDefault(w, "absent", tc.defaultValue)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestGetLocalOrDefault_IntMatchingType(t *testing.T) {
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: map[string]any{"count": 99}},
	}
	got := webFramework.GetLocalOrDefault(w, "count", -1)
	assert.Equal(t, got, 99)
}

func TestGetLocalOrDefault_DoesNotMutateLocals(t *testing.T) {
	locals := map[string]any{"tenant": "acme"}
	w := webFramework.WebFramework{
		Parser: webFramework.FakeParser{Locals: locals},
	}
	_ = webFramework.GetLocalOrDefault(w, "tenant", "fallback")
	_ = webFramework.GetLocalOrDefault(w, "absent", "fallback")
	assert.Equal(t, locals["tenant"], "acme")
	_, present := locals["absent"]
	assert.Assert(t, !present, "GetLocalOrDefault must not register missing keys")
}
