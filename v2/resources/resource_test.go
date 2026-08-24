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

func (r *TestResource) List() handlers.EndpointRuntime {
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

func (r *TestResource) Show() handlers.EndpointRuntime {
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

func (r *TestResource) New() handlers.EndpointRuntime {
	return handlers.NewEndpoint[struct{}, TestItem]("new", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, TestItem]) (TestItem, error) {
			return TestItem{}, nil
		},
	)
}

func (r *TestResource) Create() handlers.EndpointRuntime {
	return handlers.NewEndpoint[TestItem, TestItem]("create", libRequest.JSON,
		func(req *TestItem, trx *handlers.HandlerRequest[TestItem, TestItem]) (TestItem, error) {
			r.items[req.ID] = *req
			return *req, nil
		},
	)
}

func (r *TestResource) Edit() handlers.EndpointRuntime {
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

func (r *TestResource) Update() handlers.EndpointRuntime {
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

func (r *TestResource) Destroy() handlers.EndpointRuntime {
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
		CreateFn func() handlers.EndpointRuntime
	}{
		TestResource: newTestResource(),
		CreateFn:     func() handlers.EndpointRuntime { return nil },
	}

	// Use a wrapper that returns nil for Create.
	wrapper := &partialResourceWrapper{
		TestResource: partialResource.TestResource,
		createFn:     func() handlers.EndpointRuntime { return nil },
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
	createFn func() handlers.EndpointRuntime
}

func (w *partialResourceWrapper) Create() handlers.EndpointRuntime {
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

func (e *emptyResourceImpl) List() handlers.EndpointRuntime    { return nil }
func (e *emptyResourceImpl) Show() handlers.EndpointRuntime    { return nil }
func (e *emptyResourceImpl) New() handlers.EndpointRuntime     { return nil }
func (e *emptyResourceImpl) Create() handlers.EndpointRuntime  { return nil }
func (e *emptyResourceImpl) Edit() handlers.EndpointRuntime    { return nil }
func (e *emptyResourceImpl) Update() handlers.EndpointRuntime  { return nil }
func (e *emptyResourceImpl) Destroy() handlers.EndpointRuntime { return nil }

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

func (r *intIDResource) List() handlers.EndpointRuntime    { return nil }
func (r *intIDResource) New() handlers.EndpointRuntime     { return nil }
func (r *intIDResource) Create() handlers.EndpointRuntime  { return nil }
func (r *intIDResource) Edit() handlers.EndpointRuntime    { return nil }
func (r *intIDResource) Update() handlers.EndpointRuntime  { return nil }
func (r *intIDResource) Destroy() handlers.EndpointRuntime { return nil }

func (r *intIDResource) Show() handlers.EndpointRuntime {
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

func (r *panicResourceImpl) List() handlers.EndpointRuntime {
	return handlers.NewEndpoint[struct{}, []TestItem]("list", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, []TestItem]) ([]TestItem, error) {
			panic("resource panic")
		},
	)
}
func (r *panicResourceImpl) Show() handlers.EndpointRuntime    { return nil }
func (r *panicResourceImpl) New() handlers.EndpointRuntime     { return nil }
func (r *panicResourceImpl) Create() handlers.EndpointRuntime  { return nil }
func (r *panicResourceImpl) Edit() handlers.EndpointRuntime    { return nil }
func (r *panicResourceImpl) Update() handlers.EndpointRuntime  { return nil }
func (r *panicResourceImpl) Destroy() handlers.EndpointRuntime { return nil }

// reloadResp is the response type for the custom Reload operation.
type reloadResp struct {
	Reloaded bool `json:"reloaded"`
}

// TestRegister_CustomOperation verifies that a custom (non-CRUD)
// operation like Reload is registered and accessible.
func TestRegister_CustomOperation(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()
	r := newTestResource()

	reloadEndpoint := handlers.NewEndpoint[struct{}, reloadResp](
		"reload-items",
		libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, reloadResp]) (reloadResp, error) {
			return reloadResp{Reloaded: true}, nil
		},
	)

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
		Custom: []CustomOperation{
			{Method: "POST", Path: "/reload", Endpoint: reloadEndpoint},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// POST /items/reload should return 200 with reloaded=true.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/items/reload", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp reloadResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Reloaded {
		t.Fatal("expected reloaded=true")
	}
}

// TestRegister_CustomOperationPrecedence verifies that a custom
// operation path (e.g. /items/reload) does not conflict with the
// /{id} route. "reload" should match the custom route, not be
// treated as an ID.
func TestRegister_CustomOperationPrecedence(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()
	r := newTestResource()

	reloadEndpoint := handlers.NewEndpoint[struct{}, reloadResp](
		"reload-items",
		libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, reloadResp]) (reloadResp, error) {
			return reloadResp{Reloaded: true}, nil
		},
	)

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
		Custom: []CustomOperation{
			{Method: "POST", Path: "/reload", Endpoint: reloadEndpoint},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// POST /items/reload should hit the custom route (200), not Show (404).
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/items/reload", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("POST /items/reload: expected 200 (custom route), got %d: %s", w.Code, w.Body.String())
	}

	// GET /items/1 should still hit Show (200).
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/items/1", nil)
	engine.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("GET /items/1: expected 200 (Show), got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestRegister_MultipleCustomOperations verifies that multiple custom
// operations can be registered on the same resource.
func TestRegister_MultipleCustomOperations(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:        "/items",
		Resource:    r,
		RespHandler: respHandler,
		Custom: []CustomOperation{
			{
				Method: "POST",
				Path:   "/reload",
				Endpoint: handlers.NewEndpoint[struct{}, reloadResp](
					"reload-items", libRequest.NoBinding,
					func(req *struct{}, trx *handlers.HandlerRequest[struct{}, reloadResp]) (reloadResp, error) {
						return reloadResp{Reloaded: true}, nil
					},
				),
			},
			{
				Method: "POST",
				Path:   "/validate",
				Endpoint: handlers.NewEndpoint[struct{}, map[string]bool](
					"validate-items", libRequest.NoBinding,
					func(req *struct{}, trx *handlers.HandlerRequest[struct{}, map[string]bool]) (map[string]bool, error) {
						return map[string]bool{"valid": true}, nil
					},
				),
			},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// POST /items/reload
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/items/reload", nil)
	engine.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("POST /items/reload: expected 200, got %d", w.Code)
	}

	// POST /items/validate
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/items/validate", nil)
	engine.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("POST /items/validate: expected 200, got %d", w2.Code)
	}
}
