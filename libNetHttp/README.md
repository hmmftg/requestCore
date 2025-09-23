# Net/HTTP Web Framework Support for requestCore

This package provides support for Go's standard `net/http` package as a web framework in the requestCore ecosystem.

## Features

- ✅ Full compatibility with requestCore's `RequestParser` interface
- ✅ Support for all standard HTTP methods (GET, POST, PUT, DELETE, PATCH)
- ✅ Built-in middleware support (CORS, Logging, Recovery, Auth)
- ✅ File upload and download capabilities
- ✅ Cookie handling
- ✅ URL parameter and query parameter extraction
- ✅ JSON request/response handling
- ✅ Form data parsing
- ✅ Custom error handling
- ✅ Static file serving
- ✅ Redirect support

## Quick Start

### Basic Server Setup

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/hmmftg/requestCore/libNetHttp"
)

func main() {
    // Create a server with requestCore integration
    server := libNetHttp.CreateExampleServer()
    
    log.Println("Starting server on :8080")
    log.Fatal(server.ListenAndServe())
}
```

### Custom Handler with requestCore

```go
func MyHandler(w http.ResponseWriter, r *http.Request) {
    // Initialize requestCore context
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Get request information
    method := parser.GetMethod()
    path := parser.GetPath()
    
    // Parse JSON body
    var requestData MyRequestStruct
    err := parser.GetBody(&requestData)
    if err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Process request...
    
    // Send JSON response
    response := MyResponseStruct{
        Message: "Success",
        Data:    requestData,
    }
    
    parser.SendJSONRespBody(http.StatusOK, response)
}
```

## Middleware

### Built-in Middleware

```go
// Chain multiple middleware
handler := libNetHttp.ChainMiddleware(
    libNetHttp.LoggingMiddleware(),
    libNetHttp.CORSMiddleware(),
    libNetHttp.RecoveryMiddleware(),
    libNetHttp.AuthMiddleware(),
)

