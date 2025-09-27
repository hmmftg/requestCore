# OpenTelemetry Integration for requestCore

## Overview

This implementation adds comprehensive OpenTelemetry tracing support to the requestCore library, enabling observability across all web frameworks (Gin, Fiber, net/http) and providing detailed tracing for HTTP requests, database operations, and external API calls.

## Key Features

### ✅ **Multi-Framework Support**
- **Gin**: Official `otelgin` instrumentation
- **Fiber**: Official `otelfiber` instrumentation  
- **net/http**: Official `otelhttp` instrumentation
- **Unified Interface**: Consistent tracing API across all frameworks

### ✅ **Comprehensive Tracing**
- **HTTP Requests**: Automatic request/response tracing
- **Database Operations**: Query and transaction tracing
- **External API Calls**: Outbound request tracing
- **Custom Spans**: Manual span creation for business logic

### ✅ **Flexible Configuration**
- **Multiple Exporters**: Jaeger, Zipkin, stdout, none
- **Environment Variables**: Runtime configuration
- **YAML Configuration**: File-based configuration
- **Sampling Control**: Configurable sampling ratios

### ✅ **Rich Context**
- **Request Headers**: Automatic header extraction
- **User Context**: User ID and session tracking
- **Business Context**: Program, module, method tracking
- **Error Tracking**: Detailed error recording

## Files Created/Modified

### New Files:
- `libTracing/tracer.go` - Core tracing manager and functionality
- `libTracing/model.go` - Tracing data structures and models
- `libTracing/carrier.go` - Trace context propagation
- `libTracing/config.go` - Configuration management
- `libTracing/tracer_test.go` - Comprehensive tests
- `libGin/tracing.go` - Gin tracing middleware
- `libGin/tracing_test.go` - Gin tracing tests
- `libFiber/tracing.go` - Fiber tracing middleware
- `libFiber/tracing_test.go` - Fiber tracing tests
- `libNetHttp/tracing.go` - net/http tracing middleware

### Modified Files:
- `webFramework/model.go` - Extended interfaces with tracing support
- `libRequest/model.go` - Added tracing fields to Request struct
- `libContext/init.go` - Integrated tracing context initialization
- `handlers/baseHandler.go` - Added tracing support to handlers

## Configuration

### Environment Variables

```bash
# Service information
export TRACING_SERVICE_NAME="requestCore"
export TRACING_SERVICE_VERSION="1.0.0"
export TRACING_ENVIRONMENT="production"

# Exporter configuration
export TRACING_EXPORTER="jaeger"  # jaeger, zipkin, stdout, none
export TRACING_JAEGER_ENDPOINT="http://localhost:14268/api/traces"
export TRACING_ZIPKIN_ENDPOINT="http://localhost:9411/api/v2/spans"

# Sampling configuration
export TRACING_SAMPLING_RATIO="1.0"  # 0.0 to 1.0
export TRACING_ENABLED="true"
```

### YAML Configuration

Create a `tracing_config.yaml` file:

```yaml
service_name: "requestCore"
service_version: "1.0.0"
environment: "production"
exporter: "jaeger"
jaeger_endpoint: "http://localhost:14268/api/traces"
zipkin_endpoint: "http://localhost:9411/api/v2/spans"
sampling_ratio: 1.0
enabled: true
attributes:
  custom.key: "custom.value"
  deployment.region: "us-east-1"
```

## Usage Examples

### 1. **Basic Initialization**

```go
package main

import (
    "context"
    "log"
    
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    // Initialize tracing from environment variables
    err := libTracing.InitializeTracingFromEnv()
    if err != nil {
        log.Fatal("Failed to initialize tracing:", err)
    }
    
    // Or initialize from config file
    err = libTracing.InitializeTracingFromConfig("tracing_config.yaml")
    if err != nil {
        log.Fatal("Failed to initialize tracing:", err)
    }
    
    // Graceful shutdown
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        libTracing.ShutdownTracing(ctx)
    }()
    
    // Your application code here...
}
```

### 2. **Gin Integration**

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/hmmftg/requestCore/libGin"
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    // Initialize tracing
    libTracing.InitializeTracingFromEnv()
    
    // Create Gin router
    r := gin.Default()
    
    // Add tracing middleware
    r.Use(libGin.TracingMiddleware())
    
    // Add routes
    r.GET("/users/:id", func(c *gin.Context) {
        // Add custom span attributes
        libGin.AddSpanAttribute(c, "user.id", c.Param("id"))
        libGin.AddSpanAttributes(c, map[string]string{
            "operation": "get_user",
            "resource": "users",
        })
        
        // Add span event
        libGin.AddSpanEvent(c, "user_lookup", map[string]string{
            "user_id": c.Param("id"),
        })
        
        c.JSON(200, gin.H{"id": c.Param("id")})
    })
    
    r.Run(":8080")
}
```

### 3. **Fiber Integration**

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/hmmftg/requestCore/libFiber"
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    // Initialize tracing
    libTracing.InitializeTracingFromEnv()
    
    // Create Fiber app
    app := fiber.New()
    
    // Add tracing middleware
    app.Use(libFiber.TracingMiddleware())
    
    // Add routes
    app.Get("/users/:id", func(c *fiber.Ctx) error {
        // Add custom span attributes
        libFiber.AddSpanAttribute(c, "user.id", c.Params("id"))
        libFiber.AddSpanAttributes(c, map[string]string{
            "operation": "get_user",
            "resource": "users",
        })
        
        // Add span event
        libFiber.AddSpanEvent(c, "user_lookup", map[string]string{
            "user_id": c.Params("id"),
        })
        
        return c.JSON(fiber.Map{"id": c.Params("id")})
    })
    
    app.Listen(":8080")
}
```

