package validation

import (
	"sort"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/response"
)

type userReq struct {
	Name    string   `json:"name" validate:"required,min=2,max=50"`
	Email   string   `json:"email" validate:"required,email"`
	Age     int      `json:"age" validate:"gte=0,lte=150"`
	Role    string   `json:"role" validate:"oneof=admin user guest"`
	Website string   `json:"website" validate:"omitempty,url"`
	Tags    []string `json:"tags" validate:"omitempty,max=5,dive,min=1"`
}

func TestValidate_Success(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   30,
		Role:  "admin",
		Tags:  []string{"a", "b"},
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestValidate_Required(t *testing.T) {
	v := New()
	req := userReq{}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	// name (required), email (required), role (oneof fails on empty).
	if len(violations) != 3 {
		t.Fatalf("expected 3 violations (name, email, role), got %d: %+v", len(violations), violations)
	}
	// Required violations should have rule "required".
	requiredCount := 0
	for _, vio := range violations {
		if vio.Rule == "required" {
			requiredCount++
		}
	}
	if requiredCount != 2 {
		t.Errorf("expected 2 required violations, got %d", requiredCount)
	}
	// Field names should be wire names (json tags).
	fields := make([]string, 0, len(violations))
	for _, vio := range violations {
		fields = append(fields, vio.Field)
	}
	sort.Strings(fields)
	if fields[0] != "email" || fields[1] != "name" || fields[2] != "role" {
		t.Errorf("fields = %v, want [email name role]", fields)
	}
}

func TestValidate_MinMax(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "A", // too short (min=2)
		Email: "a@b.com",
		Age:   200, // too old (lte=150)
		Role:  "user",
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(violations), violations)
	}
	// Verify rules.
	rules := map[string]string{}
	for _, vio := range violations {
		rules[vio.Field] = vio.Rule
	}
	if rules["name"] != "min" {
		t.Errorf("name rule = %q, want min", rules["name"])
	}
	if rules["age"] != "lte" {
		t.Errorf("age rule = %q, want lte", rules["age"])
	}
}

func TestValidate_Email(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "Alice",
		Email: "not-an-email",
		Role:  "user",
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Field != "email" {
		t.Errorf("field = %q, want email", violations[0].Field)
	}
	if violations[0].Rule != "email" {
		t.Errorf("rule = %q, want email", violations[0].Rule)
	}
}

func TestValidate_OneOf(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "Alice",
		Email: "a@b.com",
		Role:  "superadmin", // not in oneof
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Rule != "oneof" {
		t.Errorf("rule = %q, want oneof", violations[0].Rule)
	}
}

func TestValidate_DeterministicOrdering(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "",    // required
		Email: "bad", // email
		Age:   -1,    // gte
		Role:  "x",   // oneof
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) < 2 {
		t.Fatalf("expected at least 2 violations, got %d", len(violations))
	}
	// Verify sorted by field, then rule.
	for i := 1; i < len(violations); i++ {
		prev := violations[i-1]
		cur := violations[i]
		if prev.Field > cur.Field {
			t.Errorf("violation %d field %q > %q", i-1, prev.Field, cur.Field)
		}
		if prev.Field == cur.Field && prev.Rule > cur.Rule {
			t.Errorf("violation %d rule %q > %q", i-1, prev.Rule, cur.Rule)
		}
	}
}

func TestValidate_NestedStruct(t *testing.T) {
	type inner struct {
		Value string `json:"value" validate:"required"`
	}
	type outer struct {
		Inner inner `json:"inner" validate:"required"`
	}
	v := New()
	req := outer{}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	// Should have at least one violation for the nested required field.
	if len(violations) == 0 {
		t.Fatal("expected at least 1 violation for nested struct")
	}
}

func TestValidate_SliceDive(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "Alice",
		Email: "a@b.com",
		Role:  "user",
		Tags:  []string{"", "valid"}, // first tag empty (min=1 via dive)
	}
	violations, err := v.Validate(&req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("expected at least 1 violation for empty tag")
	}
}

func TestValidate_PointerTarget(t *testing.T) {
	v := New()
	req := &userReq{
		Name:  "Alice",
		Email: "a@b.com",
		Role:  "user",
	}
	violations, err := v.Validate(req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestValidate_ValueTarget(t *testing.T) {
	v := New()
	req := userReq{
		Name:  "Alice",
		Email: "a@b.com",
		Role:  "user",
	}
	violations, err := v.Validate(req)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestValidate_NilTarget(t *testing.T) {
	v := New()
	_, err := v.Validate(nil)
	if err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestValidate_FieldNameResolution_JsonTag(t *testing.T) {
	type r struct {
		UserName string `json:"user_name" validate:"required"`
	}
	v := New()
	violations, _ := v.Validate(&r{})
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Field != "user_name" {
		t.Errorf("field = %q, want user_name", violations[0].Field)
	}
}

func TestValidate_FieldNameResolution_QueryTag(t *testing.T) {
	type r struct {
		Page int `query:"page" validate:"required"`
	}
	v := New()
	violations, _ := v.Validate(&r{})
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Field != "page" {
		t.Errorf("field = %q, want page", violations[0].Field)
	}
}

func TestValidate_FieldNameResolution_NoTag(t *testing.T) {
	type r struct {
		Name string `validate:"required"`
	}
	v := New()
	violations, _ := v.Validate(&r{})
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Field != "Name" {
		t.Errorf("field = %q, want Name", violations[0].Field)
	}
}

func TestValidate_MessageContainsField(t *testing.T) {
	v := New()
	violations, _ := v.Validate(&userReq{})
	for _, vio := range violations {
		if !strings.Contains(vio.Message, vio.Field) {
			t.Errorf("message %q does not contain field %q", vio.Message, vio.Field)
		}
	}
}

func TestValidate_ViolationType(t *testing.T) {
	v := New()
	violations, _ := v.Validate(&userReq{})
	for _, vio := range violations {
		// Ensure it's the response.Violation type.
		var _ response.Violation = vio
	}
}
