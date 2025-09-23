package libNetHttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNetHttpParser(t *testing.T) {
	// Create a test request
	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Id", "test-user")
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Initialize parser
	parser := InitContext(req, w)

	// Test basic methods
	if parser.GetMethod() != "GET" {
		t.Errorf("Expected method GET, got %s", parser.GetMethod())
	}

	if parser.GetPath() != "/test" {
		t.Errorf("Expected path /test, got %s", parser.GetPath())
	}

	if parser.GetHeaderValue("User-Id") != "test-user" {
		t.Errorf("Expected User-Id header to be test-user, got %s", parser.GetHeaderValue("User-Id"))
	}

	if parser.GetRawUrlQuery() != "param=value" {
		t.Errorf("Expected query param=value, got %s", parser.GetRawUrlQuery())
	}
}

func TestJSONResponse(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	parser := InitContext(req, w)

	// Test JSON response
	response := map[string]string{
		"message": "test",
		"status":  "success",
	}

	err := parser.SendJSONRespBody(http.StatusOK, response)
	if err != nil {
		t.Errorf("Error sending JSON response: %v", err)
	}

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Error unmarshaling response: %v", err)
	}

	if result["message"] != "test" {
		t.Errorf("Expected message 'test', got %s", result["message"])
	}
}

func TestMiddleware(t *testing.T) {
	// Test CORS middleware
	corsHandler := CORSMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()

	corsHandler.ServeHTTP(w, req)

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS header Access-Control-Allow-Origin to be *, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestChainMiddleware(t *testing.T) {
	// Create a simple middleware that adds a header
	addHeaderMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "test-value")
			next.ServeHTTP(w, r)
		})
	}

	// Chain middleware
	handler := ChainMiddleware(
		addHeaderMiddleware,
		LoggingMiddleware(),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that header was added
	if w.Header().Get("X-Test") != "test-value" {
		t.Errorf("Expected X-Test header to be test-value, got %s", w.Header().Get("X-Test"))
	}
}