### 4. **net/http Integration**

```go
package main

import (
    "net/http"
    
    "github.com/hmmftg/requestCore/libNetHttp"
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    // Initialize tracing
    libTracing.InitializeTracingFromEnv()
    
    // Create HTTP handler
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add custom span attributes
        libNetHttp.AddSpanAttribute(r.Context(), "operation", "get_user")
        libNetHttp.AddSpanAttributes(r.Context(), map[string]string{
            "resource": "users",
            "method": r.Method,
        })
        
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Hello, World!"))
    })
    
    // Add tracing middleware
    tracedHandler := libNetHttp.TracingMiddleware()(handler)
    
    http.Handle("/", tracedHandler)
    http.ListenAndServe(":8080", nil)
}
```

### 5. **Handler Integration**

```go
package handlers

import (
    "github.com/hmmftg/requestCore/handlers"
    "github.com/hmmftg/requestCore/libRequest"
    "github.com/hmmftg/requestCore/webFramework"
)

type UserHandler struct{}

func (h UserHandler) Parameters() handlers.HandlerParameters {
    return handlers.HandlerParameters{
        Title:           "GetUser",
        Body:            libRequest.JSON,
        ValidateHeader:  true,
        SaveToRequest:   true,
        Path:            "/users/:id",
        EnableTracing:   true,  // Enable tracing for this handler
        TracingSpanName: "get_user_handler",
    }
}

func (h UserHandler) Handler(req handlers.HandlerRequest[GetUserRequest, GetUserResponse]) (GetUserResponse, error) {
    // Add custom span attributes
    req.AddSpanAttribute("user.id", req.Request.UserID)
    req.AddSpanAttributes(map[string]string{
        "operation": "get_user",
        "resource": "users",
    })
    
    // Add span event
    req.AddSpanEvent("user_lookup_started", map[string]string{
        "user_id": req.Request.UserID,
    })
    
    // Start child span for database operation
    ctx, childSpan := req.StartChildSpan("database_query", map[string]string{
        "db.operation": "SELECT",
        "db.table": "users",
    })
    defer childSpan.End()
    
    // Your business logic here...
    user, err := getUserFromDB(ctx, req.Request.UserID)
    if err != nil {
        req.RecordSpanError(err, map[string]string{
            "error.type": "database_error",
        })
        return GetUserResponse{}, err
    }
    
    return GetUserResponse{User: user}, nil
}
```

### 6. **Custom Span Creation**

```go
package main

import (
    "context"
    "time"
    
    "github.com/hmmftg/requestCore/libTracing"
)

func processOrder(ctx context.Context, orderID string) error {
    // Start custom span
    ctx, span := libTracing.StartSpan(ctx, "process_order", map[string]string{
        "order.id": orderID,
        "operation": "process_order",
    })
    defer span.End()
    
    // Add attributes
    libTracing.AddSpanAttributes(ctx, map[string]string{
        "order.status": "processing",
        "order.timestamp": time.Now().Format(time.RFC3339),
    })
    
    // Add event
    libTracing.AddSpanEvent(ctx, "order_validation_started", map[string]string{
        "order.id": orderID,
    })
    
    // Validate order
    if err := validateOrder(ctx, orderID); err != nil {
        libTracing.RecordError(ctx, err, map[string]string{
            "error.type": "validation_error",
        })
        return err
    }
    
    // Process payment
    if err := processPayment(ctx, orderID); err != nil {
        libTracing.RecordError(ctx, err, map[string]string{
            "error.type": "payment_error",
        })
        return err
    }
    
    // Add success event
    libTracing.AddSpanEvent(ctx, "order_processed_successfully", map[string]string{
        "order.id": orderID,
        "order.status": "completed",
    })
    
    return nil
}
```

### 7. **Database Tracing**

