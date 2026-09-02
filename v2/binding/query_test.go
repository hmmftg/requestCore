package binding

import (
	"net/url"
	"testing"

	"github.com/hmmftg/requestCore/v2/request/faketransport"
)

// Focused tests for query binding edge cases.

type queryFull struct {
	Name    string   `query:"name"`
	Age     int      `query:"age"`
	Active  bool     `query:"active"`
	Score   float64  `query:"score"`
	Tags    []string `query:"tags"`
	Count   uint     `query:"count"`
	NoTag   string
	Ignored string `query:"-"`
}

func TestQuery_AllTypes(t *testing.T) {
	q := url.Values{}
	q.Set("name", "Alice")
	q.Set("age", "30")
	q.Set("active", "true")
	q.Set("score", "3.14")
	q.Add("tags", "a")
	q.Add("tags", "b")
	q.Set("count", "5")
	q.Set("ignored", "x")
	q.Set("notag", "y")

	opts := make([]faketransport.Option, 0, len(q))
	for k, vs := range q {
		for i, v := range vs {
			if i == 0 {
				opts = append(opts, faketransport.WithQueryParam(k, v))
			} else {
				opts = append(opts, faketransport.WithQueryParamAdd(k, v))
			}
		}
	}
	ft := faketransport.New("GET", "/x", opts...)

	var req queryFull
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Alice" {
		t.Errorf("Name = %q", req.Name)
	}
	if req.Age != 30 {
		t.Errorf("Age = %d", req.Age)
	}
	if !req.Active {
		t.Errorf("Active = false")
	}
	if req.Score != 3.14 {
		t.Errorf("Score = %f", req.Score)
	}
	if len(req.Tags) != 2 {
		t.Errorf("Tags len = %d", len(req.Tags))
	}
	if req.Count != 5 {
		t.Errorf("Count = %d", req.Count)
	}
	if req.NoTag != "" {
		t.Errorf("NoTag should be empty, got %q", req.NoTag)
	}
	if req.Ignored != "" {
		t.Errorf("Ignored should be empty, got %q", req.Ignored)
	}
}

func TestQuery_EmptyValues(t *testing.T) {
	q := url.Values{}
	q.Set("name", "")
	q.Set("age", "")
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("name", ""),
		faketransport.WithQueryParam("age", ""),
	)
	_ = q
	var req queryFull
	// Empty string for int should error.
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err == nil {
		t.Fatal("expected error for empty int, got nil")
	}
}

func TestQuery_InvalidUint(t *testing.T) {
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("count", "-5"),
	)
	var req queryFull
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err == nil {
		t.Fatal("expected error for negative uint, got nil")
	}
}

func TestQuery_InvalidBool(t *testing.T) {
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("active", "maybe"),
	)
	var req queryFull
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err == nil {
		t.Fatal("expected error for invalid bool, got nil")
	}
}

func TestQuery_InvalidFloat(t *testing.T) {
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("score", "not-a-float"),
	)
	var req queryFull
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err == nil {
		t.Fatal("expected error for invalid float, got nil")
	}
}

func TestQuery_OmitEmptyTag(t *testing.T) {
	type omitReq struct {
		Name string `query:"name,omitempty"`
	}
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("name", "Bob"),
	)
	var req omitReq
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", req.Name)
	}
}

func TestQuery_NoQueryParams(t *testing.T) {
	ft := faketransport.New("GET", "/x")
	var req queryFull
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	// All fields should remain at zero values.
	if req.Name != "" || req.Age != 0 || req.Active != false {
		t.Errorf("expected zero values, got %+v", req)
	}
}

func TestQuery_FallbackToJsonTag(t *testing.T) {
	type fbReq struct {
		Email string `json:"email"`
	}
	ft := faketransport.New("GET", "/x",
		faketransport.WithQueryParam("email", "a@b.com"),
	)
	var req fbReq
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Email != "a@b.com" {
		t.Errorf("Email = %q", req.Email)
	}
}
