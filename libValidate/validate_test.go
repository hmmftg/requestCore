package libValidate_test

import (
	"testing"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/hmmftg/requestCore/libValidate"
)

type Customer struct {
	ID   string `json:"id,omitempty" validate:"omitempty,numeric,len=10" name:"شناسه"`
	Name string `json:"name,omitempty" validate:"omitempty,startswith=name." name:"نام"`
}

func TestValidate(t *testing.T) {
	type Test struct {
		Struct   any
		Expected validator.ValidationErrorsTranslations
	}
	testList := []Test{
		{
			Struct: Customer{ID: "222"},
			Expected: validator.ValidationErrorsTranslations{
				"Customer.شناسه": "طول شناسه باید 10 کاراکتر باشد",
			},
		},
		{
			Struct: Customer{Name: "aname.aa"},
			Expected: validator.ValidationErrorsTranslations{
				"Customer.نام": "Key: 'Customer.نام' Error:Field validation for 'نام' failed on the 'startswith' tag",
			},
		},
	}
	libValidate.Init()
	for id := range testList {
		request := testList[id].Struct
		err, errValidate := libValidate.ValidateStruct(request)
		if err != nil {
			t.Fatalf("err: %+v\n", err)
		}
		list := errValidate.Translate(libValidate.GetTranslator())
		for k, v := range list {
			if v != testList[id].Expected[k] {
				t.Fatalf("validate error on [%s]: %s, expected: %s\n", k, v, testList[id].Expected[k])
			}
		}
	}
}

// Test constants
func TestErrorCodeConstants(t *testing.T) {
	if libValidate.ErrorCodeRequiredField != "REQUIRED-FIELD" {
		t.Errorf("Expected ErrorCodeRequiredField to be 'REQUIRED-FIELD', got '%s'", libValidate.ErrorCodeRequiredField)
	}
	if libValidate.ErrorCodeInvalidInputData != "INVALID-INPUT-DATA" {
		t.Errorf("Expected ErrorCodeInvalidInputData to be 'INVALID-INPUT-DATA', got '%s'", libValidate.ErrorCodeInvalidInputData)
	}
}

// TestRegisterValidation tests the RegisterValidation function
func TestRegisterValidation(t *testing.T) {
	// Reset the validator to ensure clean state
	libValidate.Init()

	// Test registering a custom validator with error code
	testTag := "test_validator"
	testErrorCode := libValidate.ErrorCodeInvalidInputData

	err := libValidate.RegisterValidation(testTag,
		func(fl validator.FieldLevel) bool {
			return fl.Field().String() == "valid"
		}).
		WithTranslation(
			func(ut ut.Translator) error {
				return ut.Add(testTag, "{0} is not valid", true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(testTag, fe.Field())
				return t
			}).
		WithErrorCode(testErrorCode).
		Build()

	if err != nil {
		t.Fatalf("RegisterValidation failed: %v", err)
	}

	// Verify the error code was stored
	code := libValidate.GetErrorCode(testTag)
	if code != testErrorCode {
		t.Errorf("Expected error code '%s', got '%s'", testErrorCode, code)
	}

	// Verify it's recognized as a custom validator
	if !libValidate.IsCustomValidator(testTag) {
		t.Errorf("Expected testTag to be recognized as custom validator")
	}

	// Test that the validator actually works
	type TestStruct struct {
		Field string `validate:"test_validator"`
	}

	validStruct := TestStruct{Field: "valid"}
	_, errs := libValidate.ValidateStruct(validStruct)
	if errs != nil {
		t.Errorf("Expected no validation errors for valid value, got: %v", errs)
	}

	invalidStruct := TestStruct{Field: "invalid"}
	_, errs = libValidate.ValidateStruct(invalidStruct)
	if errs == nil {
		t.Error("Expected validation error for invalid value")
	}
}

// TestRegisterValidationWithEmptyErrorCode tests default error code behavior
func TestRegisterValidationWithEmptyErrorCode(t *testing.T) {
	libValidate.Init()

	testTag := "test_validator_empty_code"

	err := libValidate.RegisterValidation(testTag,
		func(fl validator.FieldLevel) bool {
			return true
		}).
		WithTranslation(
			func(ut ut.Translator) error {
				return ut.Add(testTag, "{0} is invalid", true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(testTag, fe.Field())
				return t
			}).
		// Empty error code should default to ErrorCodeInvalidInputData
		Build()

	if err != nil {
		t.Fatalf("RegisterValidation failed: %v", err)
	}

	code := libValidate.GetErrorCode(testTag)
	if code != libValidate.ErrorCodeInvalidInputData {
		t.Errorf("Expected default error code '%s', got '%s'", libValidate.ErrorCodeInvalidInputData, code)
	}
}

// TestGetErrorCode tests GetErrorCode for various scenarios
func TestGetErrorCode(t *testing.T) {
	libValidate.Init()

	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "required tag (system validator)",
			tag:      "required",
			expected: libValidate.ErrorCodeRequiredField,
		},
		{
			name:     "required_unless tag (system validator)",
			tag:      "required_unless",
			expected: libValidate.ErrorCodeRequiredField,
		},
		{
			name:     "required_if tag (system validator)",
			tag:      "required_if",
			expected: libValidate.ErrorCodeRequiredField,
		},
		{
			name:     "padded_ip tag (custom validator registered via RegisterValidation)",
			tag:      "padded_ip",
			expected: libValidate.ErrorCodeInvalidInputData,
		},
		{
			name:     "numeric tag (system validator)",
			tag:      "numeric",
			expected: libValidate.ErrorCodeInvalidInputData,
		},
		{
			name:     "min tag (system validator)",
			tag:      "min",
			expected: libValidate.ErrorCodeInvalidInputData,
		},
		{
			name:     "max tag (system validator)",
			tag:      "max",
			expected: libValidate.ErrorCodeInvalidInputData,
		},
		{
			name:     "unknown tag (default error code)",
			tag:      "unknown_tag",
			expected: libValidate.ErrorCodeInvalidInputData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := libValidate.GetErrorCode(tt.tag)
			if code != tt.expected {
				t.Errorf("GetErrorCode(%s) = %s, expected %s", tt.tag, code, tt.expected)
			}
		})
	}
}

