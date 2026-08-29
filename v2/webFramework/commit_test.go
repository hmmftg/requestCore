package webFramework

import (
	"errors"
	"sync"
	"testing"
)

// TestRunBeforeCommitHooks_Idempotent verifies that hooks run exactly once
// even when called multiple times (e.g. from both the parser's SendResponse
// and response.Handler.commit).
func TestRunBeforeCommitHooks_Idempotent(t *testing.T) {
	ctx := &RequestContext{}
	calls := 0
	ctx.AddBeforeCommitHook(func(c *RequestContext) error {
		calls++
		return nil
	})

	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected hooks to run exactly once, got %d", calls)
	}
}

// TestRunBeforeCommitHooks_StrictErrorPreventsCommit verifies that when a
// strict hook returns an error, RunBeforeCommitHooks returns it so the
// caller (response.Handler.commit or parser SendResponse) can abort the
// write. This is the mechanism by which strict session save failure
// prevents the pending success response.
func TestRunBeforeCommitHooks_StrictErrorPreventsCommit(t *testing.T) {
	ctx := &RequestContext{}
	strictErr := errors.New("strict save failed")
	ctx.AddBeforeCommitHook(func(c *RequestContext) error {
		return strictErr
	})

	err := ctx.RunBeforeCommitHooks()
	if err == nil {
		t.Fatal("expected strict hook error to be returned, got nil")
	}
	if !errors.Is(err, strictErr) {
		t.Fatalf("expected strictErr, got %v", err)
	}
}

// TestRunBeforeCommitHooks_AllHooksRunOnFirstError verifies that when a
// hook returns an error, subsequent hooks still run. The first error is
// returned so the caller can decide whether to abort.
func TestRunBeforeCommitHooks_AllHooksRunOnFirstError(t *testing.T) {
	ctx := &RequestContext{}
	order := []string{}
	ctx.AddBeforeCommitHook(func(c *RequestContext) error {
		order = append(order, "first")
		return errors.New("best-effort failure")
	})
	ctx.AddBeforeCommitHook(func(c *RequestContext) error {
		order = append(order, "second")
		return nil
	})

	err := ctx.RunBeforeCommitHooks()
	if err == nil {
		t.Fatal("expected first hook error to be returned, got nil")
	}
	if len(order) != 2 {
		t.Fatalf("expected both hooks to run, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" {
		t.Fatalf("expected order [first, second], got %v", order)
	}
}

// TestRunBeforeCommitHooks_NilHookIgnored verifies that nil hooks are not
// registered.
func TestRunBeforeCommitHooks_NilHookIgnored(t *testing.T) {
	ctx := &RequestContext{}
	ctx.AddBeforeCommitHook(nil)
	ctx.AddBeforeCommitHook(func(c *RequestContext) error {
		return nil
	})
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCommitState_FirstCommitWins verifies that the first MarkCommitted
// call wins and subsequent calls are ignored.
func TestCommitState_FirstCommitWins(t *testing.T) {
	cs := &CommitState{}
	if cs.Committed() {
		t.Fatal("expected not committed initially")
	}
	cs.MarkCommitted(200)
	if !cs.Committed() {
		t.Fatal("expected committed after MarkCommitted")
	}
	if cs.Status() != 200 {
		t.Fatalf("expected status 200, got %d", cs.Status())
	}
	// Second call should be ignored.
	cs.MarkCommitted(500)
	if cs.Status() != 200 {
		t.Fatalf("expected first commit (200) to win, got %d", cs.Status())
	}
}

// TestCommitState_ConcurrentMarkCommitted verifies that concurrent
// MarkCommitted calls do not race and the first one wins.
func TestCommitState_ConcurrentMarkCommitted(t *testing.T) {
	cs := &CommitState{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(status int) {
			defer wg.Done()
			cs.MarkCommitted(status)
		}(200 + i)
	}
	wg.Wait()
	if !cs.Committed() {
		t.Fatal("expected committed after concurrent calls")
	}
	// Status should be one of the concurrent values (first winner).
	if cs.Status() < 200 || cs.Status() >= 300 {
		t.Fatalf("expected status in [200,300), got %d", cs.Status())
	}
}

// TestRequestContext_Committed_NoCommitState verifies that Committed
// returns false when no CommitState is associated.
func TestRequestContext_Committed_NoCommitState(t *testing.T) {
	ctx := &RequestContext{}
	if ctx.Committed() {
		t.Fatal("expected false with nil commit state")
	}
}

// TestRequestContext_MarkCommitted_NoCommitState verifies that
// MarkCommitted is a no-op when no CommitState is associated.
func TestRequestContext_MarkCommitted_NoCommitState(t *testing.T) {
	ctx := &RequestContext{}
	ctx.MarkCommitted(200)
	if ctx.Committed() {
		t.Fatal("expected false with nil commit state after MarkCommitted")
	}
}
