package libRequest

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"
)

// RequestModel holds the database query interface and SQL statements for request persistence.
type RequestModel struct {
	QueryInterface libQuery.QueryRunnerInterface
	InsertInDb     string
	UpdateInDb     string
	QueryInDb      string
}

// RequestInterface defines the methods for initializing and persisting requests.
type RequestInterface interface {
	Initialize(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error)
	InitRequest(c webFramework.WebFramework, method, url string) error
	InitializeNoLog(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error)
	InsertRequest(request RequestPtr) error
	CheckDuplicateRequest(request RequestPtr) error
	UpdateRequestWithContext(ctx context.Context, request RequestPtr) error
}

// LogData holds a single log entry with timestamp and context metadata.
type LogData struct {
	Time    time.Time `json:"dt"`
	Program string    `json:"program"`
	Module  string    `json:"module"`
	Method  string    `json:"method"`
	LogText string    `json:"log_text"`
}

// EventData holds a collection of log entries with associated tags.
type EventData struct {
	Time time.Time         `json:"dt"`
	Tags map[string]string `json:"tags"`
	Logs []LogData         `json:"logs"`
}

// RequestHeader holds the standard HTTP request headers for requestCore.
type RequestHeader struct {
	RequestID string `header:"Request-Id" reqHeader:"Request-Id" validate:"required,min=10,max=64"`
	Program   string `header:"Program-Id" reqHeader:"Program-Id"`
	Module    string `header:"Module-Id"  reqHeader:"Module-Id"`
	Method    string `header:"Method-Id"  reqHeader:"Method-Id"`
	User      string `header:"User-Id"    reqHeader:"User-Id"`
	Branch    string `header:"Branch-Id"  reqHeader:"Branch-Id"`
	Bank      string `header:"Bank-Id"    reqHeader:"Bank-Id"`
	Person    string `header:"Person-Id"  reqHeader:"Person-Id"`
}

// GetID returns the request ID.
func (r RequestHeader) GetID() string {
	return r.RequestID
}

// GetUser returns the user ID from the header.
func (r RequestHeader) GetUser() string {
	return r.User
}

// GetBank returns the bank ID from the header.
func (r RequestHeader) GetBank() string {
	return r.Bank
}

// GetBranch returns the branch ID from the header.
func (r RequestHeader) GetBranch() string {
	return r.Branch
}

// GetPerson returns the person ID from the header.
func (r RequestHeader) GetPerson() string {
	return r.Person
}

// GetProgram returns the program ID from the header.
func (r RequestHeader) GetProgram() string {
	return r.Program
}

// GetModule returns the module ID from the header.
func (r RequestHeader) GetModule() string {
	return r.Module
}

// GetMethod returns the method ID from the header.
func (r RequestHeader) GetMethod() string {
	return r.Method
}

// SetUser sets the user ID on the header.
func (r *RequestHeader) SetUser(user string) {
	r.User = user
}

// SetProgram sets the program ID on the header.
func (r *RequestHeader) SetProgram(program string) {
	r.Program = program
}

// SetModule sets the module ID on the header.
func (r *RequestHeader) SetModule(module string) {
	r.Module = module
}

// SetMethod sets the method ID on the header.
func (r *RequestHeader) SetMethod(method string) {
	r.Method = method
}

// SetBranch sets the branch ID on the header.
func (r *RequestHeader) SetBranch(branch string) {
	r.Branch = branch
}

// SetBank sets the bank ID on the header.
func (r *RequestHeader) SetBank(bank string) {
	r.Bank = bank
}

// SetPerson sets the person ID on the header.
func (r *RequestHeader) SetPerson(person string) {
	r.Person = person
}

// RequestPtr is a pointer to a Request.
type RequestPtr *Request

// Request holds the full request lifecycle data including header, body, and tracing context.
type Request struct {
	Header    webFramework.HeaderInterface `json:"header"`
	ID        string                       `json:"id"`
	RequestID string                       `json:"request_id"`
	Time      time.Time                    `json:"dt"`
	Incoming  any                          `json:"incoming"`
	Req       string                       `json:"req"`
	Resp      string                       `json:"resp"`
	Outgoing  any                          `json:"outgoing"`
	Tags      map[string]string            `json:"tags"`
	Result    string                       `json:"result"`
	// Tracing fields
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
	Sampled bool   `json:"sampled"`
}

// Tracing methods for Request
func (r *Request) SetTraceContext(spanCtx trace.SpanContext) {
	r.TraceID = spanCtx.TraceID().String()
	r.SpanID = spanCtx.SpanID().String()
	r.Sampled = spanCtx.IsSampled()
}

// GetTraceContext reconstructs and returns the trace span context from the request.
func (r *Request) GetTraceContext() trace.SpanContext {
	if r.TraceID == "" || r.SpanID == "" {
		return trace.SpanContext{}
	}

	traceID, _ := trace.TraceIDFromHex(r.TraceID)
	spanID, _ := trace.SpanIDFromHex(r.SpanID)

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
}

// HasTraceContext reports whether the request has trace context information.
func (r *Request) HasTraceContext() bool {
	return r.TraceID != "" && r.SpanID != ""
}