mux.HandleFunc("/api/endpoint", handler(http.HandlerFunc(MyHandler)).ServeHTTP)
```

### Custom Middleware

```go
func CustomMiddleware() libNetHttp.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Pre-processing
            log.Println("Before handler")
            
            // Call next middleware/handler
            next.ServeHTTP(w, r)
            
            // Post-processing
            log.Println("After handler")
        })
    }
}
```

## Request Parsing

### JSON Body Parsing

```go
type UserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    var user UserRequest
    err := parser.GetBody(&user)
    if err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Process user creation...
}
```

### URL Parameters

```go
func GetUser(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Set URL parameters (typically done by your router)
    parser.AddParam("id", "123")
    
    // Get parameter
    userID := parser.GetUrlParam("id")
    
    // Get all parameters
    params := parser.GetUrlParams()
}
```

### Query Parameters

```go
func SearchUsers(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Parse query parameters into struct
    type SearchParams struct {
        Query string `json:"q"`
        Limit int    `json:"limit"`
    }
    
    var searchParams SearchParams
    err := parser.GetUrlQuery(&searchParams)
    if err != nil {
        http.Error(w, "Invalid query parameters", http.StatusBadRequest)
        return
    }
    
    // Process search...
}
```

### Form Data

```go
func HandleForm(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Parse form data
    err := parser.ParseForm()
    if err != nil {
        http.Error(w, "Error parsing form", http.StatusBadRequest)
        return
    }
    
    // Get form values
    name := parser.GetFormValue("name")
    email := parser.GetFormValue("email")
    
    // Get all values for a key
    tags := parser.GetFormValues("tags")
}
```

## File Operations

### File Upload

```go
func UploadFile(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Parse multipart form
    err := parser.ParseMultipartForm(32 << 20) // 32 MB max
    if err != nil {
        http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
        return
    }
    
    // Save uploaded file
    err = parser.SaveFile("file", "/uploads/uploaded_file.txt")
    if err != nil {
        http.Error(w, "Error saving file", http.StatusInternalServerError)
        return
    }
    
    // Response
    response := map[string]string{
        "message": "File uploaded successfully",
    }
    parser.SendJSONRespBody(http.StatusOK, response)
}
```

### File Download

```go
func DownloadFile(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Serve file as attachment
    parser.FileAttachment("/path/to/file.pdf", "document.pdf")
}
```

## Cookie Handling

```go
func HandleCookies(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Get all cookies
    cookies := parser.GetCookies()
    
    // Get specific cookie
    sessionCookie, err := parser.GetCookie("session")
    if err != nil {
        // Cookie not found
    }
    
    // Set new cookie
    cookie := &http.Cookie{
        Name:    "user_preference",
        Value:   "dark_mode",
        Expires: time.Now().Add(24 * time.Hour),
        Path:    "/",
    }
    parser.SetCookie(cookie)
}
```

## Error Handling

```go
func ErrorHandler(w http.ResponseWriter, r *http.Request) {
    wf := libContext.InitNetHttpContext(r, w, false)
    parser := wf.Parser.(libNetHttp.NetHttpParser)
    
    // Custom error response
    errorResponse := map[string]string{
        "error":   "Custom error message",
        "code":    "CUSTOM_ERROR",
        "details": "Additional error details",
    }
    
    parser.SendJSONRespBody(http.StatusBadRequest, errorResponse)
}
```

## Integration with BaseHandler

If you're using requestCore's BaseHandler, you can integrate it like this:

```go
func CreateServerWithBaseHandler() *http.Server {
    mux := http.NewServeMux()
    
    // Add middleware
    handler := libNetHttp.ChainMiddleware(
        libNetHttp.LoggingMiddleware(),
        libNetHttp.CORSMiddleware(),
        libNetHttp.RecoveryMiddleware(),
    )
    
    // Wrap BaseHandler with NetHttpHandler
    mux.HandleFunc("/api/users", handler(libNetHttp.NetHttpHandler(
        handlers.BaseHandler(core, userHandler, false)
    )).ServeHTTP)
    
    return &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }
}
```

## Testing

### Unit Testing

```go
func TestMyHandler(t *testing.T) {
    // Create test request
    req := httptest.NewRequest("GET", "/api/test", nil)
    req.Header.Set("User-Id", "test-user")
    
    // Create response recorder
    w := httptest.NewRecorder()
    
    // Call handler
    MyHandler(w, req)
    
    // Assertions
    assert.Equal(t, http.StatusOK, w.Code)
    
    var response map[string]interface{}
    err := json.Unmarshal(w.Body.Bytes(), &response)
    assert.NoError(t, err)
    assert.Equal(t, "Success", response["message"])
}
```

## Performance Considerations

- Use connection pooling for database connections
- Implement proper caching strategies
- Use `http.ServeMux` for simple routing or consider `gorilla/mux` for complex routing
- Enable HTTP/2 for better performance
- Use middleware sparingly to avoid overhead

## Comparison with Other Frameworks

| Feature | net/http | Gin | Fiber |
|---------|----------|-----|-------|
| Performance | High | High | Very High |
| Memory Usage | Low | Medium | Low |
| Learning Curve | Medium | Low | Low |
| Ecosystem | Large | Large | Growing |
| Built-in Features | Basic | Rich | Rich |
| Middleware | Manual | Built-in | Built-in |

## Best Practices

1. **Use middleware for cross-cutting concerns** (logging, auth, CORS)
2. **Validate input early** in your handlers
3. **Use proper HTTP status codes**
4. **Implement proper error handling**
5. **Use context for request-scoped values**
6. **Implement graceful shutdown**
7. **Use structured logging**
8. **Add health check endpoints**

## Example Complete Server

```go
package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/hmmftg/requestCore/libNetHttp"
)

func main() {
    // Create server
    server := &http.Server{
        Addr:    ":8080",
        Handler: createHandler(),
    }
    
    // Start server in goroutine
    go func() {
        log.Println("Starting server on :8080")
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed to start: %v", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("Shutting down server...")
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
    
    log.Println("Server exited")
}

func createHandler() http.Handler {
    mux := http.NewServeMux()
    
    // Add middleware
    handler := libNetHttp.ChainMiddleware(
        libNetHttp.LoggingMiddleware(),
        libNetHttp.CORSMiddleware(),
        libNetHttp.RecoveryMiddleware(),
    )
    
    // Routes
    mux.HandleFunc("/health", handler(http.HandlerFunc(healthHandler)).ServeHTTP)
    mux.HandleFunc("/api/users", handler(http.HandlerFunc(usersHandler)).ServeHTTP)
    
    return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    // Your user handling logic here
    response := map[string]string{"message": "Users endpoint"}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

This implementation provides full compatibility with requestCore while leveraging Go's standard `net/http` package for maximum performance and flexibility.
