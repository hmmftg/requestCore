package libTracing

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// TracingConfigLoader handles loading and managing tracing configurations
type TracingConfigLoader struct {
	configPath string
	config     *TracingConfig
}

var globalTracingConfig *TracingConfig
var globalTracingManager *TracingManager

// NewTracingConfigLoader creates a new loader instance
func NewTracingConfigLoader(path string) *TracingConfigLoader {
	return &TracingConfigLoader{
		configPath: path,
	}
}

// LoadConfig loads the tracing configuration from the specified YAML file
func (l *TracingConfigLoader) LoadConfig() error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("failed to read tracing config file %s: %w", l.configPath, err)
	}

	var config TracingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal tracing config from %s: %w", l.configPath, err)
	}
	l.config = &config
	globalTracingConfig = &config // Set the global instance
	return nil
}

// LoadConfigFromEnv loads configuration from environment variables
func (l *TracingConfigLoader) LoadConfigFromEnv() error {
	configPath := os.Getenv("TRACING_CONFIG_PATH")
	if configPath != "" {
		l.configPath = configPath
		return l.LoadConfig()
	}

	// Load from environment variables
	config := &TracingConfig{
		ServiceName:    getEnvOrDefault("TRACING_SERVICE_NAME", "requestCore"),
		ServiceVersion: getEnvOrDefault("TRACING_SERVICE_VERSION", "1.0.0"),
		Environment:    getEnvOrDefault("TRACING_ENVIRONMENT", "development"),
		Exporter:       getEnvOrDefault("TRACING_EXPORTER", "stdout"),
		JaegerEndpoint: getEnvOrDefault("TRACING_JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		ZipkinEndpoint: getEnvOrDefault("TRACING_ZIPKIN_ENDPOINT", "http://localhost:9411/api/v2/spans"),
		SamplingRatio:  getEnvFloatOrDefault("TRACING_SAMPLING_RATIO", 1.0),
		Enabled:        getEnvBoolOrDefault("TRACING_ENABLED", true),
		Attributes:     make(map[string]string),
	}

	// Load additional attributes from environment
	config.Attributes["service.name"] = config.ServiceName
	config.Attributes["service.version"] = config.ServiceVersion
	config.Attributes["deployment.environment"] = config.Environment

	l.config = config
	globalTracingConfig = config
	return nil
}

// GetConfig returns the loaded configuration
func (l *TracingConfigLoader) GetConfig() *TracingConfig {
	return l.config
}

// GetGlobalTracingConfig provides access to the globally loaded tracing configuration
func GetGlobalTracingConfig() *TracingConfig {
	return globalTracingConfig
}

// InitGlobalTracingConfig initializes global tracing configuration
func InitGlobalTracingConfig(configPath string) error {
	loader := NewTracingConfigLoader(configPath)

	// Try to load from file first
	if err := loader.LoadConfig(); err != nil {
		// If file loading fails, try environment variables
		if err := loader.LoadConfigFromEnv(); err != nil {
			return fmt.Errorf("failed to load tracing config: %w", err)
		}
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// TracingInitializer handles tracing initialization
type TracingInitializer struct {
	manager *TracingManager
	config  *TracingConfig
}

// NewTracingInitializer creates a new tracing initializer
func NewTracingInitializer(config *TracingConfig) (*TracingInitializer, error) {
	manager, err := NewTracingManager(config)
	if err != nil {
		return nil, err
	}

	return &TracingInitializer{
		manager: manager,
		config:  config,
	}, nil
}

// InitializeTracing initializes tracing with the given configuration
func InitializeTracing(config *TracingConfig) error {
	initializer, err := NewTracingInitializer(config)
	if err != nil {
		return err
	}

	// Set global manager
	globalTracingManager = initializer.manager

	return nil
}

// InitializeTracingFromConfig initializes tracing from configuration file
func InitializeTracingFromConfig(configPath string) error {
	// Load configuration
	if err := InitGlobalTracingConfig(configPath); err != nil {
		return err
	}

	// Initialize tracing
	return InitializeTracing(GetGlobalTracingConfig())
}

// InitializeTracingFromEnv initializes tracing from environment variables
func InitializeTracingFromEnv() error {
	loader := NewTracingConfigLoader("")
	if err := loader.LoadConfigFromEnv(); err != nil {
		return err
	}

	return InitializeTracing(loader.GetConfig())
}

// GetGlobalTracingManager returns the global tracing manager
func GetGlobalTracingManager() *TracingManager {
	return globalTracingManager
}

// ShutdownTracing gracefully shuts down tracing
func ShutdownTracing(ctx context.Context) error {
	if globalTracingManager != nil {
		return globalTracingManager.Shutdown(ctx)
	}
	return nil
}

// TracingConfig represents tracing configuration
type TracingConfig struct {
	// Service information
	ServiceName    string `yaml:"service_name" json:"service_name"`
	ServiceVersion string `yaml:"service_version" json:"service_version"`
	Environment    string `yaml:"environment" json:"environment"`

	// Exporter configuration
	Exporter       string `yaml:"exporter" json:"exporter"`
	JaegerEndpoint string `yaml:"jaeger_endpoint" json:"jaeger_endpoint"`
	ZipkinEndpoint string `yaml:"zipkin_endpoint" json:"zipkin_endpoint"`

	// Sampling configuration
	SamplingRatio float64 `yaml:"sampling_ratio" json:"sampling_ratio"`

	// Additional attributes
	Attributes map[string]string `yaml:"attributes" json:"attributes"`

	// Enable/disable tracing
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// TracingMiddlewareConfig holds configuration for tracing middleware
type TracingMiddlewareConfig struct {
	// Whether to trace HTTP requests
	TraceHTTP bool `yaml:"trace_http" json:"trace_http"`

	// Whether to trace database operations
	TraceDB bool `yaml:"trace_db" json:"trace_db"`

	// Whether to trace external API calls
	TraceAPI bool `yaml:"trace_api" json:"trace_api"`

	// Whether to include request/response bodies
	IncludeBodies bool `yaml:"include_bodies" json:"include_bodies"`

	// Maximum body size to include (in bytes)
	MaxBodySize int `yaml:"max_body_size" json:"max_body_size"`

	// Sensitive headers to exclude from tracing
	SensitiveHeaders []string `yaml:"sensitive_headers" json:"sensitive_headers"`

	// Sensitive query parameters to exclude from tracing
	SensitiveQueryParams []string `yaml:"sensitive_query_params" json:"sensitive_query_params"`
}

// DefaultTracingMiddlewareConfig returns default middleware configuration
func DefaultTracingMiddlewareConfig() *TracingMiddlewareConfig {
	return &TracingMiddlewareConfig{
		TraceHTTP:            true,
		TraceDB:              true,
		TraceAPI:             true,
		IncludeBodies:        false,
		MaxBodySize:          1024,
		SensitiveHeaders:     []string{"Authorization", "Cookie", "X-API-Key"},
		SensitiveQueryParams: []string{"password", "token", "secret"},
	}
}
