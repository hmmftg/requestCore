package libRequest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// HeaderConfigLoader handles loading and managing dynamic header configurations
type HeaderConfigLoader struct {
	configPath string
	config     *DynamicHeaderConfig
}

// NewHeaderConfigLoader creates a new header configuration loader
func NewHeaderConfigLoader(configPath string) *HeaderConfigLoader {
	return &HeaderConfigLoader{
		configPath: configPath,
	}
}

// LoadConfig loads the header configuration from the specified file
func (h *HeaderConfigLoader) LoadConfig() error {
	if h.configPath == "" {
		// Use default configuration
		h.config = h.getDefaultConfig()
		return nil
	}

	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", h.configPath, err)
	}

	var config DynamicHeaderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", h.configPath, err)
	}

	h.config = &config
	return nil
}

// GetConfig returns the loaded configuration
func (h *HeaderConfigLoader) GetConfig() *DynamicHeaderConfig {
	if h.config == nil {
		h.config = h.getDefaultConfig()
	}
	return h.config
}

// LoadConfigFromEnv loads configuration from environment variables
func (h *HeaderConfigLoader) LoadConfigFromEnv() error {
	configPath := os.Getenv("HEADER_CONFIG_PATH")
	if configPath == "" {
		// Try default locations
		defaultPaths := []string{
			"config/dynamic_headers.yaml",
			"dynamic_headers.yaml",
			"./config/dynamic_headers.yaml",
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}

	if configPath != "" {
		h.configPath = configPath
		return h.LoadConfig()
	}

	// Use default configuration if no file found
	// This allows the library to work without requiring a config file
	h.config = h.getDefaultConfig()
	return nil
}

// getDefaultConfig returns a default configuration
func (h *HeaderConfigLoader) getDefaultConfig() *DynamicHeaderConfig {
	return &DynamicHeaderConfig{
		RequiredHeaders: map[string]HeaderConfig{
			"Request-Id": {
				HeaderName:  "Request-Id",
				Required:    true,
				MinLength:   10,
				MaxLength:   64,
				Description: "Unique request identifier",
			},
			"User-Id": {
				HeaderName:  "User-Id",
				Required:    true,
				MinLength:   1,
				MaxLength:   50,
				Description: "User identifier",
			},
		},
		OptionalHeaders: map[string]HeaderConfig{
			"Branch-Id": {
				HeaderName:   "Branch-Id",
				Required:     false,
				MinLength:    1,
				MaxLength:    20,
				DefaultValue: "",
				Description:  "Branch identifier",
			},
			"Bank-Id": {
				HeaderName:   "Bank-Id",
				Required:     false,
				MinLength:    1,
				MaxLength:    20,
				DefaultValue: "",
				Description:  "Bank identifier",
			},
			"Person-Id": {
				HeaderName:   "Person-Id",
				Required:     false,
				MinLength:    1,
				MaxLength:    20,
				DefaultValue: "",
				Description:  "Person identifier",
			},
		},
		CustomHeaders: map[string]HeaderConfig{
			"Tenant-Id": {
				HeaderName:   "Tenant-Id",
				Required:     false,
				MinLength:    1,
				MaxLength:    50,
				DefaultValue: "default",
				Description:  "Multi-tenant identifier",
			},
			"Environment": {
				HeaderName:   "Environment",
				Required:     false,
				MinLength:    1,
				MaxLength:    20,
				DefaultValue: "production",
				Description:  "Environment identifier",
			},
		},
	}
}

// ValidateConfig validates the loaded configuration
func (h *HeaderConfigLoader) ValidateConfig() error {
	if h.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Validate required headers
	for key, config := range h.config.RequiredHeaders {
		if err := h.validateHeaderConfig(key, config); err != nil {
			return fmt.Errorf("invalid required header config for %s: %w", key, err)
		}
	}

	// Validate optional headers
	for key, config := range h.config.OptionalHeaders {
		if err := h.validateHeaderConfig(key, config); err != nil {
			return fmt.Errorf("invalid optional header config for %s: %w", key, err)
		}
	}

	// Validate custom headers
	for key, config := range h.config.CustomHeaders {
		if err := h.validateHeaderConfig(key, config); err != nil {
			return fmt.Errorf("invalid custom header config for %s: %w", key, err)
		}
	}

	return nil
}

// validateHeaderConfig validates a single header configuration
func (h *HeaderConfigLoader) validateHeaderConfig(key string, config HeaderConfig) error {
	if config.HeaderName == "" {
		return fmt.Errorf("header name cannot be empty")
	}
	if config.MinLength < 0 {
		return fmt.Errorf("min length cannot be negative")
	}
	if config.MaxLength < 0 {
		return fmt.Errorf("max length cannot be negative")
	}
	if config.MinLength > config.MaxLength {
		return fmt.Errorf("min length cannot be greater than max length")
	}
	return nil
}

// GetHeaderNames returns all configured header names
func (h *HeaderConfigLoader) GetHeaderNames() []string {
	if h.config == nil {
		return []string{}
	}

	var names []string

	// Add required headers
	for _, config := range h.config.RequiredHeaders {
		names = append(names, config.HeaderName)
	}

	// Add optional headers
	for _, config := range h.config.OptionalHeaders {
		names = append(names, config.HeaderName)
	}

	// Add custom headers
	for _, config := range h.config.CustomHeaders {
		names = append(names, config.HeaderName)
	}

	return names
}

// IsHeaderRequired checks if a header is required
func (h *HeaderConfigLoader) IsHeaderRequired(headerName string) bool {
	if h.config == nil {
		return false
	}

	for _, config := range h.config.RequiredHeaders {
		if config.HeaderName == headerName {
			return config.Required
		}
	}

	return false
}

// GetHeaderConfig returns the configuration for a specific header
func (h *HeaderConfigLoader) GetHeaderConfig(headerName string) (HeaderConfig, bool) {
	if h.config == nil {
		return HeaderConfig{}, false
	}

	// Check required headers
	for _, config := range h.config.RequiredHeaders {
		if config.HeaderName == headerName {
			return config, true
		}
	}

	// Check optional headers
	for _, config := range h.config.OptionalHeaders {
		if config.HeaderName == headerName {
			return config, true
		}
	}

	// Check custom headers
	for _, config := range h.config.CustomHeaders {
		if config.HeaderName == headerName {
			return config, true
		}
	}

	return HeaderConfig{}, false
}

// Global header configuration loader instance
var globalHeaderLoader *HeaderConfigLoader

// InitGlobalHeaderConfig initializes the global header configuration
func InitGlobalHeaderConfig(configPath string) error {
	globalHeaderLoader = NewHeaderConfigLoader(configPath)
	return globalHeaderLoader.LoadConfigFromEnv()
}

// GetGlobalHeaderConfig returns the global header configuration
func GetGlobalHeaderConfig() *DynamicHeaderConfig {
	if globalHeaderLoader == nil {
		globalHeaderLoader = NewHeaderConfigLoader("")
		globalHeaderLoader.LoadConfigFromEnv()
	}
	return globalHeaderLoader.GetConfig()
}

// CreateDynamicRequestHeader creates a new dynamic request header with global configuration
func CreateDynamicRequestHeader() *DynamicRequestHeader {
	config := GetGlobalHeaderConfig()
	return NewDynamicRequestHeader(config)
}
