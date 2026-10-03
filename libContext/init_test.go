package libContext

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/libNetHttp"
	"github.com/hmmftg/requestCore/webFramework"
)

func TestInitNetHTTPContext(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Id", "test-user")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Initialize net/http context
	wf2 := InitNetHTTPContext(req, w, false)

	// Verify the context was created correctly
	if wf2.Parser == nil {
		t.Error("Parser should not be nil")
	}

	// Verify we can cast to NetHTTPParser
	parser, ok := wf2.Parser.(*libNetHttp.NetHTTPParser)
	if !ok {
		t.Error("Parser should be of type NetHTTPParser")
	}

	// Test basic functionality
	if parser.GetMethod() != "GET" {
		t.Errorf("Expected method GET, got %s", parser.GetMethod())
	}

	if parser.GetPath() != "/test" {
		t.Errorf("Expected path /test, got %s", parser.GetPath())
	}

	if parser.GetHeaderValue("User-Id") != "test-user" {
		t.Errorf("Expected User-Id header to be test-user, got %s", parser.GetHeaderValue("User-Id"))
	}
}

func TestTestingParserGetBody(t *testing.T) {
	type SampleBody struct {
		ID string `json:"id"`
	}

	t.Run("nil body returns ErrEmptyBody", func(t *testing.T) {
		parser := TestingParser{}
		var target SampleBody
		err := parser.GetBody(&target)
		if !errors.Is(err, webFramework.ErrEmptyBody) {
			t.Fatalf("want ErrEmptyBody, got %v", err)
		}
	})

	t.Run("configured BodyError takes precedence", func(t *testing.T) {
		parser := TestingParser{BodyError: errors.New("boom")}
		var target SampleBody
		err := parser.GetBody(&target)
		if err == nil || err.Error() != "boom" {
			t.Fatalf("want configured BodyError, got %v", err)
		}
	})

	t.Run("body fixture populates target", func(t *testing.T) {
		parser := TestingParser{Body: SampleBody{ID: "42"}}
		var target SampleBody
		if err := parser.GetBody(&target); err != nil {
			t.Fatal(err)
		}
		if target.ID != "42" {
			t.Fatalf("want id 42, got %q", target.ID)
		}
	})
}

func TestTestingParserNilFixtures(t *testing.T) {
	// nil header fixture must not panic; a non-nil fixture must bind.
	bound := RequestHeaderStub{}
	parser := TestingParser{Header: &RequestHeaderStub{ID: "7"}}
	if err := parser.GetHeader(&bound); err != nil {
		t.Fatal(err)
	}
	if bound.ID != "7" {
		t.Fatalf("want id 7, got %q", bound.ID)
	}
	parser = TestingParser{}
	if err := parser.GetHeader(&RequestHeaderStub{}); err != nil {
		t.Fatal(err)
	}
	var target map[string]string
	if err := parser.GetURI(&target); err != nil {
		t.Fatal(err)
	}
}

type RequestHeaderStub struct {
	ID string
}

func (h *RequestHeaderStub) GetID() string      { return h.ID }
func (h *RequestHeaderStub) GetUser() string    { return h.ID }
func (h *RequestHeaderStub) GetProgram() string { return h.ID }
func (h *RequestHeaderStub) GetModule() string  { return h.ID }
func (h *RequestHeaderStub) GetMethod() string  { return h.ID }
func (h *RequestHeaderStub) SetUser(string)     {}
func (h *RequestHeaderStub) SetProgram(string)  {}
func (h *RequestHeaderStub) SetModule(string)   {}
func (h *RequestHeaderStub) SetMethod(string)   {}

func TestInitNetHTTPContextWithUnknownUser(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Initialize net/http context with unknown user
	wf := InitNetHTTPContext(req, w, true)

	// Verify the context was created correctly
	if wf.Parser == nil {
		t.Error("Parser should not be nil")
	}

	// Verify we can cast to NetHTTPParser
	parser, ok := wf.Parser.(*libNetHttp.NetHTTPParser)
	if !ok {
		t.Error("Parser should be of type NetHTTPParser")
	}

	// Test that unknown user is set
	if parser.GetLocalString("userId") != "unknown" {
		t.Errorf("Expected userId to be 'unknown', got %s", parser.GetLocalString("userId"))
	}
}
