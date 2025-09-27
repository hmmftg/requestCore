package libFiber_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libTracing"
	"gotest.tools/v3/assert"
)

func TestFiberTracingMiddleware(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	err := libTracing.InitializeTracing(config)
	assert.NilError(t, err)
	defer func() {
		if err := libTracing.ShutdownTracing(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Create Fiber app
	app := fiber.New()

	// Add tracing middleware
	app.Use(libFiber.TracingMiddleware())

	// Add test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Request-Id", "test-request-123")

	// Execute request
	resp, err := app.Test(req)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestFiberCustomTracingMiddleware(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Create Fiber app
	app := fiber.New()

	// Add custom tracing middleware
	app.Use(libFiber.CustomTracingMiddleware(tm))

	// Add test route
	app.Get("/test", func(c *fiber.Ctx) error {
		// Test span attribute addition
		libFiber.AddSpanAttribute(c, "test.key", "test.value")
		libFiber.AddSpanAttributes(c, map[string]string{
			"test.key2": "test.value2",
		})

		// Test span event
		libFiber.AddSpanEvent(c, "test-event", map[string]string{
			"event.key": "event.value",
		})

		return c.JSON(fiber.Map{"message": "test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Request-Id", "test-request-456")

	// Execute request
	resp, err := app.Test(req)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestFiberSpanError(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Create Fiber app
	app := fiber.New()

	// Add custom tracing middleware
	app.Use(libFiber.CustomTracingMiddleware(tm))

	// Add test route that returns error
	app.Get("/error", func(c *fiber.Ctx) error {
		// Test error recording
		testErr := fmt.Errorf("test error")
		libFiber.RecordSpanError(c, testErr, map[string]string{
			"error.type": "test_error",
		})

		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "test error"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/error", nil)

	// Execute request
	resp, err := app.Test(req)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusInternalServerError)
}

func TestFiberGetSpanFromContext(t *testing.T) {
	// Initialize tracing
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	// Create Fiber app
	app := fiber.New()

	// Add custom tracing middleware
	app.Use(libFiber.CustomTracingMiddleware(tm))

	// Add test route
	app.Get("/span", func(c *fiber.Ctx) error {
		// Test getting span from context
		span := libFiber.GetSpanFromContext(c)
		assert.Assert(t, span != nil, "Span should not be nil")
		assert.Assert(t, span.IsRecording(), "Span should be recording")

		return c.JSON(fiber.Map{"message": "span test"})
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/span", nil)

	// Execute request
	resp, err := app.Test(req)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}
