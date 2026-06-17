package libChi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hmmftg/requestCore/libChi"
	"github.com/hmmftg/requestCore/libNetHttp"
)

func TestParamsMiddlewareInjectsSingleParam(t *testing.T) {
	router := chi.NewRouter()

	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)
		if got := parser.GetUrlParam("id"); got != "42" {
			t.Fatalf("expected id=42, got %q", got)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
}

func TestParamsMiddlewareInjectsMultipleParamsAndGetUri(t *testing.T) {
	type uriParams struct {
		ID        string `json:"id"`
		AccountID string `json:"accountId"`
	}

	router := chi.NewRouter()

	router.Get("/users/{id}/accounts/{accountId}", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)

		if got := parser.GetUrlParam("id"); got != "42" {
			t.Fatalf("expected id=42, got %q", got)
		}
		if got := parser.GetUrlParam("accountId"); got != "A1" {
			t.Fatalf("expected accountId=A1, got %q", got)
		}
		if _, ok := parser.CheckUrlParam("missing"); ok {
			t.Fatal("missing param should not exist")
		}

		var uri uriParams
		if err := parser.GetUri(&uri); err != nil {
			t.Fatalf("GetUri failed: %v", err)
		}
		if uri.ID != "42" || uri.AccountID != "A1" {
			t.Fatalf("unexpected uri parse result: %+v", uri)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42/accounts/A1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
}

func TestExtractURLParamsMissingRouteContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	params := libChi.ExtractURLParams(req)
	if len(params) != 0 {
		t.Fatalf("expected empty params, got %+v", params)
	}
}

func TestParamsMiddlewareNoopWithoutRouteParams(t *testing.T) {
	handler := libChi.ParamsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parser := libNetHttp.InitContext(r, w)
		if len(parser.GetUrlParams()) != 0 {
			t.Fatalf("expected empty params map, got %+v", parser.GetUrlParams())
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}
