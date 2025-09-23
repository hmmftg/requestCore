package libRequest

import (
	"fmt"
)

// DynamicHeaderConfig defines the configuration for dynamic headers
type DynamicHeaderConfig struct {
	// Required headers that must be present
	RequiredHeaders map[string]HeaderConfig `yaml:"required"`
	// Optional headers that can be present
	OptionalHeaders map[string]HeaderConfig `yaml:"optional"`
	// Custom headers that can be dynamically added
	CustomHeaders map[string]HeaderConfig `yaml:"custom"`
}

// HeaderConfig defines the configuration for a single header
type HeaderConfig struct {
	HeaderName     string `yaml:"headerName"`     // The actual HTTP header name
	Required       bool   `yaml:"required"`       // Whether this header is required
	MinLength      int    `yaml:"minLength"`      // Minimum length validation
	MaxLength      int    `yaml:"maxLength"`      // Maximum length validation
	DefaultValue   string `yaml:"defaultValue"`   // Default value if not provided
	ValidationRule string `yaml:"validationRule"` // Custom validation rule
	Description    string `yaml:"description"`    // Description for documentation
}

// DynamicRequestHeader represents a dynamic header structure
type DynamicRequestHeader struct {
	// Core headers that are always present
	RequestId string `header:"Request-Id" reqHeader:"Request-Id" validate:"required,min=10,max=64"`
	Program   string `header:"Program-Id" reqHeader:"Program-Id"`
	Module    string `header:"Module-Id"  reqHeader:"Module-Id"`
	Method    string `header:"Method-Id"  reqHeader:"Method-Id"`
	User      string `header:"User-Id"    reqHeader:"User-Id"`

	// Dynamic headers stored as key-value pairs
	DynamicHeaders map[string]string `json:"dynamicHeaders"`

	// Configuration for header validation
	Config *DynamicHeaderConfig `json:"-"`
}

// NewDynamicRequestHeader creates a new dynamic request header with configuration
func NewDynamicRequestHeader(config *DynamicHeaderConfig) *DynamicRequestHeader {
	return &DynamicRequestHeader{
		DynamicHeaders: make(map[string]string),
		Config:         config,
	}
}

// SetDynamicHeader sets a dynamic header value
func (r *DynamicRequestHeader) SetDynamicHeader(key, value string) {
	if r.DynamicHeaders == nil {
		r.DynamicHeaders = make(map[string]string)
	}
	r.DynamicHeaders[key] = value
}

// GetDynamicHeader gets a dynamic header value
func (r *DynamicRequestHeader) GetDynamicHeader(key string) string {
	if r.DynamicHeaders == nil {
		return ""
	}
	return r.DynamicHeaders[key]
}

// GetDynamicHeaderWithDefault gets a dynamic header value with default fallback
func (r *DynamicRequestHeader) GetDynamicHeaderWithDefault(key string) string {
	value := r.GetDynamicHeader(key)
	if value == "" && r.Config != nil {
		if config, exists := r.Config.RequiredHeaders[key]; exists {
			return config.DefaultValue
		}
		if config, exists := r.Config.OptionalHeaders[key]; exists {
			return config.DefaultValue
		}
		if config, exists := r.Config.CustomHeaders[key]; exists {
			return config.DefaultValue
		}
	}
	return value
}

// ValidateDynamicHeaders validates all dynamic headers against their configuration
func (r *DynamicRequestHeader) ValidateDynamicHeaders() error {
	if r.Config == nil {
		return nil // No validation if no config
	}

	// Validate required headers
	for key, config := range r.Config.RequiredHeaders {
		value := r.GetDynamicHeader(key)
		if config.Required && value == "" {
			return fmt.Errorf("required header %s is missing", key)
		}
		if err := r.validateHeaderValue(key, value, config); err != nil {
			return err
		}
	}

	// Validate optional headers
	for key, config := range r.Config.OptionalHeaders {
		value := r.GetDynamicHeader(key)
		if value != "" {
			if err := r.validateHeaderValue(key, value, config); err != nil {
				return err
			}
		}
	}

	// Validate custom headers
	for key, config := range r.Config.CustomHeaders {
		value := r.GetDynamicHeader(key)
		if value != "" {
			if err := r.validateHeaderValue(key, value, config); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateHeaderValue validates a single header value against its configuration
func (r *DynamicRequestHeader) validateHeaderValue(key, value string, config HeaderConfig) error {
	if config.MinLength > 0 && len(value) < config.MinLength {
		return fmt.Errorf("header %s value too short (minimum %d characters)", key, config.MinLength)
	}
	if config.MaxLength > 0 && len(value) > config.MaxLength {
		return fmt.Errorf("header %s value too long (maximum %d characters)", key, config.MaxLength)
	}
	// Add more validation rules as needed
	return nil
}

// GetHeaderMap returns all headers as a map for easy access
func (r *DynamicRequestHeader) GetHeaderMap() map[string]string {
	headers := map[string]string{
		"Request-Id": r.RequestId,
		"Program-Id": r.Program,
		"Module-Id":  r.Module,
		"Method-Id":  r.Method,
		"User-Id":    r.User,
	}

	// Add dynamic headers
	for key, value := range r.DynamicHeaders {
		headers[key] = value
	}

	return headers
}

// Dynamic header compatibility methods
func (r DynamicRequestHeader) GetId() string {
	return r.RequestId
}
func (r DynamicRequestHeader) GetUser() string {
	return r.User
}
func (r DynamicRequestHeader) GetBank() string {
	return r.GetDynamicHeaderWithDefault("Bank-Id")
}
func (r DynamicRequestHeader) GetBranch() string {
	return r.GetDynamicHeaderWithDefault("Branch-Id")
}
func (r DynamicRequestHeader) GetPerson() string {
	return r.GetDynamicHeaderWithDefault("Person-Id")
}
func (r DynamicRequestHeader) GetProgram() string {
	return r.Program
}
func (r DynamicRequestHeader) GetModule() string {
	return r.Module
}
func (r DynamicRequestHeader) GetMethod() string {
	return r.Method
}

func (r *DynamicRequestHeader) SetUser(user string) {
	r.User = user
}
func (r *DynamicRequestHeader) SetProgram(program string) {
	r.Program = program
}
func (r *DynamicRequestHeader) SetModule(module string) {
	r.Module = module
}
func (r *DynamicRequestHeader) SetMethod(method string) {
	r.Method = method
}
func (r *DynamicRequestHeader) SetBranch(branch string) {
	r.SetDynamicHeader("Branch-Id", branch)
}
func (r *DynamicRequestHeader) SetBank(bank string) {
	r.SetDynamicHeader("Bank-Id", bank)
}
func (r *DynamicRequestHeader) SetPerson(person string) {
	r.SetDynamicHeader("Person-Id", person)
}
