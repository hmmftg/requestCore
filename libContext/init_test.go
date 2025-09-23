package libContext

import (
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/libNetHttp"
)

func TestInitNetHttpContext(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Id", "test-user")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Initialize net/http context
	wf := InitNetHttpContext(req, w, false)

	// Verify the context was created correctly
	if wf.Parser == nil {
		t.Error("Parser should not be nil")
	}

	// Verify we can cast to NetHttpParser
	parser, ok := wf.Parser.(libNetHttp.NetHttpParser)
	if !ok {
		t.Error("Parser should be of type NetHttpParser")
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

func TestInitNetHttpContextWithUnknownUser(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Initialize net/http context with unknown user
	wf := InitNetHttpContext(req, w, true)

	// Verify the context was created correctly
	if wf.Parser == nil {
		t.Error("Parser should not be nil")
	}

	// Verify we can cast to NetHttpParser
	parser, ok := wf.Parser.(libNetHttp.NetHttpParser)
	if !ok {
		t.Error("Parser should be of type NetHttpParser")
	}

	// Test that unknown user is set
	if parser.GetLocalString("userId") != "unknown" {
		t.Errorf("Expected userId to be 'unknown', got %s", parser.GetLocalString("userId"))
	}
}
