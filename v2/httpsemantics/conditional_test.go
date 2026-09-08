package httpsemantics

import (
	"net/http"
	"testing"
	"time"
)

func TestETagString_Strong(t *testing.T) {
	e := ETag{Value: "abc123"}
	if got := e.String(); got != `"abc123"` {
		t.Errorf("String() = %q, want %q", got, `"abc123"`)
	}
}

func TestETagString_Weak(t *testing.T) {
	e := ETag{Weak: true, Value: "abc123"}
	if got := e.String(); got != `W/"abc123"` {
		t.Errorf("String() = %q, want %q", got, `W/"abc123"`)
	}
}

func TestParseETag_Strong(t *testing.T) {
	e, err := ParseETag(`"abc123"`)
	if err != nil {
		t.Fatalf("ParseETag() error = %v", err)
	}
	if e.Weak {
		t.Error("Weak = true, want false")
	}
	if e.Value != "abc123" {
		t.Errorf("Value = %q, want %q", e.Value, "abc123")
	}
}

func TestParseETag_Weak(t *testing.T) {
	e, err := ParseETag(`W/"abc123"`)
	if err != nil {
		t.Fatalf("ParseETag() error = %v", err)
	}
	if !e.Weak {
		t.Error("Weak = false, want true")
	}
	if e.Value != "abc123" {
		t.Errorf("Value = %q, want %q", e.Value, "abc123")
	}
}

func TestParseETag_Empty(t *testing.T) {
	_, err := ParseETag("")
	if err == nil {
		t.Error("ParseETag(\"\") should error")
	}
}

func TestParseETag_Malformed(t *testing.T) {
	_, err := ParseETag("abc123")
	if err == nil {
		t.Error("ParseETag(\"abc123\") should error")
	}
}

func TestParseETag_UnescapedQuote(t *testing.T) {
	_, err := ParseETag(`"ab"c"`)
	if err == nil {
		t.Error("ParseETag with unescaped quote should error")
	}
}

func TestParseETagList_Single(t *testing.T) {
	tags, err := ParseETagList(`"abc"`)
	if err != nil {
		t.Fatalf("ParseETagList() error = %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("len = %d, want 1", len(tags))
	}
}

func TestParseETagList_Multiple(t *testing.T) {
	tags, err := ParseETagList(`"abc", "def", W/"ghi"`)
	if err != nil {
		t.Fatalf("ParseETagList() error = %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("len = %d, want 3", len(tags))
	}
	if tags[2].Value != "ghi" || !tags[2].Weak {
		t.Errorf("tags[2] = %+v, want weak ghi", tags[2])
	}
}

func TestParseETagList_Wildcard(t *testing.T) {
	tags, err := ParseETagList("*")
	if err != nil {
		t.Fatalf("ParseETagList() error = %v", err)
	}
	if len(tags) != 1 || tags[0].Value != "*" {
		t.Fatalf("tags = %+v, want wildcard", tags)
	}
}

func TestETagStrongEqual(t *testing.T) {
	a := ETag{Value: "abc"}
	b := ETag{Value: "abc"}
	if !a.StrongEqual(b) {
		t.Error("StrongEqual should be true for matching strong tags")
	}
	c := ETag{Weak: true, Value: "abc"}
	if a.StrongEqual(c) {
		t.Error("StrongEqual should be false when one is weak")
	}
}

func TestETagWeakEqual(t *testing.T) {
	a := ETag{Value: "abc"}
	b := ETag{Weak: true, Value: "abc"}
	if !a.WeakEqual(b) {
		t.Error("WeakEqual should be true regardless of weakness")
	}
}

func TestEvaluatePreconditions_IfMatch_Match(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfMatch:      `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: true,
	})
	if result != PreconditionProceed {
		t.Errorf("result = %v, want PreconditionProceed", result)
	}
}

func TestEvaluatePreconditions_IfMatch_NoMatch(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfMatch:      `"abc"`,
		ResourceETag: `"def"`,
		IsSafeMethod: true,
	})
	if result != PreconditionFailed {
		t.Errorf("result = %v, want PreconditionFailed", result)
	}
}

func TestEvaluatePreconditions_IfMatch_Wildcard(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfMatch:      `*`,
		ResourceETag: `"anything"`,
		IsSafeMethod: true,
	})
	if result != PreconditionProceed {
		t.Errorf("result = %v, want PreconditionProceed", result)
	}
}

func TestEvaluatePreconditions_IfMatch_Wildcard_NoResource(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfMatch:      `*`,
		ResourceETag: "",
		IsSafeMethod: true,
	})
	if result != PreconditionFailed {
		t.Errorf("result = %v, want PreconditionFailed", result)
	}
}

func TestEvaluatePreconditions_IfNoneMatch_SafeMethod_304(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfNoneMatch:  `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: true,
	})
	if result != PreconditionNotModified {
		t.Errorf("result = %v, want PreconditionNotModified", result)
	}
}

func TestEvaluatePreconditions_IfNoneMatch_UnsafeMethod_412(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IfNoneMatch:  `"abc"`,
		ResourceETag: `"abc"`,
		IsSafeMethod: false,
	})
	if result != PreconditionFailed {
		t.Errorf("result = %v, want PreconditionFailed", result)
	}
}

func TestEvaluatePreconditions_IfModifiedSince_NotModified(t *testing.T) {
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	since := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	result := EvaluatePreconditions(PreconditionInput{
		IfModifiedSince:  since,
		ResourceModified: modTime,
		IsSafeMethod:     true,
	})
	if result != PreconditionNotModified {
		t.Errorf("result = %v, want PreconditionNotModified", result)
	}
}

func TestEvaluatePreconditions_IfModifiedSince_Modified(t *testing.T) {
	modTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	since := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	result := EvaluatePreconditions(PreconditionInput{
		IfModifiedSince:  since,
		ResourceModified: modTime,
		IsSafeMethod:     true,
	})
	if result != PreconditionProceed {
		t.Errorf("result = %v, want PreconditionProceed", result)
	}
}

func TestEvaluatePreconditions_IfUnmodifiedSince_Modified(t *testing.T) {
	modTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	since := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	result := EvaluatePreconditions(PreconditionInput{
		IfUnmodifiedSince: since,
		ResourceModified:  modTime,
		IsSafeMethod:      true,
	})
	if result != PreconditionFailed {
		t.Errorf("result = %v, want PreconditionFailed", result)
	}
}

func TestEvaluatePreconditions_NoPreconditions(t *testing.T) {
	result := EvaluatePreconditions(PreconditionInput{
		IsSafeMethod: true,
	})
	if result != PreconditionProceed {
		t.Errorf("result = %v, want PreconditionProceed", result)
	}
}

func TestEvaluatePreconditions_Precedence_IfMatchBeforeIfNoneMatch(t *testing.T) {
	// If-Match fails, If-None-Match would succeed. If-Match should win.
	result := EvaluatePreconditions(PreconditionInput{
		IfMatch:      `"abc"`,
		IfNoneMatch:  `"def"`,
		ResourceETag: `"def"`,
		IsSafeMethod: true,
	})
	if result != PreconditionFailed {
		t.Errorf("result = %v, want PreconditionFailed (If-Match takes precedence)", result)
	}
}

func TestIsNoBodyStatus(t *testing.T) {
	if !IsNoBodyStatus(http.StatusNoContent) {
		t.Error("204 should be no-body")
	}
	if !IsNoBodyStatus(http.StatusNotModified) {
		t.Error("304 should be no-body")
	}
	if IsNoBodyStatus(http.StatusOK) {
		t.Error("200 should not be no-body")
	}
}
