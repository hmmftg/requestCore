package resources

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/handlers"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

func init() {
	// Use chi for tests — it is stdlib-only.
}

// testExecutor creates an executor with a fresh registry and nop telemetry.
func testExecutor() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
}

// testRouter creates a chi router with a mapped error handler.
func testRouter() routing.Router {
	r := v2libChi.NewRouter()
	mapper := response.DefaultMapperRegistry()
	r.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, mapper, err)
	})
	return r
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
	return handlers.Get[struct{}, []TestItem]("list-items", "/items",
		func(ctx *request.Context, req struct{}) ([]TestItem, error) {
			items := make([]TestItem, 0, len(r.items))
			for _, v := range r.items {
				items = append(items, v)
			}
			return items, nil
		},
	)
}

func (r *TestResource) Show() handlers.EndpointRuntime {
	return handlers.Get[struct{}, TestItem]("show-item", "/items/{id}",
		func(ctx *request.Context, req struct{}) (TestItem, error) {
			id, err := GetParsedID[string](ctx, "id")
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
	return handlers.Get[struct{}, TestItem]("new-item", "/items/new",
		func(ctx *request.Context, req struct{}) (TestItem, error) {
			return TestItem{}, nil
		},
	)
}

func (r *TestResource) Create() handlers.EndpointRuntime {
	return handlers.Post[TestItem, TestItem]("create-item", "/items",
		func(ctx *request.Context, req TestItem) (TestItem, error) {
			r.items[req.ID] = req
			return req, nil
		},
	)
}

func (r *TestResource) Edit() handlers.EndpointRuntime {
	return handlers.Get[struct{}, TestItem]("edit-item", "/items/{id}/edit",
		func(ctx *request.Context, req struct{}) (TestItem, error) {
			id, err := GetParsedID[string](ctx, "id")
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
	return handlers.Put[TestItem, TestItem]("update-item", "/items/{id}",
		func(ctx *request.Context, req TestItem) (TestItem, error) {
			id, err := GetParsedID[string](ctx, "id")
			if err != nil {
				return TestItem{}, err
			}
			if _, ok := r.items[id]; !ok {
				return TestItem{}, errors.New("not found")
			}
			req.ID = id
			r.items[id] = req
			return req, nil
		},
	)
}

func (r *TestResource) Destroy() handlers.EndpointRuntime {
	return handlers.Delete[struct{}, struct{}]("destroy-item", "/items/{id}",
		func(ctx *request.Context, req struct{}) (struct{}, error) {
			id, err := GetParsedID[string](ctx, "id")
			if err != nil {
				return struct{}{}, err
			}
			delete(r.items, id)
			return struct{}{}, nil
		},
	)
}

func TestRegister_AllOperations(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: r,
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

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
			var req *http.Request
			var err error
			if tt.body != "" {
				req, err = http.NewRequest(tt.method, srv.URL+tt.path, strings.NewReader(tt.body))
				if err != nil {
					t.Fatalf("NewRequest: %v", err)
				}
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(tt.method, srv.URL+tt.path, nil)
				if err != nil {
					t.Fatalf("NewRequest: %v", err)
				}
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s: request: %v", tt.name, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("%s: expected %d, got %d", tt.name, tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestRegister_NilExecutor(t *testing.T) {
	router := testRouter()
	r := newTestResource()
	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: r,
		Executor: nil,
	})
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestRegister_NilHandlerSkipped(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	// A resource that returns nil for Create.
	wrapper := &partialResourceWrapper{
		TestResource: newTestResource(),
		createFn:     func() handlers.EndpointRuntime { return nil },
	}

	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: wrapper,
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// POST /items should not be registered (nil handler, no Defaults).
	// chi returns 405 when the path exists for other methods (GET /items
	// is registered for List), or 404 if no method is registered.
	resp, err := http.Post(srv.URL+"/items", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 404 or 405 for POST /items (nil handler), got %d", resp.StatusCode)
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
	exec := testExecutor()
	router := testRouter()

	// A resource that returns nil for all operations.
	emptyResource := &emptyResourceImpl{}

	defaults := &ResourceDefaults{Executor: exec}
	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: emptyResource,
		Executor: exec,
		Defaults: defaults,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

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
		req, err := http.NewRequest(tt.method, srv.URL+tt.path, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tt.method, tt.path, err)
		}
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: expected 405, got %d", tt.method, tt.path, resp.StatusCode)
		}
		resp.Body.Close()
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
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:             "/items",
		Resource:         r,
		Executor:         exec,
		EnablePatchAlias: true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// PATCH /items/1 should work as an alias for PUT.
	req, err := http.NewRequest("PATCH", srv.URL+"/items/1", strings.NewReader(`{"id":"1","name":"patched"}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for PATCH alias, got %d", resp.StatusCode)
	}
}

func TestRegister_IntID(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	r := &intIDResource{}
	err := Register[int](router, Config[int]{
		Path:     "/items",
		Resource: r,
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/items/42")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["id"] != 42 {
		t.Fatalf("expected id 42, got %d", result["id"])
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
	return handlers.Get[struct{}, map[string]int]("show-int-item", "/items/{id}",
		func(ctx *request.Context, req struct{}) (map[string]int, error) {
			id, err := GetParsedID[int](ctx, "id")
			if err != nil {
				return nil, err
			}
			return map[string]int{"id": id}, nil
		},
	)
}

// Ensure routing import is used.
var _ routing.Router = (*v2libChi.ChiRouter)(nil)

func TestRegister_PanicRecovery(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	panicResource := &panicResourceImpl{}
	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: panicResource,
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// GET /items should panic and be recovered as 500.
	resp, err := http.Get(srv.URL + "/items")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for panicked resource, got %d", resp.StatusCode)
	}
}

type panicResourceImpl struct{}

func (r *panicResourceImpl) List() handlers.EndpointRuntime {
	return handlers.Get[struct{}, []TestItem]("list-panic", "/items",
		func(ctx *request.Context, req struct{}) ([]TestItem, error) {
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
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	reloadEndpoint := handlers.Post[struct{}, reloadResp]("reload-items", "/items/reload",
		func(ctx *request.Context, req struct{}) (reloadResp, error) {
			return reloadResp{Reloaded: true}, nil
		},
	)

	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: r,
		Executor: exec,
		Custom: []CustomOperation{
			{Method: "POST", Path: "/reload", Endpoint: reloadEndpoint},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// POST /items/reload should return 200 with reloaded=true.
	resp, err := http.Post(srv.URL+"/items/reload", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result reloadResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.Reloaded {
		t.Fatal("expected reloaded=true")
	}
}

// TestRegister_CustomOperationPrecedence verifies that a custom
// operation path (e.g. /items/reload) does not conflict with the
// /{id} route. "reload" should match the custom route, not be
// treated as an ID.
func TestRegister_CustomOperationPrecedence(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	reloadEndpoint := handlers.Post[struct{}, reloadResp]("reload-items-prec", "/items/reload",
		func(ctx *request.Context, req struct{}) (reloadResp, error) {
			return reloadResp{Reloaded: true}, nil
		},
	)

	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: r,
		Executor: exec,
		Custom: []CustomOperation{
			{Method: "POST", Path: "/reload", Endpoint: reloadEndpoint},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// POST /items/reload should hit the custom route (200), not Show (404/405).
	resp, err := http.Post(srv.URL+"/items/reload", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST /items/reload: expected 200 (custom route), got %d", resp.StatusCode)
	}

	// GET /items/1 should still hit Show (200).
	resp2, err := http.Get(srv.URL + "/items/1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("GET /items/1: expected 200 (Show), got %d", resp2.StatusCode)
	}
}

// TestRegister_MultipleCustomOperations verifies that multiple custom
// operations can be registered on the same resource.
func TestRegister_MultipleCustomOperations(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	err := Register[string](router, Config[string]{
		Path:     "/items",
		Resource: r,
		Executor: exec,
		Custom: []CustomOperation{
			{
				Method: "POST",
				Path:   "/reload",
				Endpoint: handlers.Post[struct{}, reloadResp]("reload-multi", "/items/reload",
					func(ctx *request.Context, req struct{}) (reloadResp, error) {
						return reloadResp{Reloaded: true}, nil
					},
				),
			},
			{
				Method: "POST",
				Path:   "/validate",
				Endpoint: handlers.Post[struct{}, map[string]bool]("validate-multi", "/items/validate",
					func(ctx *request.Context, req struct{}) (map[string]bool, error) {
						return map[string]bool{"valid": true}, nil
					},
				),
			},
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// POST /items/reload
	resp, err := http.Post(srv.URL+"/items/reload", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST /items/reload: expected 200, got %d", resp.StatusCode)
	}

	// POST /items/validate
	resp2, err := http.Post(srv.URL+"/items/validate", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("POST /items/validate: expected 200, got %d", resp2.StatusCode)
	}
}

// TestResourceBuilder_Register verifies the fluent builder API.
func TestResourceBuilder_Register(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	r := newTestResource()

	err := NewResource[string]("/items").
		EnablePatch().
		Register(router, exec, r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	// GET /items should work.
	resp, err := http.Get(srv.URL + "/items")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
