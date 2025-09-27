package libTracing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hmmftg/requestCore/libTracing"
	"gotest.tools/v3/assert"
)

func TestTracingManager(t *testing.T) {
	// Test with default config
	config := libTracing.DefaultTracingConfig()
	tm, err := libTracing.NewTracingManager(config)
	assert.NilError(t, err, "NewTracingManager should not return an error")
	assert.Assert(t, tm != nil, "TracingManager should not be nil")
	defer tm.Shutdown(context.Background())

	// Test span creation
	ctx := context.Background()
	ctx, span := tm.StartSpan(ctx, "test-span")
	assert.Assert(t, span != nil, "Span should not be nil")
	assert.Assert(t, span.IsRecording(), "Span should be recording")
	span.End()

	// Test StartSpanWithAttributes
	attrs := map[string]string{"key1": "value1", "key2": "value2"}
	ctx2, span2 := tm.StartSpanWithAttributes(ctx, "test-span-with-attrs", attrs)
	assert.Assert(t, span2 != nil, "Span2 should not be nil")
	assert.Assert(t, span2.IsRecording(), "Span2 should be recording")
	span2.End()

	// Test AddSpanAttributes
	tm.AddSpanAttributes(ctx2, map[string]string{"newKey": "newValue"})

	// Test AddSpanEvent
	tm.AddSpanEvent(ctx2, "testEvent", map[string]string{"eventKey": "eventValue"})

	// Test RecordError
	tm.RecordError(ctx2, fmt.Errorf("test error"), map[string]string{"errorType": "test"})
}

func TestTracingConfigLoader(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "tracing.yaml")
	configContent := `
service_name: "test-service"
service_version: "0.1.0"
exporter: "jaeger"
jaeger_endpoint: "http://localhost:14268/api/traces"
sampling_ratio: 0.5
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NilError(t, err, "Failed to write temp config file")

	loader := libTracing.NewTracingConfigLoader(configPath)
	err = loader.LoadConfig()
	assert.NilError(t, err, "LoadConfig should not return an error")
	config := loader.GetConfig()
	assert.Assert(t, config != nil, "Loaded config should not be nil")

	// Verify loaded config
	assert.Equal(t, config.ServiceName, "test-service")
	assert.Equal(t, config.Exporter, "jaeger")
	assert.Equal(t, config.JaegerEndpoint, "http://localhost:14268/api/traces")
	assert.Equal(t, config.SamplingRatio, 0.5)

	// Test GetGlobalTracingConfig
	globalConfig := libTracing.GetGlobalTracingConfig()
	assert.Assert(t, globalConfig == config, "Global config should be the same as loaded config")

	// Test error case for non-existent file, should fall back to default
	nonExistentLoader := libTracing.NewTracingConfigLoader("non_existent_file.yaml")
	err = nonExistentLoader.LoadConfig()
	assert.Assert(t, err != nil, "Loading non-existent file should return an error")
	defaultConfig := nonExistentLoader.GetConfig()
	assert.Assert(t, defaultConfig == nil, "Config should be nil when loading fails")

	// Test that GetGlobalTracingConfig still works
	globalConfig2 := libTracing.GetGlobalTracingConfig()
	assert.Assert(t, globalConfig2 != nil, "Global config should not be nil")
	assert.Equal(t, globalConfig2.ServiceName, "test-service") // Should be the previously loaded config
}