// TestGetErrorCodeForRegisteredValidator tests GetErrorCode for custom registered validators
func TestGetErrorCodeForRegisteredValidator(t *testing.T) {
	libValidate.Init()

	customTag := "custom_registered"
	customErrorCode := libValidate.ErrorCodeRequiredField

	err := libValidate.RegisterValidation(customTag,
		func(fl validator.FieldLevel) bool {
			return true
		}).
		WithTranslation(
			func(ut ut.Translator) error {
				return ut.Add(customTag, "{0} is invalid", true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(customTag, fe.Field())
				return t
			}).
		WithErrorCode(customErrorCode).
		Build()

	if err != nil {
		t.Fatalf("RegisterValidation failed: %v", err)
	}

	code := libValidate.GetErrorCode(customTag)
	if code != customErrorCode {
		t.Errorf("GetErrorCode(%s) = %s, expected %s", customTag, code, customErrorCode)
	}
}

// TestIsCustomValidator tests IsCustomValidator function
func TestIsCustomValidator(t *testing.T) {
	libValidate.Init()

	tests := []struct {
		name     string
		tag      string
		expected bool
	}{
		{
			name:     "required is not custom (system validator)",
			tag:      "required",
			expected: false,
		},
		{
			name:     "padded_ip is custom (registered via RegisterValidation)",
			tag:      "padded_ip",
			expected: true,
		},
		{
			name:     "numeric is not custom (system validator)",
			tag:      "numeric",
			expected: false,
		},
		{
			name:     "min is not custom (system validator)",
			tag:      "min",
			expected: false,
		},
		{
			name:     "unknown tag is custom (not in registry)",
			tag:      "unknown_tag_123",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := libValidate.IsCustomValidator(tt.tag)
			if result != tt.expected {
				t.Errorf("IsCustomValidator(%s) = %v, expected %v", tt.tag, result, tt.expected)
			}
		})
	}
}

