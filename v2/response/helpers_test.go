package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
)

func TestNoContent(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	if err := NoContent(ctx, transport); err != nil {
		t.Fatalf("NoContent failed: %v", err)
	}
	if !transport.committed {
		t.Fatal("expected transport to be committed")
	}
	if transport.status != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", transport.status)
	}
	if transport.body != nil {
		t.Fatalf("expected nil body, got %v", transport.body)
	}
}

func TestRedirect(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	if err := Redirect(ctx, transport, http.StatusFound, "/new"); err != nil {
		t.Fatalf("Redirect failed: %v", err)
	}
	if transport.status != http.StatusFound {
		t.Fatalf("expected 302, got %d", transport.status)
	}
	if transport.headers.Get("Location") != "/new" {
		t.Fatalf("expected Location /new, got %q", transport.headers.Get("Location"))
	}
}

func TestRedirect_InvalidStatus(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	err := Redirect(ctx, transport, http.StatusOK, "/new")
	if err == nil {
		t.Fatal("expected error for 200 redirect")
	}
}

func TestWriteSuccess(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	body := []byte(`{"message":"ok"}`)
	if err := WriteSuccess(ctx, transport, http.StatusOK, "application/json", body); err != nil {
		t.Fatalf("WriteSuccess failed: %v", err)
	}
	if transport.status != http.StatusOK {
		t.Fatalf("expected 200, got %d", transport.status)
	}
	if string(transport.body) != `{"message":"ok"}` {
		t.Fatalf("expected body, got %q", string(transport.body))
	}
}

func TestWriteProblem(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	problem := NewProblemWithCode(http.StatusBadRequest, "Bad Request", "BAD_INPUT")
	if err := WriteProblem(ctx, transport, problem); err != nil {
		t.Fatalf("WriteProblem failed: %v", err)
	}
	if transport.status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", transport.status)
	}
	if transport.headers.Get("Content-Type") != ProblemContentType {
		t.Fatalf("expected %s, got %s", ProblemContentType, transport.headers.Get("Content-Type"))
	}

	var p map[string]any
	if err := json.Unmarshal(transport.body, &p); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if p["status"].(float64) != 400 {
		t.Fatalf("expected status 400, got %v", p["status"])
	}
}

func TestWriteProblemFromError(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	mapper := NewMapperRegistry()
	_ = mapper.Register(
		func(err error) bool {
			var ce *customTestError
			return errors.As(err, &ce)
		},
		func(err error) *Problem {
			return NewProblemWithCode(http.StatusNotFound, "Not Found", "NOT_FOUND")
		},
	)

	err := WriteProblemFromError(ctx, transport, mapper, &customTestError{msg: "missing"})
	if err != nil {
		t.Fatalf("WriteProblemFromError failed: %v", err)
	}
	if transport.status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", transport.status)
	}
}

func TestWriteProblemFromError_NilMapper(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &helpersTransport{}

	err := WriteProblemFromError(ctx, transport, nil, errors.New("oops"))
	if err != nil {
		t.Fatalf("WriteProblemFromError failed: %v", err)
	}
	if transport.status != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", transport.status)
	}
}

type customTestError struct{ msg string }

func (e *customTestError) Error() string { return e.msg }

type helpersTransport struct {
	status    int
	headers   http.Header
	body      []byte
	committed bool
}

func (t *helpersTransport) WriteResponse(status int, ct string, headers http.Header, body []byte) error {
	t.committed = true
	t.status = status
	t.headers = make(http.Header)
	for k, vs := range headers {
		for _, v := range vs {
			t.headers.Add(k, v)
		}
	}
	if ct != "" {
		t.headers.Set("Content-Type", ct)
	}
	t.body = body
	return nil
}

func (t *helpersTransport) Committed() bool { return t.committed }
