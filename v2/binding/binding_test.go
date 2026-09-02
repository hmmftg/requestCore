package binding

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/request/faketransport"
)

// helper to build a fake transport with a JSON body.
func fakeWithBody(method, path, body string) *faketransport.FakeTransport {
	return faketransport.New(method, path, faketransport.WithBody(body))
}

// helper to build a fake transport with query parameters from url.Values.
func fakeWithQuery(method, path string, q url.Values) *faketransport.FakeTransport {
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
	return faketransport.New(method, path, opts...)
}

// helper to build a fake transport with path parameters.
func fakeWithPath(method, path string, params map[string]string) *faketransport.FakeTransport {
	opts := make([]faketransport.Option, 0, len(params))
	for k, v := range params {
		opts = append(opts, faketransport.WithPathParam(k, v))
	}
	return faketransport.New(method, path, opts...)
}

// helper to build a fake transport with headers.
func fakeWithHeader(method, path string, h http.Header) *faketransport.FakeTransport {
	opts := make([]faketransport.Option, 0, len(h))
	for k, vs := range h {
		for _, v := range vs {
			opts = append(opts, faketransport.WithHeader(k, v))
		}
	}
	return faketransport.New(method, path, opts...)
}

type jsonReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

type queryReq struct {
	Name string   `query:"name"`
	Age  int      `query:"age"`
	Tags []string `query:"tags"`
}

type pathReq struct {
	ID   string `path:"id"`
	Slug string `path:"slug"`
}

type headerReq struct {
	Auth    string `header:"X-Auth"`
	TraceID string `header:"X-Trace-Id"`
}

func TestBindJSON_Success(t *testing.T) {
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	ft := fakeWithBody("POST", "/users", body)
	var req jsonReq
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Alice" {
		t.Errorf("Name = %q, want %q", req.Name, "Alice")
	}
	if req.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", req.Email, "alice@example.com")
	}
	if req.Age != 30 {
		t.Errorf("Age = %d, want 30", req.Age)
	}
}

func TestBindJSON_EmptyBody(t *testing.T) {
	ft := fakeWithBody("POST", "/users", "")
	var req jsonReq
	if err := Bind(ft.Context(), DefaultJSONPlan, &req); err != nil {
		t.Fatalf("Bind with empty body failed: %v", err)
	}
	if req.Name != "" {
		t.Errorf("Name = %q, want empty", req.Name)
	}
}

func TestBindJSON_TrailingData(t *testing.T) {
	body := `{"name":"Alice"}{"name":"Bob"}`
	ft := fakeWithBody("POST", "/users", body)
	var req jsonReq
	err := Bind(ft.Context(), DefaultJSONPlan, &req)
	if err == nil {
		t.Fatal("expected ErrTrailingData, got nil")
	}
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("expected ErrTrailingData, got %v", err)
	}
	var be *BindingError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BindingError, got %T", err)
	}
	if be.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want 400", be.HTTPStatus())
	}
}

func TestBindJSON_InvalidJSON(t *testing.T) {
	body := `{not valid json`
	ft := fakeWithBody("POST", "/users", body)
	var req jsonReq
	err := Bind(ft.Context(), DefaultJSONPlan, &req)
	if err == nil {
		t.Fatal("expected ErrInvalidJSON, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
	var be *BindingError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BindingError, got %T", err)
	}
	if be.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want 400", be.HTTPStatus())
	}
}

func TestBindJSON_BodyTooLarge(t *testing.T) {
	body := strings.Repeat("a", 200)
	ft := fakeWithBody("POST", "/users", body)
	plan := Plan{Mode: ModeJSON, MaxBodyBytes: 100}
	var req jsonReq
	err := Bind(ft.Context(), plan, &req)
	if err == nil {
		t.Fatal("expected ErrBodyTooLarge, got nil")
	}
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
	var be *BindingError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BindingError, got %T", err)
	}
	if be.HTTPStatus() != http.StatusRequestEntityTooLarge {
		t.Errorf("HTTPStatus = %d, want 413", be.HTTPStatus())
	}
}