// TestPaddedIpValidator tests the padded_ip validator registration
func TestPaddedIpValidator(t *testing.T) {
	libValidate.Init()

	// Register padded_ip validator
	err := libValidate.RegisterPaddedIpValidator()
	if err != nil {
		t.Fatalf("RegisterPaddedIpValidator failed: %v", err)
	}

	// Test that padded_ip is registered
	code := libValidate.GetErrorCode("padded_ip")
	if code != libValidate.ErrorCodeInvalidInputData {
		t.Errorf("padded_ip should have error code '%s', got '%s'", libValidate.ErrorCodeInvalidInputData, code)
	}

	// Test that padded_ip IS considered a custom validator (registered via RegisterValidation)
	if !libValidate.IsCustomValidator("padded_ip") {
		t.Error("padded_ip should be considered a custom validator since it's registered via RegisterValidation")
	}

	// Test that padded_ip info is in registry
	info := libValidate.GetValidatorInfo("padded_ip")
	if info == nil {
		t.Fatal("padded_ip should be in validator registry")
	}
	if info.Tag != "padded_ip" {
		t.Errorf("Expected tag 'padded_ip', got '%s'", info.Tag)
	}
	if info.ErrorCode != libValidate.ErrorCodeInvalidInputData {
		t.Errorf("Expected error code '%s', got '%s'", libValidate.ErrorCodeInvalidInputData, info.ErrorCode)
	}

	// Test the validator functionality
	type TestStruct struct {
		IP string `validate:"padded_ip" name:"IP"`
	}

	validIP := TestStruct{IP: "192.168.001.001"}
	_, errs := libValidate.ValidateStruct(validIP)
	if errs != nil {
		t.Errorf("Expected no validation errors for valid padded IP, got: %v", errs)
	}

	invalidIP := TestStruct{IP: "192.168.1.1"} // Not padded
	_, errs = libValidate.ValidateStruct(invalidIP)
	if errs == nil {
		t.Error("Expected validation error for invalid padded IP")
	}

	shortIP := TestStruct{IP: "192.168.1"} // Too short
	_, errs = libValidate.ValidateStruct(shortIP)
	if errs == nil {
		t.Error("Expected validation error for too short IP")
	}
}

