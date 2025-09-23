package libRequest

import (
	"testing"
)

func TestDynamicRequestHeader(t *testing.T) {
	// Create a dynamic header with default configuration
	header := CreateDynamicRequestHeader()

	// Set core headers
	header.RequestId = "req-12345"
	header.User = "user123"
	header.Program = "testapp"
	header.Module = "auth"
	header.Method = "login"

	// Set dynamic headers
	header.SetDynamicHeader("Branch-Id", "BR001")
	header.SetDynamicHeader("Bank-Id", "BANK001")
	header.SetDynamicHeader("Tenant-Id", "tenant-abc")

	// Test getting dynamic headers
	if branch := header.GetDynamicHeader("Branch-Id"); branch != "BR001" {
		t.Errorf("Expected Branch-Id to be 'BR001', got '%s'", branch)
	}

	if bank := header.GetDynamicHeader("Bank-Id"); bank != "BANK001" {
		t.Errorf("Expected Bank-Id to be 'BANK001', got '%s'", bank)
	}

	// Test compatibility methods
	if header.GetBranch() != "BR001" {
		t.Errorf("Expected GetBranch() to return 'BR001', got '%s'", header.GetBranch())
	}

	if header.GetBank() != "BANK001" {
		t.Errorf("Expected GetBank() to return 'BANK001', got '%s'", header.GetBank())
	}

	// Test header map
	headerMap := header.GetHeaderMap()
	if headerMap["Branch-Id"] != "BR001" {
		t.Errorf("Expected header map to contain Branch-Id='BR001'")
	}

	if headerMap["Request-Id"] != "req-12345" {
		t.Errorf("Expected header map to contain Request-Id='req-12345'")
	}
}

func TestHeaderMigration(t *testing.T) {
	// Create legacy header
	legacyHeader := &RequestHeader{
		RequestId: "req-12345",
		User:      "user123",
		Program:   "testapp",
		Module:    "auth",
		Method:    "login",
		Branch:    "BR001",
		Bank:      "BANK001",
		Person:    "PERSON001",
	}

	// Migrate to dynamic header
	migration := GetGlobalMigration()
	dynamicHeader := migration.MigrateFromLegacyHeader(legacyHeader)

	// Validate migration
	if err := migration.ValidateMigration(legacyHeader, dynamicHeader); err != nil {
		t.Errorf("Migration validation failed: %v", err)
	}

	// Test that dynamic headers were set correctly
	if dynamicHeader.GetDynamicHeader("Branch-Id") != "BR001" {
		t.Errorf("Expected Branch-Id to be 'BR001' after migration")
	}

	if dynamicHeader.GetDynamicHeader("Bank-Id") != "BANK001" {
		t.Errorf("Expected Bank-Id to be 'BANK001' after migration")
	}

	if dynamicHeader.GetDynamicHeader("Person-Id") != "PERSON001" {
		t.Errorf("Expected Person-Id to be 'PERSON001' after migration")
	}
}

func TestHeaderConfigLoader(t *testing.T) {
	// Test default configuration
	loader := NewHeaderConfigLoader("")
	if err := loader.LoadConfigFromEnv(); err != nil {
		t.Errorf("Failed to load default config: %v", err)
	}

	config := loader.GetConfig()
	if config == nil {
		t.Error("Expected config to be loaded")
	}

	// Test validation
	if err := loader.ValidateConfig(); err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// Test header names
	headerNames := loader.GetHeaderNames()
	if len(headerNames) == 0 {
		t.Error("Expected at least some header names")
	}

	// Test required header check
	if !loader.IsHeaderRequired("Request-Id") {
		t.Error("Expected Request-Id to be required")
	}

	// Test header config retrieval
	if config, exists := loader.GetHeaderConfig("Request-Id"); !exists {
		t.Error("Expected Request-Id config to exist")
	} else if config.HeaderName != "Request-Id" {
		t.Errorf("Expected header name to be 'Request-Id', got '%s'", config.HeaderName)
	}
}