func TestBindJSON_DisallowUnknownFields(t *testing.T) {
	body := `{"name":"Alice","unknown":"x"}`
	ft := fakeWithBody("POST", "/users", body)
	plan := Plan{Mode: ModeJSON, DisallowUnknownFields: true}
	var req jsonReq
	err := Bind(ft.Context(), plan, &req)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestBindQuery_Success(t *testing.T) {
	q := url.Values{}
	q.Set("name", "Bob")
	q.Set("age", "25")
	q.Add("tags", "go")
	q.Add("tags", "rust")
	ft := fakeWithQuery("GET", "/users", q)
	var req queryReq
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Bob" {
		t.Errorf("Name = %q, want %q", req.Name, "Bob")
	}
	if req.Age != 25 {
		t.Errorf("Age = %d, want 25", req.Age)
	}
	if len(req.Tags) != 2 || req.Tags[0] != "go" || req.Tags[1] != "rust" {
		t.Errorf("Tags = %v, want [go rust]", req.Tags)
	}
}

func TestBindQuery_InvalidInt(t *testing.T) {
	q := url.Values{}
	q.Set("age", "not-a-number")
	ft := fakeWithQuery("GET", "/users", q)
	var req queryReq
	err := Bind(ft.Context(), DefaultQueryPlan, &req)
	if err == nil {
		t.Fatal("expected error for invalid int, got nil")
	}
	var be *BindingError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BindingError, got %T", err)
	}
	if be.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want 400", be.HTTPStatus())
	}
	if be.Field != "age" {
		t.Errorf("Field = %q, want %q", be.Field, "age")
	}
}

func TestBindPath_Success(t *testing.T) {
	params := map[string]string{"id": "123", "slug": "hello-world"}
	ft := fakeWithPath("GET", "/posts/123/hello-world", params)
	var req pathReq
	if err := Bind(ft.Context(), DefaultPathPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.ID != "123" {
		t.Errorf("ID = %q, want %q", req.ID, "123")
	}
	if req.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", req.Slug, "hello-world")
	}
}

func TestBindHeader_Success(t *testing.T) {
	h := http.Header{}
	h.Set("X-Auth", "token-abc")
	h.Set("X-Trace-Id", "trace-123")
	ft := fakeWithHeader("GET", "/users", h)
	var req headerReq
	if err := Bind(ft.Context(), DefaultHeaderPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Auth != "token-abc" {
		t.Errorf("Auth = %q, want %q", req.Auth, "token-abc")
	}
	if req.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want %q", req.TraceID, "trace-123")
	}
}

func TestBind_NoneMode(t *testing.T) {
	ft := fakeWithBody("GET", "/users", "")
	var req jsonReq
	plan := Plan{Mode: ModeNone}
	if err := Bind(ft.Context(), plan, &req); err != nil {
		t.Fatalf("Bind with ModeNone failed: %v", err)
	}
	if req.Name != "" {
		t.Errorf("Name = %q, want empty", req.Name)
	}
}

func TestBind_NilTarget(t *testing.T) {
	ft := fakeWithBody("GET", "/users", "")
	if err := Bind(ft.Context(), DefaultJSONPlan, nil); err == nil {
		t.Fatal("expected error for nil target, got nil")
	}
}

func TestBind_NonPointerTarget(t *testing.T) {
	ft := fakeWithBody("GET", "/users", "")
	req := jsonReq{}
	if err := Bind(ft.Context(), DefaultJSONPlan, req); err == nil {
		t.Fatal("expected error for non-pointer target, got nil")
	}
}

func TestBind_NilPointerTarget(t *testing.T) {
	ft := fakeWithBody("GET", "/users", "")
	var req *jsonReq
	if err := Bind(ft.Context(), DefaultJSONPlan, req); err == nil {
		t.Fatal("expected error for nil pointer target, got nil")
	}
}