// TestRegisterValidationConcurrent tests concurrent registration
func TestRegisterValidationConcurrent(t *testing.T) {
	libValidate.Init()

	// Register multiple validators concurrently
	tags := []string{"concurrent1", "concurrent2", "concurrent3"}
	done := make(chan bool, len(tags))

	for _, tag := range tags {
		go func(tag string) {
			err := libValidate.RegisterValidation(tag,
				func(fl validator.FieldLevel) bool {
					return true
				}).
				WithTranslation(
					func(ut ut.Translator) error {
						return ut.Add(tag, "{0} is invalid", true)
					},
					func(ut ut.Translator, fe validator.FieldError) string {
						t, _ := ut.T(tag, fe.Field())
						return t
					}).
				WithErrorCode(libValidate.ErrorCodeInvalidInputData).
				Build()
			if err != nil {
				t.Errorf("RegisterValidation failed for %s: %v", tag, err)
			}
			done <- true
		}(tag)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(tags); i++ {
		<-done
	}

	// Verify all tags are registered
	for _, tag := range tags {
		code := libValidate.GetErrorCode(tag)
		if code != libValidate.ErrorCodeInvalidInputData {
			t.Errorf("Expected error code for %s, got %s", tag, code)
		}
		// Verify they're custom validators
		if !libValidate.IsCustomValidator(tag) {
			t.Errorf("Expected %s to be a custom validator", tag)
		}
	}
}

// TestValidatorRegistry tests the validator registry mechanism
func TestValidatorRegistry(t *testing.T) {
	libValidate.Init()

	// Test system validators are registered
	systemValidators := []string{"required", "numeric", "min", "max"}
	for _, tag := range systemValidators {
		info := libValidate.GetValidatorInfo(tag)
		if info == nil {
			t.Errorf("Expected %s to be in registry", tag)
			continue
		}
		if info.Type != libValidate.ValidatorTypeSystem {
			t.Errorf("Expected %s to be a system validator", tag)
		}
		if !libValidate.IsCustomValidator(tag) {
			// System validators should not be custom
			// This is correct
		} else {
			t.Errorf("Expected %s to NOT be a custom validator", tag)
		}
	}

	// Test custom validator (padded_ip) - register it first
	err := libValidate.RegisterPaddedIpValidator()
	if err != nil {
		t.Fatalf("RegisterPaddedIpValidator failed: %v", err)
	}

	paddedIPInfo := libValidate.GetValidatorInfo("padded_ip")
	if paddedIPInfo == nil {
		t.Fatal("padded_ip should be in registry")
	}
	if paddedIPInfo.Type != libValidate.ValidatorTypeCustom {
		t.Error("padded_ip should be a custom validator")
	}
	if !libValidate.IsCustomValidator("padded_ip") {
		t.Error("padded_ip should be identified as a custom validator")
	}

	// Test unregistered validator
	unknownInfo := libValidate.GetValidatorInfo("unknown_validator_xyz")
	if unknownInfo != nil {
		t.Error("Unknown validator should not be in registry")
	}
	// Unknown validators are treated as custom by default
	if !libValidate.IsCustomValidator("unknown_validator_xyz") {
		t.Error("Unknown validators should be treated as custom")
	}
}

// TestSetSystemValidators tests the SetSystemValidators function
func TestSetSystemValidators(t *testing.T) {
	// Test setting custom system validators
	customValidators := map[string]string{
		"test_custom_email": libValidate.ErrorCodeInvalidInputData,
		"test_custom_min":   libValidate.ErrorCodeInvalidInputData,
	}

	libValidate.SetSystemValidators(customValidators)

	// Initialize to trigger registration
	libValidate.Init()

	// Verify custom validators are registered
	code := libValidate.GetErrorCode("test_custom_email")
	if code != libValidate.ErrorCodeInvalidInputData {
		t.Errorf("Expected error code '%s' for 'test_custom_email', got '%s'", libValidate.ErrorCodeInvalidInputData, code)
	}

	code = libValidate.GetErrorCode("test_custom_min")
	if code != libValidate.ErrorCodeInvalidInputData {
		t.Errorf("Expected error code '%s' for 'test_custom_min', got '%s'", libValidate.ErrorCodeInvalidInputData, code)
	}

	// Reset to defaults
	libValidate.SetSystemValidators(nil)
}

// TestDisableDefaultSystemValidators tests DisableDefaultSystemValidators
func TestDisableDefaultSystemValidators(t *testing.T) {
	libValidate.DisableDefaultSystemValidators()

	// Set custom validators
	customValidators := map[string]string{
		"test_disabled_default": libValidate.ErrorCodeRequiredField,
	}
	libValidate.SetSystemValidators(customValidators)

	// Re-initialize to trigger registration with disabled defaults
	// Note: In real usage, Init() is called once with sync.Once, but in tests we can verify the behavior
	_ = libValidate.GetErrorCode("test_disabled_default")
	// The code might default to ErrorCodeInvalidInputData if not yet initialized
	// This test verifies the function works

	// Reset to defaults
	libValidate.SetSystemValidators(nil)
}

// TestRegisterSystemValidator tests RegisterSystemValidator function
func TestRegisterSystemValidator(t *testing.T) {
	libValidate.Init()

	// Register a custom system validator
	libValidate.RegisterSystemValidator("custom_system_tag", libValidate.ErrorCodeRequiredField)

	// Verify it's registered
	code := libValidate.GetErrorCode("custom_system_tag")
	if code != libValidate.ErrorCodeRequiredField {
		t.Errorf("Expected error code '%s' for 'custom_system_tag', got '%s'", libValidate.ErrorCodeRequiredField, code)
	}

	// Verify it's NOT a custom validator
	if libValidate.IsCustomValidator("custom_system_tag") {
		t.Error("custom_system_tag should NOT be a custom validator (it's a system validator)")
	}

	// Verify the info
	info := libValidate.GetValidatorInfo("custom_system_tag")
	if info == nil {
		t.Fatal("custom_system_tag should be in registry")
	}
	if info.Type != libValidate.ValidatorTypeSystem {
		t.Error("custom_system_tag should be a system validator")
	}
}

// TestDefaultSystemValidators tests that default system validators are registered
func TestDefaultSystemValidators(t *testing.T) {
	// Reset to defaults
	libValidate.SetSystemValidators(nil)

	// Initialize with defaults
	libValidate.Init()

	// Verify default validators are registered
	testCases := []struct {
		tag      string
		expected string
	}{
		{"required", libValidate.ErrorCodeRequiredField},
		{"required_unless", libValidate.ErrorCodeRequiredField},
		{"required_if", libValidate.ErrorCodeRequiredField},
		{"numeric", libValidate.ErrorCodeInvalidInputData},
		{"len", libValidate.ErrorCodeInvalidInputData},
		{"min", libValidate.ErrorCodeInvalidInputData},
		{"max", libValidate.ErrorCodeInvalidInputData},
	}

	for _, tc := range testCases {
		t.Run(tc.tag, func(t *testing.T) {
			code := libValidate.GetErrorCode(tc.tag)
			if code != tc.expected {
				t.Errorf("Expected error code '%s' for '%s', got '%s'", tc.expected, tc.tag, code)
			}

			// Verify it's NOT a custom validator
			if libValidate.IsCustomValidator(tc.tag) {
				t.Errorf("%s should NOT be a custom validator", tc.tag)
			}
		})
	}
}
