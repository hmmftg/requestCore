package libGin_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/libTracing"
	"gotest.tools/v3/assert"
)

func TestGinTracingMiddleware(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	err := libTracing.InitializeTracing(config)
	assert.NilError(t, err)
	defer func() {
		if err := libTracing.ShutdownTracing(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create Gin engine
	r := gin.New()

	// Add tracing middleware
	r.Use(libGin.TracingMiddleware())

	// Add test route
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Request-Id", "test-request-123")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	r.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, rr.Code, http.StatusOK)
}

func TestGinCustomTracingMiddleware(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create Gin engine
	r := gin.New()

	// Add custom tracing middleware
	r.Use(libGin.CustomTracingMiddleware(tm))

	// Add test route
	r.GET("/test", func(c *gin.Context) {
		// Test span attribute addition
		libGin.AddSpanAttribute(c, "test.key", "test.value")
		libGin.AddSpanAttributes(c, map[string]string{
			"test.key2": "test.value2",
		})

		// Test span event
		libGin.AddSpanEvent(c, "test-event", map[string]string{
			"event.key": "event.value",
		})

		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Request-Id", "test-request-456")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	r.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, rr.Code, http.StatusOK)
}

func TestGinSpanError(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create Gin engine
	r := gin.New()

	// Add custom tracing middleware
	r.Use(libGin.CustomTracingMiddleware(tm))

	// Add test route that returns error
	r.GET("/error", func(c *gin.Context) {
		// Test error recording
		testErr := fmt.Errorf("test error")
		libGin.RecordSpanError(c, testErr, map[string]string{
			"error.type": "test_error",
		})

		c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/error", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	r.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, rr.Code, http.StatusInternalServerError)
}

func TestGinGetSpanFromContext(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create Gin engine
	r := gin.New()

	// Add custom tracing middleware
	r.Use(libGin.CustomTracingMiddleware(tm))

	// Add test route
	r.GET("/span", func(c *gin.Context) {
		// Test getting span from context
		span := libGin.GetSpanFromContext(c)
		assert.Assert(t, span != nil, "Span should not be nil")
		assert.Assert(t, span.IsRecording(), "Span should be recording")

		c.JSON(http.StatusOK, gin.H{"message": "span test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/span", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute request
	r.ServeHTTP(rr, req)

	// Check response
	assert.Equal(t, rr.Code, http.StatusOK)
}
