package resources

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/v2/handlers"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testRespHandler() *v2response.Handler {
	registry := v2response.NewRegistry(nil)
	registry.SetFallback(v2response.LegacyFallback(response.WebHanlder{
		MessageDesc: make(map[string]string),
		ErrorDesc:   make(map[string]string),
	}))
	return v2response.NewHandler(registry, renderers.JSONRenderer{}, response.WebHanlder{})
}

// TestResource is a minimal Resource[string] implementation for testing.
type TestResource struct {
	items map[string]TestItem
}

type TestItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newTestResource() *TestResource {
	return &TestResource{
		items: map[string]TestItem{
			"1": {ID: "1", Name: "alice"},
			"2": {ID: "2", Name: "bob"},
		},
	}
}

func (r *TestResource) List() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, []TestItem]("list", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, []TestItem]) ([]TestItem, error) {
			items := make([]TestItem, 0, len(r.items))
			for _, v := range r.items {
				items = append(items, v)
			}
			return items, nil
		},
	)
}

func (r *TestResource) Show() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, TestItem]("show", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, TestItem]) (TestItem, error) {
			id, err := GetParsedID[string](trx.V2, "id")
			if err != nil {
				return TestItem{}, err
			}
			item, ok := r.items[id]
			if !ok {
				return TestItem{}, errors.New("not found")
			}
			return item, nil
		},
	)
}

func (r *TestResource) New() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, TestItem]("new", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, TestItem]) (TestItem, error) {
			return TestItem{}, nil
		},
	)
}

func (r *TestResource) Create() *handlers.Endpoint {
	return handlers.NewEndpoint[TestItem, TestItem]("create", libRequest.JSON,
		func(req *TestItem, trx *handlers.HandlerRequest[TestItem, TestItem]) (TestItem, error) {
			r.items[req.ID] = *req
			return *req, nil
		},
	)
}

func (r *TestResource) Edit() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, TestItem]("edit", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, TestItem]) (TestItem, error) {
			id, err := GetParsedID[string](trx.V2, "id")
			if err != nil {
				return TestItem{}, err
			}
			item, ok := r.items[id]
			if !ok {
				return TestItem{}, errors.New("not found")
			}
			return item, nil
		},
	)
}

func (r *TestResource) Update() *handlers.Endpoint {
	return handlers.NewEndpoint[TestItem, TestItem]("update", libRequest.JSON,
		func(req *TestItem, trx *handlers.HandlerRequest[TestItem, TestItem]) (TestItem, error) {
			id, err := GetParsedID[string](trx.V2, "id")
			if err != nil {
				return TestItem{}, err
			}
			if _, ok := r.items[id]; !ok {
				return TestItem{}, errors.New("not found")
			}
			req.ID = id
			r.items[id] = *req
			return *req, nil
		},
	)
}

func (r *TestResource) Destroy() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, struct{}]("destroy", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, struct{}]) (struct{}, error) {
			id, err := GetParsedID[string](trx.V2, "id")
			if err != nil {
				return struct{}{}, err
			}
			delete(r.items, id)
			return struct{}{}, nil
		},
	)
}

func TestRegister_AllOperations(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"List", "GET", "/items", "", 200},
		{"Show", "GET", "/items/1", "", 200},
		{"New", "GET", "/items/new", "", 200},
		{"Create", "POST", "/items", `{"id":"3","name":"charlie"}`, 200},
		{"Edit", "GET", "/items/1/edit", "", 200},
		{"Update", "PUT", "/items/1", `{"id":"1","name":"alice2"}`, 200},
		{"Destroy", "DELETE", "/items/2", "", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			engine.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("%s: expected %d, got %d: %s", tt.name, tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegister_NilHandlerSkipped(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	// A resource that returns nil for Create.
	partialResource := struct {
		*TestResource
		CreateFn func() *handlers.Endpoint
	}{
		TestResource: newTestResource(),
		CreateFn:     func() *handlers.Endpoint { return nil },
	}

	// Use a wrapper that returns nil for Create.
	wrapper := &partialResourceWrapper{
		TestResource: partialResource.TestResource,
		createFn:     func() *handlers.Endpoint { return nil },
	}

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    wrapper,
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// POST /items should return 404 (no route registered).
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/items", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for POST /items (nil handler), got %d", w.Code)
	}
}

