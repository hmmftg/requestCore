package binding

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/request/faketransport"
)

// Focused tests for JSON binding edge cases.

type jsonNested struct {
	User jsonUser `json:"user"`
}

type jsonUser struct {
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Tags    []string `json:"tags"`
	Address *struct {
		City string `json:"city"`
		Zip  string `json:"zip"`
	} `json:"address"`
}

func TestJSON_NestedStruct(t *testing.T) {
	body := `{"user":{"name":"Alice","email":"a@x.com","tags":["a","b"]}}`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonNested
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.User.Name != "Alice" {
		t.Errorf("Name = %q", req.User.Name)
	}
	if len(req.User.Tags) != 2 {
		t.Errorf("Tags len = %d", len(req.User.Tags))
	}
}

func TestJSON_PointerField(t *testing.T) {
	body := `{"user":{"name":"Bob","address":{"city":"NYC","zip":"10001"}}}`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonNested
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.User.Address == nil {
		t.Fatal("Address is nil")
	}
	if req.User.Address.City != "NYC" {
		t.Errorf("City = %q", req.User.Address.City)
	}
}

func TestJSON_PointerFieldNull(t *testing.T) {
	body := `{"user":{"name":"Bob","address":null}}`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonNested
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.User.Address != nil {
		t.Errorf("Address should be nil, got %+v", req.User.Address)
	}
}

func TestJSON_ArrayRoot(t *testing.T) {
	body := `[{"name":"a"},{"name":"b"}]`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req []jsonUser
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if len(req) != 2 {
		t.Fatalf("len = %d, want 2", len(req))
	}
	if req[1].Name != "b" {
		t.Errorf("req[1].Name = %q", req[1].Name)
	}
}

func TestJSON_TrailingDataAfterObject(t *testing.T) {
	body := `{"name":"a"} extra`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonUser
	err := Bind(ft.Context(), DefaultJSONPlan, &req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJSON_TwoJSONValues(t *testing.T) {
	body := `{"name":"a"}{"name":"b"}`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonUser
	err := Bind(ft.Context(), DefaultJSONPlan, &req)
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("expected ErrTrailingData, got %v", err)
	}
}

func TestJSON_WhitespaceOnly(t *testing.T) {
	body := "   \n\t  "
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	var req jsonUser
	// Whitespace-only body: json.Decode returns io.EOF, treated as empty.
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
}

func TestJSON_LargeBodyWithinLimit(t *testing.T) {
	// Body just under the limit.
	body := `{"name":"` + strings.Repeat("a", 90) + `"}`
	ft := faketransport.New("POST", "/x", faketransport.WithBody(body))
	plan := Plan{Mode: ModeJSON, MaxBodyBytes: 200}
	var req jsonUser
	if err := Bind(ft.Context(), plan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if len(req.Name) != 90 {
		t.Errorf("Name len = %d, want 90", len(req.Name))
	}
}

func TestJSON_HTTPStatusDefault(t *testing.T) {
	be := &BindingError{Cause: ErrInvalidJSON, Message: "x"}
	if be.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("default status = %d, want 400", be.HTTPStatus())
	}
}

func TestJSON_HTTPStatus413(t *testing.T) {
	be := &BindingError{Status: http.StatusRequestEntityTooLarge, Cause: ErrBodyTooLarge}
	if be.HTTPStatus() != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", be.HTTPStatus())
	}
}
