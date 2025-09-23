package libRequest

import (
	"fmt"
	"reflect"
)

// HeaderMigration provides utilities for migrating from fixed headers to dynamic headers
type HeaderMigration struct {
	legacyMapping map[string]string
}

// NewHeaderMigration creates a new header migration utility
func NewHeaderMigration() *HeaderMigration {
	return &HeaderMigration{
		legacyMapping: map[string]string{
			"branch-id":   "Branch-Id",
			"bank-name":   "Bank-Id",
			"person-id":   "Person-Id",
			"department":  "Department-Id",
			"region":      "Region-Id",
			"tenant":      "Tenant-Id",
			"environment": "Environment",
		},
	}
}

// MigrateFromLegacyHeader migrates a legacy RequestHeader to DynamicRequestHeader
func (m *HeaderMigration) MigrateFromLegacyHeader(legacy *RequestHeader) *DynamicRequestHeader {
	dynamic := CreateDynamicRequestHeader()

	// Copy core fields
	dynamic.RequestId = legacy.RequestId
	dynamic.Program = legacy.Program
	dynamic.Module = legacy.Module
	dynamic.Method = legacy.Method
	dynamic.User = legacy.User

	// Migrate legacy fields to dynamic headers
	if legacy.Branch != "" {
		dynamic.SetDynamicHeader("Branch-Id", legacy.Branch)
	}
	if legacy.Bank != "" {
		dynamic.SetDynamicHeader("Bank-Id", legacy.Bank)
	}
	if legacy.Person != "" {
		dynamic.SetDynamicHeader("Person-Id", legacy.Person)
	}

	return dynamic
}

// MigrateToLegacyHeader migrates a DynamicRequestHeader to legacy RequestHeader
func (m *HeaderMigration) MigrateToLegacyHeader(dynamic *DynamicRequestHeader) *RequestHeader {
	legacy := &RequestHeader{
		RequestId: dynamic.RequestId,
		Program:   dynamic.Program,
		Module:    dynamic.Module,
		Method:    dynamic.Method,
		User:      dynamic.User,
	}

	// Migrate dynamic headers to legacy fields
	if branch := dynamic.GetDynamicHeader("Branch-Id"); branch != "" {
		legacy.Branch = branch
	}
	if bank := dynamic.GetDynamicHeader("Bank-Id"); bank != "" {
		legacy.Bank = bank
	}
	if person := dynamic.GetDynamicHeader("Person-Id"); person != "" {
		legacy.Person = person
	}

	return legacy
}

// ConvertLegacyHeaderName converts legacy header names to new format
func (m *HeaderMigration) ConvertLegacyHeaderName(legacyName string) string {
	if newName, exists := m.legacyMapping[legacyName]; exists {
		return newName
	}
	return legacyName
}

// AddLegacyMapping adds a new legacy mapping
func (m *HeaderMigration) AddLegacyMapping(legacyName, newName string) {
	m.legacyMapping[legacyName] = newName
}

// GetLegacyMappings returns all legacy mappings
func (m *HeaderMigration) GetLegacyMappings() map[string]string {
	return m.legacyMapping
}

// ValidateMigration validates that a migration was successful
func (m *HeaderMigration) ValidateMigration(legacy *RequestHeader, dynamic *DynamicRequestHeader) error {
	// Check core fields
	if legacy.RequestId != dynamic.RequestId {
		return fmt.Errorf("RequestId mismatch: %s != %s", legacy.RequestId, dynamic.RequestId)
	}
	if legacy.Program != dynamic.Program {
		return fmt.Errorf("Program mismatch: %s != %s", legacy.Program, dynamic.Program)
	}
	if legacy.Module != dynamic.Module {
		return fmt.Errorf("Module mismatch: %s != %s", legacy.Module, dynamic.Module)
	}
	if legacy.Method != dynamic.Method {
		return fmt.Errorf("Method mismatch: %s != %s", legacy.Method, dynamic.Method)
	}
	if legacy.User != dynamic.User {
		return fmt.Errorf("User mismatch: %s != %s", legacy.User, dynamic.User)
	}

	// Check dynamic headers
	if legacy.Branch != dynamic.GetDynamicHeader("Branch-Id") {
		return fmt.Errorf("Branch mismatch: %s != %s", legacy.Branch, dynamic.GetDynamicHeader("Branch-Id"))
	}
	if legacy.Bank != dynamic.GetDynamicHeader("Bank-Id") {
		return fmt.Errorf("Bank mismatch: %s != %s", legacy.Bank, dynamic.GetDynamicHeader("Bank-Id"))
	}
	if legacy.Person != dynamic.GetDynamicHeader("Person-Id") {
		return fmt.Errorf("Person mismatch: %s != %s", legacy.Person, dynamic.GetDynamicHeader("Person-Id"))
	}

	return nil
}

// CreateDynamicHeaderFromMap creates a DynamicRequestHeader from a map of headers
func (m *HeaderMigration) CreateDynamicHeaderFromMap(headers map[string]string) *DynamicRequestHeader {
	dynamic := CreateDynamicRequestHeader()

	// Set core headers
	if requestId, exists := headers["Request-Id"]; exists {
		dynamic.RequestId = requestId
	}
	if program, exists := headers["Program-Id"]; exists {
		dynamic.Program = program
	}
	if module, exists := headers["Module-Id"]; exists {
		dynamic.Module = module
	}
	if method, exists := headers["Method-Id"]; exists {
		dynamic.Method = method
	}
	if user, exists := headers["User-Id"]; exists {
		dynamic.User = user
	}

	// Set dynamic headers
	for key, value := range headers {
		// Skip core headers
		if key == "Request-Id" || key == "Program-Id" || key == "Module-Id" || key == "Method-Id" || key == "User-Id" {
			continue
		}

		// Convert legacy names if needed
		newKey := m.ConvertLegacyHeaderName(key)
		dynamic.SetDynamicHeader(newKey, value)
	}

	return dynamic
}

// GetHeaderDifferences compares two header structures and returns differences
func (m *HeaderMigration) GetHeaderDifferences(header1, header2 interface{}) []string {
	var differences []string

	// Use reflection to compare fields
	v1 := reflect.ValueOf(header1)
	v2 := reflect.ValueOf(header2)

	if v1.Kind() == reflect.Ptr {
		v1 = v1.Elem()
	}
	if v2.Kind() == reflect.Ptr {
		v2 = v2.Elem()
	}

	if v1.Type() != v2.Type() {
		differences = append(differences, fmt.Sprintf("Type mismatch: %s != %s", v1.Type(), v2.Type()))
		return differences
	}

	// Compare fields
	for i := 0; i < v1.NumField(); i++ {
		field1 := v1.Field(i)
		field2 := v2.Field(i)
		fieldName := v1.Type().Field(i).Name

		if !reflect.DeepEqual(field1.Interface(), field2.Interface()) {
			differences = append(differences, fmt.Sprintf("Field %s: %v != %v", fieldName, field1.Interface(), field2.Interface()))
		}
	}

	return differences
}

// Global migration instance
var globalMigration *HeaderMigration

// GetGlobalMigration returns the global migration instance
func GetGlobalMigration() *HeaderMigration {
	if globalMigration == nil {
		globalMigration = NewHeaderMigration()
	}
	return globalMigration
}