type partialResourceWrapper struct {
	*TestResource
	createFn func() *handlers.Endpoint
}

func (w *partialResourceWrapper) Create() *handlers.Endpoint {
	return w.createFn()
}

func TestRegister_Defaults405(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	// A resource that returns nil for all operations.
	emptyResource := &emptyResourceImpl{}

	defaults := &ResourceDefaults{}
	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    emptyResource,
		RespHandler: respHandler,
		Defaults:    defaults,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// All operations should return 405.
	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/items"},
		{"POST", "/items"},
		{"GET", "/items/1"},
		{"GET", "/items/new"},
		{"GET", "/items/1/edit"},
		{"PUT", "/items/1"},
		{"DELETE", "/items/1"},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tt.method, tt.path, nil)
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: expected 405, got %d: %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
}

type emptyResourceImpl struct{}

func (e *emptyResourceImpl) List() *handlers.Endpoint    { return nil }
func (e *emptyResourceImpl) Show() *handlers.Endpoint    { return nil }
func (e *emptyResourceImpl) New() *handlers.Endpoint     { return nil }
func (e *emptyResourceImpl) Create() *handlers.Endpoint  { return nil }
func (e *emptyResourceImpl) Edit() *handlers.Endpoint    { return nil }
func (e *emptyResourceImpl) Update() *handlers.Endpoint  { return nil }
func (e *emptyResourceImpl) Destroy() *handlers.Endpoint { return nil }

func TestRegister_PatchAlias(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:             "/items",
		Resource:         r,
		RespHandler:      respHandler,
		EnablePatchAlias: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// PATCH /items/1 should work as an alias for PUT.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/items/1", strings.NewReader(`{"id":"1","name":"patched"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for PATCH alias, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_IntID(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	r := &intIDResource{}
	err := Register[int](router, Config[int]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/items/42", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["id"] != 42 {
		t.Fatalf("expected id 42, got %d", resp["id"])
	}
}

type intIDResource struct{}

func (r *intIDResource) List() *handlers.Endpoint    { return nil }
func (r *intIDResource) New() *handlers.Endpoint     { return nil }
func (r *intIDResource) Create() *handlers.Endpoint  { return nil }
func (r *intIDResource) Edit() *handlers.Endpoint    { return nil }
func (r *intIDResource) Update() *handlers.Endpoint  { return nil }
func (r *intIDResource) Destroy() *handlers.Endpoint { return nil }

func (r *intIDResource) Show() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, map[string]int]("show", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, map[string]int]) (map[string]int, error) {
			id, err := GetParsedID[int](trx.V2, "id")
			if err != nil {
				return nil, err
			}
			return map[string]int{"id": id}, nil
		},
	)
}

// Ensure routing import is used.
var _ routing.Router = (*v2libGin.GinRouter)(nil)

func TestRegister_PanicRecovery(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	panicResource := &panicResourceImpl{}
	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    panicResource,
		RespHandler: respHandler,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// GET /items should panic and be recovered as 500.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/items", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for panicked resource, got %d: %s", w.Code, w.Body.String())
	}
}

type panicResourceImpl struct{}

func (r *panicResourceImpl) List() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, []TestItem]("list", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, []TestItem]) ([]TestItem, error) {
			panic("resource panic")
		},
	)
}
func (r *panicResourceImpl) Show() *handlers.Endpoint    { return nil }
func (r *panicResourceImpl) New() *handlers.Endpoint     { return nil }
func (r *panicResourceImpl) Create() *handlers.Endpoint  { return nil }
func (r *panicResourceImpl) Edit() *handlers.Endpoint    { return nil }
func (r *panicResourceImpl) Update() *handlers.Endpoint  { return nil }
func (r *panicResourceImpl) Destroy() *handlers.Endpoint { return nil }
