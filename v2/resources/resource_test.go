package resources

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/response"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testResource implements Resource[string] for testing.
type testResource struct{}

func (r *testResource) Index() IndexOperation {
	return IndexOperation{
		Title: "list-users",
		Handler: func(trx *ResourceContext) (any, error) {
			return []map[string]any{
				{"id": "1", "name": "alice"},
				{"id": "2", "name": "bob"},
			}, nil
		},
	}
}

func (r *testResource) Show() ShowOperation[string] {
	return ShowOperation[string]{
		Title: "show-user",
		Handler: func(id string, trx *ResourceContext) (any, error) {
			return map[string]any{"id": id, "name": "user-" + id}, nil
		},
	}
}

func (r *testResource) Create() CreateOperation {
	return CreateOperation{
		Title: "create-user",
		Handler: func(trx *ResourceContext) (any, error) {
			return map[string]any{"id": "3", "created": true}, nil
		},
	}
}

func (r *testResource) Update() UpdateOperation[string] {
	return UpdateOperation[string]{
		Title: "update-user",
		Handler: func(id string, trx *ResourceContext) (any, error) {
			return map[string]any{"id": id, "updated": true}, nil
		},
	}
}

func (r *testResource) Patch() PatchOperation[string] {
	return PatchOperation[string]{
		Title: "patch-user",
		Handler: func(id string, trx *ResourceContext) (any, error) {
			return map[string]any{"id": id, "patched": true}, nil
		},
	}
}

func (r *testResource) Destroy() DestroyOperation[string] {
	return DestroyOperation[string]{
		Title: "delete-user",
		Handler: func(id string, trx *ResourceContext) (any, error) {
			return map[string]any{"id": id, "deleted": true}, nil
		},
	}
}

func (r *testResource) New() NewOperation {
	return NewOperation{
		Title: "new-user",
		Handler: func(trx *ResourceContext) (any, error) {
			return map[string]any{"form": "user-form"}, nil
		},
	}
}

func initTestV2() *v2response.Handler {
	registry := v2response.NewRegistry(nil)
	renderer := renderers.JSONRenderer{}
	return v2response.NewHandler(registry, renderer, response.WebHanlder{})
}

func TestRegister_AllOperations(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := initTestV2()

	err := Register[string](router, Config[string]{
		Path:        "/users",
		Resource:    &testResource{},
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tests := []struct {
		method     string
		path       string
		body       string
		expectCode int
	}{
		{"GET", "/users", "", http.StatusOK},
		{"GET", "/users/123", "", http.StatusOK},
		{"POST", "/users", `{"name":"test"}`, http.StatusOK},
		{"PUT", "/users/123", `{"name":"test"}`, http.StatusOK},
		{"PATCH", "/users/123", `{"name":"test"}`, http.StatusOK},
		{"DELETE", "/users/123", "", http.StatusOK},
		{"GET", "/users/new", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			if w.Code != tt.expectCode {
				t.Fatalf("expected %d for %s %s, got %d: %s", tt.expectCode, tt.method, tt.path, w.Code, w.Body.String())
			}

			// Verify response body is valid JSON
			if w.Body.Len() > 0 {
				var resp any
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("invalid JSON response: %v (body: %s)", err, w.Body.String())
				}
			}
		})
	}
}

func TestRegister_NilHandlerSkipped(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := initTestV2()

	partialResource := &partialResource{}

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    partialResource,
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Index should work
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/items", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /items, got %d", w.Code)
	}

	// Create should not be registered (404)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/items", strings.NewReader(`{}`))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for POST /items (nil handler), got %d", w.Code)
	}
}

// partialResource implements only Index and Show.
type partialResource struct{}

func (r *partialResource) Index() IndexOperation {
	return IndexOperation{
		Title: "list-items",
		Handler: func(trx *ResourceContext) (any, error) {
			return []string{"item1", "item2"}, nil
		},
	}
}

func (r *partialResource) Show() ShowOperation[string] {
	return ShowOperation[string]{
		Title: "show-item",
		Handler: func(id string, trx *ResourceContext) (any, error) {
			return map[string]any{"id": id}, nil
		},
	}
}

func (r *partialResource) Create() CreateOperation {
	return CreateOperation{Handler: nil}
}

func (r *partialResource) Update() UpdateOperation[string] {
	return UpdateOperation[string]{Handler: nil}
}

func (r *partialResource) Patch() PatchOperation[string] {
	return PatchOperation[string]{Handler: nil}
}

func (r *partialResource) Destroy() DestroyOperation[string] {
	return DestroyOperation[string]{Handler: nil}
}

func (r *partialResource) New() NewOperation {
	return NewOperation{Handler: nil}
}

func TestRegister_IDParser(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := initTestV2()

	err := Register[string](router, Config[string]{
		Path:        "/users",
		Resource:    &testResource{},
		RespHandler: respHandler,
		IDParam:     "userId",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Show with custom ID param name
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/123", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The default ID parser returns the string value
	if resp["id"] != "123" {
		t.Fatalf("expected id=123, got %v", resp["id"])
	}
}

func TestResourceContext_GetURLParam(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := initTestV2()

	var capturedID string
	r := &captureResource{
		showHandler: func(id string, trx *ResourceContext) (any, error) {
			capturedID = id
			return map[string]any{"id": id}, nil
		},
	}

	Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/items/abc", nil)
	engine.ServeHTTP(w, req)

	if capturedID != "abc" {
		t.Fatalf("expected captured ID 'abc', got %q", capturedID)
	}
}

type captureResource struct {
	showHandler func(string, *ResourceContext) (any, error)
}

func (r *captureResource) Index() IndexOperation {
	return IndexOperation{Handler: nil}
}
func (r *captureResource) Show() ShowOperation[string] {
	return ShowOperation[string]{Title: "show", Handler: r.showHandler}
}
func (r *captureResource) Create() CreateOperation {
	return CreateOperation{Handler: nil}
}
func (r *captureResource) Update() UpdateOperation[string] {
	return UpdateOperation[string]{Handler: nil}
}
func (r *captureResource) Patch() PatchOperation[string] {
	return PatchOperation[string]{Handler: nil}
}
func (r *captureResource) Destroy() DestroyOperation[string] {
	return DestroyOperation[string]{Handler: nil}
}
func (r *captureResource) New() NewOperation {
	return NewOperation{Handler: nil}
}
