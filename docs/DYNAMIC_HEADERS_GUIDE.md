# Dynamic Headers Implementation

## Overview
This implementation replaces fixed headers like `branch-id`, `bank-name`, and `person-id` with a dynamic, configurable header system that allows for flexible header management without code changes.

## Key Features

### ✅ **Dynamic Configuration**
- Headers defined in YAML configuration files
- Environment-specific overrides
- Runtime header validation
- Backward compatibility with legacy headers

### ✅ **Flexible Header Management**
- Required, optional, and custom headers
- Default values and validation rules
- Easy addition of new headers without code changes
- Support for multi-tenant applications

### ✅ **Migration Support**
- Automatic migration from legacy fixed headers
- Validation utilities for migration
- Legacy mapping for backward compatibility

## Files Created/Modified

### New Files:
- `libRequest/dynamic_headers.go` - Dynamic header structures and methods
- `libRequest/header_config.go` - Configuration loading and management
- `libRequest/header_migration.go` - Migration utilities

### Modified Files:
- `testingtools/model.go` - Updated to use dynamic headers
- `libRequest/model.go` - Kept for backward compatibility

## Configuration Template

**Note**: Configuration is optional. The library works with sensible defaults if no configuration file is provided.

To customize headers, create a `dynamic_headers.yaml` file in your application's config directory:

```yaml
# Dynamic Headers Configuration Template
# This file defines the configuration for dynamic headers in the request system

# Example configuration for dynamic headers
dynamicHeaders:
  # Required headers that must be present in every request
  required:
    Request-Id:
      headerName: "Request-Id"
      required: true
      minLength: 10
      maxLength: 64
      description: "Unique request identifier"
    
    User-Id:
      headerName: "User-Id"
      required: true
      minLength: 1
      maxLength: 50
      description: "User identifier"
  
  # Optional headers that can be present
  optional:
    Branch-Id:
      headerName: "Branch-Id"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: ""
      description: "Branch identifier"
    
    Bank-Id:
      headerName: "Bank-Id"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: ""
      description: "Bank identifier"
    
    Person-Id:
      headerName: "Person-Id"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: ""
      description: "Person identifier"
    
    Department-Id:
      headerName: "Department-Id"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: ""
      description: "Department identifier"
    
    Region-Id:
      headerName: "Region-Id"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: ""
      description: "Region identifier"
  
  # Custom headers that can be dynamically added
  custom:
    Tenant-Id:
      headerName: "Tenant-Id"
      required: false
      minLength: 1
      maxLength: 50
      defaultValue: "default"
      description: "Multi-tenant identifier"
    
    Environment:
      headerName: "Environment"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: "production"
      description: "Environment identifier (dev, staging, prod)"
    
    Client-Version:
      headerName: "Client-Version"
      required: false
      minLength: 1
      maxLength: 20
      defaultValue: "1.0.0"
      description: "Client application version"
    
    Session-Id:
      headerName: "Session-Id"
      required: false
      minLength: 1
      maxLength: 100
      defaultValue: ""
      description: "Session identifier"

# Header mapping for backward compatibility
legacyMapping:
  # Maps old fixed headers to new dynamic system
  branch-id: "Branch-Id"
  bank-name: "Bank-Id"
  person-id: "Person-Id"
  department-id: "Department-Id"
  region-id: "Region-Id"

# Environment-specific overrides
environments:
  development:
    optional:
      Branch-Id:
        defaultValue: "DEV001"
      Bank-Id:
        defaultValue: "DEV_BANK"
      Environment:
        defaultValue: "development"
  
  staging:
    optional:
      Branch-Id:
        defaultValue: "STG001"
      Bank-Id:
        defaultValue: "STG_BANK"
      Environment:
        defaultValue: "staging"
  
  production:
    optional:
      Branch-Id:
        defaultValue: ""
      Bank-Id:
        defaultValue: ""
      Environment:
        defaultValue: "production"
```

## Default Behavior

If no configuration file is provided, the library uses these defaults:

### **Required Headers:**
- `Request-Id`: Unique request identifier (10-64 characters)
- `User-Id`: User identifier (1-50 characters)

### **Optional Headers:**
- `Branch-Id`: Branch identifier (1-20 characters, empty default)
- `Bank-Id`: Bank identifier (1-20 characters, empty default)  
- `Person-Id`: Person identifier (1-20 characters, empty default)

### **Custom Headers:**
- `Tenant-Id`: Multi-tenant identifier (1-50 characters, "default" default)
- `Environment`: Environment identifier (1-20 characters, "production" default)

## Usage Examples

### 1. **Basic Dynamic Header Usage**

```go
// Create a dynamic header with global configuration
header := libRequest.CreateDynamicRequestHeader()

// Set core headers
header.RequestId = "req-12345"
header.User = "user123"
header.Program = "myapp"
header.Module = "auth"
header.Method = "login"

// Set dynamic headers
header.SetDynamicHeader("Branch-Id", "BR001")
header.SetDynamicHeader("Bank-Id", "BANK001")
header.SetDynamicHeader("Tenant-Id", "tenant-abc")

// Get dynamic headers
branch := header.GetDynamicHeader("Branch-Id")
bank := header.GetDynamicHeaderWithDefault("Bank-Id") // Uses default if not set

// Validate headers
if err := header.ValidateDynamicHeaders(); err != nil {
    log.Printf("Header validation failed: %v", err)
}
```