func TestBindingError_Unwrap(t *testing.T) {
	be := &BindingError{Cause: ErrInvalidJSON, Message: "test"}
	if !errors.Is(be, ErrInvalidJSON) {
		t.Fatal("errors.Is should match ErrInvalidJSON via Unwrap")
	}
}

func TestBindingError_Is(t *testing.T) {
	be := &BindingError{Cause: ErrTrailingData, Message: "test"}
	if !errors.Is(be, ErrTrailingData) {
		t.Fatal("errors.Is should match ErrTrailingData via Is method")
	}
	if errors.Is(be, ErrInvalidJSON) {
		t.Fatal("errors.Is should NOT match ErrInvalidJSON")
	}
}

func TestBindJSON_FallsBackToJsonTag(t *testing.T) {
	// query binding falls back to json tag when no query tag present.
	type fallbackReq struct {
		Name string `json:"name"`
	}
	q := url.Values{}
	q.Set("name", "Carol")
	ft := fakeWithQuery("GET", "/users", q)
	var req fallbackReq
	if err := Bind(ft.Context(), DefaultQueryPlan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Carol" {
		t.Errorf("Name = %q, want %q", req.Name, "Carol")
	}
}

func TestBindForm_Success(t *testing.T) {
	body := "name=Dave&age=40"
	ft := fakeWithBody("POST", "/users", body)
	plan := Plan{Mode: ModeForm, MaxBodyBytes: 1024}
	var req queryReq
	if err := Bind(ft.Context(), plan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Dave" {
		t.Errorf("Name = %q, want %q", req.Name, "Dave")
	}
	if req.Age != 40 {
		t.Errorf("Age = %d, want 40", req.Age)
	}
}

func TestBindForm_ContentTypeAccepted(t *testing.T) {
	cases := []string{
		"application/x-www-form-urlencoded",
		"application/x-www-form-urlencoded; charset=utf-8",
	}
	for _, ct := range cases {
		t.Run(ct, func(t *testing.T) {
			body := "name=Dave&age=40"
			ft := faketransport.New("POST", "/users",
				faketransport.WithBody(body),
				faketransport.WithHeader("Content-Type", ct),
			)
			plan := Plan{Mode: ModeForm, MaxBodyBytes: 1024}
			var req queryReq
			if err := Bind(ft.Context(), plan, &req); err != nil {
				t.Fatalf("Bind failed for %s: %v", ct, err)
			}
			if req.Name != "Dave" {
				t.Errorf("Name = %q", req.Name)
			}
		})
	}
}

func TestBindForm_ContentTypeRejected(t *testing.T) {
	cases := []string{
		"text/plain",
		"application/json",
		"multipart/form-data; boundary=xyz",
	}
	for _, ct := range cases {
		t.Run(ct, func(t *testing.T) {
			body := "name=Dave&age=40"
			ft := faketransport.New("POST", "/users",
				faketransport.WithBody(body),
				faketransport.WithHeader("Content-Type", ct),
			)
			plan := Plan{Mode: ModeForm, MaxBodyBytes: 1024}
			var req queryReq
			err := Bind(ft.Context(), plan, &req)
			if !errors.Is(err, ErrInvalidContentType) {
				t.Fatalf("expected ErrInvalidContentType for %s, got %v", ct, err)
			}
			var be *BindingError
			if !errors.As(err, &be) {
				t.Fatalf("expected *BindingError, got %T", err)
			}
			if be.HTTPStatus() != http.StatusUnsupportedMediaType {
				t.Errorf("status = %d, want 415", be.HTTPStatus())
			}
		})
	}
}

func TestBindForm_ContentTypeAbsentAccepted(t *testing.T) {
	body := "name=Dave&age=40"
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	plan := Plan{Mode: ModeForm, MaxBodyBytes: 1024}
	var req queryReq
	if err := Bind(ft.Context(), plan, &req); err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if req.Name != "Dave" {
		t.Errorf("Name = %q", req.Name)
	}
}

// Ensure request.Context import is used.
var _ = request.NewContext
