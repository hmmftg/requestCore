package libNetHttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libNetHttp"
)

func TestNetHttpHandlerBridgesRequestResponseIntoContext(t *testing.T) {
	handler := libNetHttp.NetHttpHandler(func(ctx context.Context) {
		wf := libContext.InitContext(ctx)
		err := wf.Parser.SendJSONRespBody(http.StatusOK, map[string]string{"status": "ok"})
		if err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("User-Id", "bridge-user")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status body to be ok, got %q", body["status"])
	}
}

func TestWithRequestResponseExtractors(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()

	ctx := libNetHttp.WithRequestResponse(context.Background(), req, rec)

	extractedReq, ok := libNetHttp.RequestFromContext(ctx)
	if !ok {
		t.Fatal("expected request in context")
	}
	if extractedReq.URL.Path != "/users/42" {
		t.Fatalf("unexpected extracted path: %s", extractedReq.URL.Path)
	}

	extractedWriter, ok := libNetHttp.ResponseWriterFromContext(ctx)
	if !ok {
		t.Fatal("expected response writer in context")
	}

	extractedWriter.WriteHeader(http.StatusNoContent)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}
