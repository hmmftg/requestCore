package libTracing

import (
	"net/http"
	"strings"
)

// HTTPHeaderCarrier implements the TextMapCarrier interface for HTTP headers
type HTTPHeaderCarrier struct {
	headers http.Header
}

// NewHTTPHeaderCarrier creates a new HTTP header carrier
func NewHTTPHeaderCarrier(headers http.Header) *HTTPHeaderCarrier {
	return &HTTPHeaderCarrier{headers: headers}
}

// Get returns the value for the given key
func (c *HTTPHeaderCarrier) Get(key string) string {
	return c.headers.Get(key)
}

// Set sets the value for the given key
func (c *HTTPHeaderCarrier) Set(key, value string) {
	c.headers.Set(key, value)
}

// Keys returns all keys
func (c *HTTPHeaderCarrier) Keys() []string {
	var keys []string
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// MapCarrier implements the TextMapCarrier interface for map[string]string
type MapCarrier struct {
	headers map[string]string
}

// NewMapCarrier creates a new map carrier
func NewMapCarrier(headers map[string]string) *MapCarrier {
	return &MapCarrier{headers: headers}
}

// Get returns the value for the given key
func (c *MapCarrier) Get(key string) string {
	return c.headers[key]
}

// Set sets the value for the given key
func (c *MapCarrier) Set(key, value string) {
	c.headers[key] = value
}

// Keys returns all keys
func (c *MapCarrier) Keys() []string {
	var keys []string
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

// TraceContextHeaders contains the standard trace context header names
var TraceContextHeaders = struct {
	TraceParent string
	TraceState  string
	Baggage     string
}{
	TraceParent: "traceparent",
	TraceState:  "tracestate",
	Baggage:     "baggage",
}

// ExtractTraceContextFromRequest extracts trace context from HTTP request
func ExtractTraceContextFromRequest(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// Extract trace context headers
	if traceParent := r.Header.Get(TraceContextHeaders.TraceParent); traceParent != "" {
		headers[TraceContextHeaders.TraceParent] = traceParent
	}
	if traceState := r.Header.Get(TraceContextHeaders.TraceState); traceState != "" {
		headers[TraceContextHeaders.TraceState] = traceState
	}
	if baggage := r.Header.Get(TraceContextHeaders.Baggage); baggage != "" {
		headers[TraceContextHeaders.Baggage] = baggage
	}

	return headers
}

// InjectTraceContextToRequest injects trace context into HTTP request
func InjectTraceContextToRequest(r *http.Request, headers map[string]string) {
	for key, value := range headers {
		// Only inject trace context headers
		if key == TraceContextHeaders.TraceParent ||
			key == TraceContextHeaders.TraceState ||
			key == TraceContextHeaders.Baggage {
			r.Header.Set(key, value)
		}
	}
}

// InjectTraceContextToResponse injects trace context into HTTP response
func InjectTraceContextToResponse(w http.ResponseWriter, headers map[string]string) {
	for key, value := range headers {
		// Only inject trace context headers
		if key == TraceContextHeaders.TraceParent ||
			key == TraceContextHeaders.TraceState ||
			key == TraceContextHeaders.Baggage {
			w.Header().Set(key, value)
		}
	}
}

// CleanSensitiveHeaders removes sensitive headers from tracing
func CleanSensitiveHeaders(headers map[string]string, sensitiveHeaders []string) map[string]string {
	cleaned := make(map[string]string)

	for k, v := range headers {
		// Check if header is sensitive
		isSensitive := false
		for _, sensitive := range sensitiveHeaders {
			if strings.EqualFold(k, sensitive) {
				isSensitive = true
				break
			}
		}

		if !isSensitive {
			cleaned[k] = v
		} else {
			// Replace sensitive value with masked value
			cleaned[k] = "[REDACTED]"
		}
	}

	return cleaned
}

// CleanSensitiveQueryParams removes sensitive query parameters from tracing
func CleanSensitiveQueryParams(queryParams map[string]string, sensitiveParams []string) map[string]string {
	cleaned := make(map[string]string)

	for k, v := range queryParams {
		// Check if parameter is sensitive
		isSensitive := false
		for _, sensitive := range sensitiveParams {
			if strings.EqualFold(k, sensitive) {
				isSensitive = true
				break
			}
		}

		if !isSensitive {
			cleaned[k] = v
		} else {
			// Replace sensitive value with masked value
			cleaned[k] = "[REDACTED]"
		}
	}

	return cleaned
}

// TraceContextInfo holds trace context information
type TraceContextInfo struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Sampled    bool              `json:"sampled"`
	Baggage    map[string]string `json:"baggage"`
	TraceState string            `json:"trace_state"`
}

// ParseTraceParent parses a traceparent header value
func ParseTraceParent(traceParent string) *TraceContextInfo {
	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		return nil
	}

	traceID := parts[1]
	spanID := parts[2]
	traceFlags := parts[3]

	// Parse trace flags (last character indicates sampling)
	sampled := false
	if len(traceFlags) > 0 && traceFlags[len(traceFlags)-1] == '1' {
		sampled = true
	}

	return &TraceContextInfo{
		TraceID: traceID,
		SpanID:  spanID,
		Sampled: sampled,
	}
}

// FormatTraceParent formats trace context info into traceparent header
func FormatTraceParent(info *TraceContextInfo) string {
	samplingFlag := "0"
	if info.Sampled {
		samplingFlag = "1"
	}

	return "00-" + info.TraceID + "-" + info.SpanID + "-" + samplingFlag
}