### 2. **Configuration Loading**

```go
// Load configuration from file
loader := libRequest.NewHeaderConfigLoader("config/dynamic_headers.yaml")
if err := loader.LoadConfig(); err != nil {
    log.Fatal("Failed to load header config:", err)
}

// Or load from environment
if err := libRequest.InitGlobalHeaderConfig("config/dynamic_headers.yaml"); err != nil {
    log.Fatal("Failed to initialize global config:", err)
}

// Get global configuration
config := libRequest.GetGlobalHeaderConfig()
```

### 3. **Migration from Legacy Headers**

```go
// Legacy header
legacyHeader := &libRequest.RequestHeader{
    RequestId: "req-12345",
    User:      "user123",
    Branch:    "BR001",
    Bank:      "BANK001",
    Person:    "PERSON001",
}

// Migrate to dynamic header
migration := libRequest.GetGlobalMigration()
dynamicHeader := migration.MigrateFromLegacyHeader(legacyHeader)

// Validate migration
if err := migration.ValidateMigration(legacyHeader, dynamicHeader); err != nil {
    log.Printf("Migration validation failed: %v", err)
}
```

### 4. **Environment-Specific Configuration**

```yaml
# config/dynamic_headers.yaml
dynamicHeaders:
  optional:
    Branch-Id:
      headerName: "Branch-Id"
      required: false
      defaultValue: ""
      description: "Branch identifier"
    
    Bank-Id:
      headerName: "Bank-Id"
      required: false
      defaultValue: ""
      description: "Bank identifier"

environments:
  development:
    optional:
      Branch-Id:
        defaultValue: "DEV001"
      Bank-Id:
        defaultValue: "DEV_BANK"
  
  production:
    optional:
      Branch-Id:
        defaultValue: ""
      Bank-Id:
        defaultValue: ""
```

### 5. **Testing with Dynamic Headers**

```go
// In tests, headers are automatically configured
func TestAPI(t *testing.T) {
    // Headers are automatically set based on configuration
    testCase := testingtools.TestCase{
        Name: "Test API",
        Header: testingtools.Header{
            {Key: "Branch-Id", Value: "TEST001"},
            {Key: "Bank-Id", Value: "TEST_BANK"},
        },
        // ... other test fields
    }
    
    // Headers will include both explicit and default values
    testingtools.TestDB(t, &testCase, &testingtools.TestOptions{
        // ... test options
    })
}
```

## Configuration Structure

### Header Types:

1. **Required Headers**: Must be present in every request
   - `Request-Id`: Unique request identifier
   - `User-Id`: User identifier

2. **Optional Headers**: Can be present, with default values
   - `Branch-Id`: Branch identifier
   - `Bank-Id`: Bank identifier
   - `Person-Id`: Person identifier
   - `Department-Id`: Department identifier
   - `Region-Id`: Region identifier

3. **Custom Headers**: Dynamically added headers
   - `Tenant-Id`: Multi-tenant identifier
   - `Environment`: Environment identifier
   - `Client-Version`: Client application version
   - `Session-Id`: Session identifier

### Validation Rules:

- **MinLength**: Minimum character length
- **MaxLength**: Maximum character length
- **Required**: Whether header is mandatory
- **DefaultValue**: Default value if not provided
- **ValidationRule**: Custom validation logic

## Migration Strategy

### Phase 1: **Parallel Implementation**
- Keep existing `RequestHeader` for backward compatibility
- Add `DynamicRequestHeader` alongside existing code
- Use migration utilities to convert between formats

### Phase 2: **Gradual Migration**
- Update tests to use dynamic headers
- Migrate handlers one by one
- Validate migration with comparison utilities

### Phase 3: **Full Migration**
- Remove legacy `RequestHeader` (optional)
- Use only `DynamicRequestHeader`
- Clean up migration utilities

## Benefits

### ✅ **Flexibility**
- Add new headers without code changes
- Environment-specific configurations
- Runtime header validation

### ✅ **Maintainability**
- Centralized header configuration
- Easy testing with configurable defaults
- Clear separation of concerns

### ✅ **Scalability**
- Support for multi-tenant applications
- Easy addition of new business domains
- Configurable validation rules

### ✅ **Backward Compatibility**
- Legacy headers still work
- Gradual migration path
- No breaking changes

## Environment Variables

- `HEADER_CONFIG_PATH`: Path to header configuration file
- Default locations checked: `config/dynamic_headers.yaml`, `dynamic_headers.yaml`

## Next Steps

1. **Start Using**: The library works out-of-the-box with sensible defaults
2. **Customize Headers** (Optional): Create `config/dynamic_headers.yaml` for custom configuration
3. **Initialize Config** (Optional): Call `libRequest.InitGlobalHeaderConfig()` if using custom config
4. **Migrate Gradually**: Use migration utilities to convert existing code
5. **Test Thoroughly**: Use the updated testing tools with dynamic headers
6. **Monitor**: Validate headers in production with the validation utilities

This implementation provides a robust, flexible solution for header management while maintaining backward compatibility and providing a clear migration path.
