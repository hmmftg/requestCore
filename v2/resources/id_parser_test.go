package resources

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/handlers"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

// intResource is a Resource[int] implementation for testing ID parser
// injection with non-string IDs.
type intResource struct{}

type intItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (r *intResource) List() handlers.EndpointRuntime   { return nil }
func (r *intResource) New() handlers.EndpointRuntime    { return nil }
func (r *intResource) Create() handlers.EndpointRuntime { return nil }

func (r *intResource) Show() handlers.EndpointRuntime {
	return handlers.Get[struct{}, intItem]("show-int", "/ints/{id}",
		func(ctx *request.Context, req struct{}) (intItem, error) {
			id, err := GetParsedIDFromContext[int](ctx, "id")
			if err != nil {
				return intItem{}, err
			}
			return intItem{ID: id, Name: "item"}, nil
		},
	)
}

func (r *intResource) Edit() handlers.EndpointRuntime    { return nil }
func (r *intResource) Update() handlers.EndpointRuntime  { return nil }
func (r *intResource) Destroy() handlers.EndpointRuntime { return nil }

func idParserTestRouter() routing.Router {
	r := v2libChi.NewRouter()
	mapper := response.DefaultMapperRegistry()
	r.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, mapper, err)
	})
	return r
}

func idParserTestExec() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
}

func TestIDParser_InjectedForShowEndpoint(t *testing.T) {
	exec := idParserTestExec()
	r := idParserTestRouter()

	err := Register[int](r, Config[int]{
		Path:     "/ints",
		Resource: &intResource{},
		Executor: exec,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	// Valid int ID
	resp, err := http.Get(srv.URL + "/ints/42")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for valid int ID, got %d", resp.StatusCode)
	}

	// Invalid int ID should produce 400
	resp2, err := http.Get(srv.URL + "/ints/abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid int ID, got %d", resp2.StatusCode)
	}
}

func TestIDParser_CustomParser(t *testing.T) {
	exec := idParserTestExec()
	r := idParserTestRouter()

	errInvalidID := errors.New("invalid ID")
	customParser := func(raw string) (int, error) {
		if raw == "777" {
			return 777, nil
		}
		return 0, errInvalidID
	}

	err := Register[int](r, Config[int]{
		Path:     "/custom",
		Resource: &intResource{},
		Executor: exec,
		IDParser: customParser,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	// Custom parser accepts 777
	resp, err := http.Get(srv.URL + "/custom/777")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for custom-valid ID, got %d", resp.StatusCode)
	}

	// Custom parser rejects 42 (which would be valid for default parser)
	resp2, err := http.Get(srv.URL + "/custom/42")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 400 {
		t.Fatalf("expected 400 for custom-invalid ID, got %d", resp2.StatusCode)
	}
}
