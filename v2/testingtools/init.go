// Package testingtools provides test utilities for v2 handler and
// middleware testing, including a test parser, test router, and
// initialization helpers.
package testingtools

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/session"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
	"github.com/hmmftg/requestCore/v2/workers"
)

// TestParserV2 is an enhanced FakeParserV2 for handler testing.
// It provides additional fields for asserting response state and
// controlling parser behavior during tests.
type TestParserV2 struct {
	*v2wf.FakeParserV2

	// SendResponseError, when non-nil, is returned by SendResponse
	// instead of capturing the response. Useful for testing error paths.
	SendResponseError error
}

// NewTestParserV2 creates a TestParserV2 with initialized maps.
func NewTestParserV2() *TestParserV2 {
	return &TestParserV2{
		FakeParserV2: v2wf.NewFakeParserV2(),
	}
}

// SendResponse captures the response or returns the configured error.
func (p *TestParserV2) SendResponse(status int, contentType string, body []byte) error {
	if p.SendResponseError != nil {
		return p.SendResponseError
	}
	return p.FakeParserV2.SendResponse(status, contentType, body)
}

// TestRequestContext creates a RequestContext suitable for testing
// handlers and middleware without a real HTTP framework.
func TestRequestContext() *v2wf.RequestContext {
	parser := NewTestParserV2()
	return &v2wf.RequestContext{
		Context:       context.Background(),
		LegacyContext: context.Background(),
		Parser:        parser,
		Legacy: webFramework.WebFramework{
			Parser: parser,
		},
	}
}

// TestRequestContextWithSession creates a RequestContext with
// session and flash objects for testing session-aware handlers.
func TestRequestContextWithSession() *v2wf.RequestContext {
	ctx := TestRequestContext()
	store := session.NoOpStore{}
	sess := session.NewSession(store)
	flash := session.NewFlash()
	ctx.Session = sess
	ctx.Flash = flash
	return ctx
}

// InitTestingV2 creates a minimal v2 testing setup with a TestParserV2,
// a response handler with a registry, and default renderers.
// It does not require a real database or framework.
func InitTestingV2() (*v2response.Handler, v2response.Registry, renderers.Renderer) {
	registry := v2response.NewRegistry(nil)
	registry.SetFallback(v2response.LegacyFallback(response.WebHanlder{
		MessageDesc: make(map[string]string),
		ErrorDesc:   make(map[string]string),
	}))
	renderer := renderers.JSONRenderer{}
	handler := v2response.NewHandler(registry, renderer, response.WebHanlder{})
	return handler, registry, renderer
}

// InitTestingV2WithWorker creates a testing setup that includes
// an in-process worker pool for testing async job behavior.
func InitTestingV2WithWorker() (*v2response.Handler, v2response.Registry, renderers.Renderer, workers.Worker) {
	handler, registry, renderer := InitTestingV2()
	worker := workers.NewInProcessWorker(workers.Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	return handler, registry, renderer, worker
}

// AssertResponse checks that the parser captured the expected response.
func AssertResponse(parser *v2wf.FakeParserV2, expectedStatus int, expectedContentType string) bool {
	return parser.ResponseWritten &&
		parser.ResponseStatus == expectedStatus &&
		parser.ResponseContentType == expectedContentType
}

// SetRequestCookie sets a cookie on the test parser for testing
// session middleware.
func SetRequestCookie(ctx *v2wf.RequestContext, name, value string) {
	if p, ok := ctx.Parser.(*TestParserV2); ok {
		p.Cookies[name] = value
	}
}

// GetSetCookies returns the cookies that were set on the response
// during handler execution.
func GetSetCookies(ctx *v2wf.RequestContext) []*http.Cookie {
	if p, ok := ctx.Parser.(*TestParserV2); ok {
		return p.SetCookies
	}
	return nil
}
