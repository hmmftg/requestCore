package response

import (
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
)

// fakeTransport implements Transport for testing.
type fakeTransport struct {
	mu        sync.Mutex
	committed bool
	status    int
	ct        string
	headers   http.Header
	body      []byte
	writeErr  error
}

func (t *fakeTransport) WriteResponse(status int, ct string, headers http.Header, body []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.committed {
		return nil
	}
	if t.writeErr != nil {
		return t.writeErr
	}
	t.committed = true
	t.status = status
	t.ct = ct
	t.headers = make(http.Header)
	for k, vs := range headers {
		for _, v := range vs {
			t.headers.Add(k, v)
		}
	}
	t.body = body
	return nil
}

func (t *fakeTransport) Committed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.committed
}

func (t *fakeTransport) Status() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func (t *fakeTransport) Headers() http.Header {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := make(http.Header, len(t.headers))
	for k, vs := range t.headers {
		h[k] = append([]string(nil), vs...)
	}
	return h
}

func (t *fakeTransport) Body() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.body
}

func TestCommitCoordinator_Success(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	if !transport.Committed() {
		t.Fatal("expected transport committed")
	}
	if transport.Status() != 200 {
		t.Fatalf("expected status 200, got %d", transport.Status())
	}
	if transport.ct != "application/json" {
		t.Fatalf("expected application/json, got %s", transport.ct)
	}
	if string(transport.Body()) != `{"ok":true}` {
		t.Fatalf("expected body, got %s", string(transport.Body()))
	}
	if cc.State() != StateObserved {
		t.Fatalf("expected Observed, got %s", cc.State())
	}
}

func TestCommitCoordinator_HookFailure(t *testing.T) {
	ctx := request.NewContext(nil)
	hookErr := errors.New("hook failed")
	ctx.AddBeforeCommitHook(func() error { return hookErr })

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error from hook failure")
	}
	// The coordinator should have written a Problem response.
	if !transport.Committed() {
		t.Fatal("expected transport committed with problem response")
	}
	if transport.Status() != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", transport.Status())
	}
	if cc.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", cc.State())
	}
}

func TestCommitCoordinator_TransportFailure(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &fakeTransport{writeErr: errors.New("connection reset")}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`{}`))
	if err == nil {
		t.Fatal("expected transport error")
	}
	if cc.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", cc.State())
	}
}

func TestCommitCoordinator_PreservesHeadersOnError(t *testing.T) {
	ctx := request.NewContext(nil)
	// Simulate session middleware setting Set-Cookie.
	ctx.Response().AddHeader("Set-Cookie", "session=abc; Path=/")
	ctx.Response().AddHeader("Set-Cookie", "flash=hello; Path=/")

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	problem := NewProblemWithCode(http.StatusBadRequest, "Bad Request", "BAD_INPUT")
	err := cc.CommitProblem(ctx, transport, problem)
	if err != nil {
		t.Fatalf("CommitProblem failed: %v", err)
	}
	if !transport.Committed() {
		t.Fatal("expected transport committed")
	}
	if transport.Status() != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", transport.Status())
	}
	// Verify Set-Cookie headers are preserved.
	cookies := transport.Headers()["Set-Cookie"]
	if len(cookies) != 2 {
		t.Fatalf("expected 2 Set-Cookie headers, got %d: %v", len(cookies), cookies)
	}
}

func TestCommitCoordinator_BodySuppressedForNoContent(t *testing.T) {
	ctx := request.NewContext(nil)
	ctx.Response().NoContent()

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 204, "", []byte(`should not appear`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	if transport.Body() != nil {
		t.Fatalf("expected nil body for 204, got %s", string(transport.Body()))
	}
}

func TestCommitCoordinator_BodySuppressedForRedirect(t *testing.T) {
	ctx := request.NewContext(nil)
	ctx.Response().Redirect(http.StatusFound, "https://example.com")

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 302, "", []byte(`should not appear`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	if transport.Body() != nil {
		t.Fatalf("expected nil body for redirect, got %s", string(transport.Body()))
	}
	loc := transport.Headers().Get("Location")
	if loc != "https://example.com" {
		t.Fatalf("expected Location header, got %q", loc)
	}
}

func TestCommitCoordinator_BodySuppressedFor304(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 304, "", []byte(`should not appear`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	if transport.Body() != nil {
		t.Fatalf("expected nil body for 304, got %s", string(transport.Body()))
	}
}

func TestCommitCoordinator_HooksRunOnceOnSuccessThenError(t *testing.T) {
	ctx := request.NewContext(nil)
	hookCount := 0
	ctx.AddBeforeCommitHook(func() error {
		hookCount++
		return nil
	})

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	// First, CommitSuccess runs hooks and fails at transport.
	transport.writeErr = errors.New("transport error")
	_ = cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`{}`))

	// Now try CommitProblem — hooks should NOT re-run.
	transport2 := &fakeTransport{}
	cc2 := NewCommitCoordinator()
	problem := NewProblemWithCode(http.StatusInternalServerError, "Internal", "INTERNAL")
	_ = cc2.CommitProblem(ctx, transport2, problem)

	if hookCount != 1 {
		t.Fatalf("expected hooks to run exactly once, got %d", hookCount)
	}
}

func TestCommitCoordinator_AlreadyCommittedNoWrite(t *testing.T) {
	ctx := request.NewContext(nil)
	transport := &fakeTransport{}
	// Pre-commit the transport.
	transport.committed = true

	cc := NewCommitCoordinator()
	problem := NewProblemWithCode(http.StatusBadRequest, "Bad", "BAD")
	err := cc.CommitProblem(ctx, transport, problem)
	if err != nil {
		t.Fatalf("expected nil error when already committed, got %v", err)
	}
	if transport.Status() != 0 {
		t.Fatalf("expected no write, got status %d", transport.Status())
	}
}

func TestCommitCoordinator_ContentTypeFromHeader(t *testing.T) {
	ctx := request.NewContext(nil)
	ctx.Response().SetHeader("Content-Type", "application/xml")

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`<xml/>`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	if transport.ct != "application/xml" {
		t.Fatalf("expected application/xml from header, got %s", transport.ct)
	}
}

func TestCommitCoordinator_MultiValueHeaders(t *testing.T) {
	ctx := request.NewContext(nil)
	ctx.Response().AddHeader("Set-Cookie", "a=1")
	ctx.Response().AddHeader("Set-Cookie", "b=2")
	ctx.Response().AddHeader("Set-Cookie", "c=3")

	transport := &fakeTransport{}
	cc := NewCommitCoordinator()

	err := cc.CommitSuccess(ctx, transport, 200, "application/json", []byte(`{}`))
	if err != nil {
		t.Fatalf("CommitSuccess failed: %v", err)
	}
	cookies := transport.Headers()["Set-Cookie"]
	if len(cookies) != 3 {
		t.Fatalf("expected 3 Set-Cookie headers, got %d: %v", len(cookies), cookies)
	}
}