```go
package main

import (
    "context"
    "database/sql"
    
    "github.com/hmmftg/requestCore/libTracing"
)

func queryUsers(ctx context.Context, db *sql.DB) ([]User, error) {
    // Start database span
    ctx, span := libTracing.StartSpan(ctx, "database_query", map[string]string{
        "db.operation": "SELECT",
        "db.table": "users",
        "db.statement": "SELECT * FROM users WHERE active = ?",
    })
    defer span.End()
    
    // Execute query
    rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE active = ?", true)
    if err != nil {
        libTracing.RecordError(ctx, err, map[string]string{
            "error.type": "database_error",
        })
        return nil, err
    }
    defer rows.Close()
    
    var users []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            libTracing.RecordError(ctx, err, map[string]string{
                "error.type": "scan_error",
            })
            return nil, err
        }
        users = append(users, user)
    }
    
    // Add result count
    libTracing.AddSpanAttribute(ctx, "db.result_count", string(len(users)))
    
    return users, nil
}
```

### 8. **External API Tracing**

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/hmmftg/requestCore/libTracing"
)

func callExternalAPI(ctx context.Context, url string) (*http.Response, error) {
    // Start API span
    ctx, span := libTracing.StartSpan(ctx, "external_api_call", map[string]string{
        "http.method": "GET",
        "http.url": url,
        "api.service": "external_service",
    })
    defer span.End()
    
    // Create HTTP client with tracing
    client := &http.Client{
        Transport: libTracing.TracingMiddleware()(http.DefaultTransport),
    }
    
    // Create request
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        libTracing.RecordError(ctx, err, map[string]string{
            "error.type": "request_creation_error",
        })
        return nil, err
    }
    
    // Execute request
    resp, err := client.Do(req)
    if err != nil {
        libTracing.RecordError(ctx, err, map[string]string{
            "error.type": "http_error",
        })
        return nil, err
    }
    
    // Add response attributes
    libTracing.AddSpanAttributes(ctx, map[string]string{
        "http.status_code": string(resp.StatusCode),
        "http.response_size": string(resp.ContentLength),
    })
    
    return resp, nil
}
```

## Advanced Configuration

### Custom Exporter Configuration

```go
package main

import (
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    config := &libTracing.TracingConfig{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        Exporter:       "jaeger",
        JaegerEndpoint: "http://jaeger:14268/api/traces",
        SamplingRatio:  0.1, // Sample 10% of requests
        Enabled:        true,
        Attributes: map[string]string{
            "deployment.region": "us-east-1",
            "service.team": "backend",
        },
    }
    
    err := libTracing.InitializeTracing(config)
    if err != nil {
        log.Fatal("Failed to initialize tracing:", err)
    }
}
```

### Middleware Configuration

```go
package main

import (
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    middlewareConfig := &libTracing.TracingMiddlewareConfig{
        TraceHTTP:            true,
        TraceDB:              true,
        TraceAPI:             true,
        IncludeBodies:        false,
        MaxBodySize:          1024,
        SensitiveHeaders:     []string{"Authorization", "Cookie"},
        SensitiveQueryParams: []string{"password", "token"},
    }
    
    // Use middleware config in your tracing setup
    // This configuration is used by the tracing middleware
}
```

## Best Practices

### 1. **Span Naming**
- Use descriptive, hierarchical names
- Follow the pattern: `service.operation.resource`
- Examples: `user.get_by_id`, `order.process_payment`, `db.query_users`

### 2. **Attribute Naming**
- Use consistent naming conventions
- Follow OpenTelemetry semantic conventions
- Examples: `http.method`, `db.operation`, `user.id`

### 3. **Error Handling**
- Always record errors with context
- Include error type and relevant attributes
- Don't forget to set span status

### 4. **Performance**
- Use sampling in production (0.1-0.5)
- Avoid high-cardinality attributes
- Keep span names static

### 5. **Security**
- Exclude sensitive headers and parameters
- Use middleware configuration for filtering
- Don't log sensitive data in attributes

## Troubleshooting

### Common Issues

1. **Tracing not working**
   - Check if tracing is enabled in configuration
   - Verify exporter endpoints are accessible
   - Check sampling ratio settings

2. **High memory usage**
   - Reduce sampling ratio
   - Check for span leaks (not calling End())
   - Monitor exporter buffer sizes

3. **Missing traces**
   - Verify trace context propagation
   - Check if spans are being created
   - Ensure proper error handling

### Debug Mode

```go
package main

import (
    "github.com/hmmftg/requestCore/libTracing"
)

func main() {
    // Enable debug mode
    config := libTracing.DefaultTracingConfig()
    config.Exporter = "stdout" // Use stdout for debugging
    config.SamplingRatio = 1.0  // Sample all requests
    
    err := libTracing.InitializeTracing(config)
    if err != nil {
        log.Fatal("Failed to initialize tracing:", err)
    }
}
```

## Integration with Observability Platforms

### Jaeger
```yaml
exporter: "jaeger"
jaeger_endpoint: "http://jaeger:14268/api/traces"
```

### Zipkin
```yaml
exporter: "zipkin"
zipkin_endpoint: "http://zipkin:9411/api/v2/spans"
```

### OpenTelemetry Collector
```yaml
exporter: "otlp"
otlp_endpoint: "http://otel-collector:4317"
```

This comprehensive OpenTelemetry integration provides full observability for your requestCore applications across all supported web frameworks.
