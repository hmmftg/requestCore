// Package binding provides framework-neutral request decoding for the
// redesigned v2 kernel.
//
// Bind decodes request data (JSON body, query parameters, path
// parameters, headers) into a typed target struct based on a Plan.
// Struct tags determine which fields map to which sources:
//
//   - `json:"name"` for JSON body fields
//   - `query:"name"` for URL query parameters
//   - `path:"name"` for path parameters
//   - `header:"name"` for request headers
//
// POST/PUT/PATCH helpers default to bounded JSON decoding.
// GET/DELETE/HEAD default to query/path/header binding with no body.
//
// binding imports only the request package and the Go standard
// library. It does not import response, handlers, app, or any v1
// package.
package binding

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/hmmftg/requestCore/v2/request"
)

// Mode specifies which request source to bind from.
type Mode int

const (
	// ModeNone indicates no binding. The target is left at its zero value.
	ModeNone Mode = iota

	// ModeJSON binds from the JSON request body.
	ModeJSON

	// ModeQuery binds from URL query parameters using `query` tags.
	ModeQuery

	// ModePath binds from path parameters using `path` tags.
	ModePath

	// ModeHeader binds from request headers using `header` tags.
	ModeHeader

	// ModeForm binds from form data (application/x-www-form-urlencoded).
	ModeForm
)

// String returns a human-readable name for the mode.
func (m Mode) String() string {
	switch m {
	case ModeNone:
		return "none"
	case ModeJSON:
		return "json"
	case ModeQuery:
		return "query"
	case ModePath:
		return "path"
	case ModeHeader:
		return "header"
	case ModeForm:
		return "form"
	default:
		return "unknown"
	}
}

// Plan describes how to bind a request into a typed value.
type Plan struct {
	// Mode specifies the primary binding source.
	Mode Mode

	// DisallowUnknownFields rejects unknown JSON fields during decoding.
	// Only applies to ModeJSON.
	DisallowUnknownFields bool

	// MaxBodyBytes limits the request body size. If 0, no limit is
	// enforced. Only applies to ModeJSON and ModeForm.
	MaxBodyBytes int64
}

// Default plans for common binding patterns.
var (
	DefaultJSONPlan   = Plan{Mode: ModeJSON, DisallowUnknownFields: false, MaxBodyBytes: 1 << 20}
	DefaultQueryPlan  = Plan{Mode: ModeQuery}
	DefaultPathPlan   = Plan{Mode: ModePath}
	DefaultHeaderPlan = Plan{Mode: ModeHeader}
)

// Bind decodes request data into target based on the given plan.
// target must be a non-nil pointer to a struct.
//
// For ModeJSON, the body is read with a size limit (MaxBodyBytes).
// Trailing JSON values are rejected (ErrTrailingData). Body overflow
// returns ErrBodyTooLarge. Decode errors return ErrInvalidJSON.
//
// For ModeQuery, ModePath, ModeHeader, the corresponding request data
// is mapped into the target struct using struct tags.
func Bind(ctx *request.Context, plan Plan, target any) error {
	if target == nil {
		return fmt.Errorf("binding: nil target")
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("binding: target must be a non-nil pointer, got %T", target)
	}

	switch plan.Mode {
	case ModeNone:
		return nil
	case ModeJSON:
		return bindJSON(ctx, plan, target)
	case ModeQuery:
		return bindTagged(ctx.QueryAll, "query", target)
	case ModePath:
		return bindTagged(func(name string) []string {
			if v := ctx.PathParam(name); v != "" {
				return []string{v}
			}
			return nil
		}, "path", target)
	case ModeHeader:
		return bindTagged(func(name string) []string {
			h := ctx.Headers()
			return h[name]
		}, "header", target)
	case ModeForm:
		return bindForm(ctx, plan, target)
	default:
		return fmt.Errorf("binding: unknown mode %d", plan.Mode)
	}
}

// bindJSON reads the request body and decodes it as JSON into target.
// When a non-empty Content-Type is present on the request, it must be
// a JSON media type (application/json or any "+json" structured suffix);
// otherwise ErrInvalidContentType (HTTP 415) is returned. An absent
// Content-Type is accepted for compatibility.
func bindJSON(ctx *request.Context, plan Plan, target any) error {
	if ct := ctx.Header("Content-Type"); ct != "" {
		if !isJSONContentType(ct) {
			return &BindingError{
				Status:  http.StatusUnsupportedMediaType,
				Cause:   ErrInvalidContentType,
				Message: fmt.Sprintf("content type %q is not a JSON media type", ct),
			}
		}
	}

	body, err := ctx.BodyBytes(plan.MaxBodyBytes)
	if err != nil {
		return &BindingError{
			Status:  http.StatusRequestEntityTooLarge,
			Cause:   ErrBodyTooLarge,
			Message: err.Error(),
		}
	}
	if len(body) == 0 {
		// Empty body: leave target at zero value. This is valid for
		// requests with no body (e.g. POST with no content).
		return nil
	}

	dec := json.NewDecoder(strings.NewReader(string(body)))
	if plan.DisallowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(target); err != nil {
		if err == io.EOF {
			// Empty body after all — treat as no body.
			return nil
		}
		return &BindingError{
			Status:  http.StatusBadRequest,
			Cause:   ErrInvalidJSON,
			Message: err.Error(),
		}
	}
	// Check for trailing data after the first JSON value.
	if dec.More() {
		return &BindingError{
			Status:  http.StatusBadRequest,
			Cause:   ErrTrailingData,
			Message: "trailing data after JSON value",
		}
	}
	return nil
}

// isJSONContentType reports whether the given Content-Type header value
// is a JSON media type. It accepts application/json and any structured
// suffix media type ending in +json (RFC 6839). Parameters are ignored.
func isJSONContentType(ct string) bool {
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	if mediaType == "application/json" {
		return true
	}
	return strings.HasSuffix(mediaType, "+json")
}

// isFormContentType reports whether the given Content-Type header value
// is application/x-www-form-urlencoded. Parameters are ignored.
func isFormContentType(ct string) bool {
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == "application/x-www-form-urlencoded"
}

// bindTagged binds from a multi-value source (query, path, header) into
// target using the named struct tag. The getter returns all values for
// a given key. Falls back to the `json` tag if the primary tag is
// absent.
func bindTagged(getter func(name string) []string, tag string, target any) error {
	return bindTaggedWithFallback(getter, tag, "json", target)
}

// bindForm binds from form-encoded body data. When a non-empty
// Content-Type is present on the request, it must be
// application/x-www-form-urlencoded; otherwise ErrInvalidContentType
// (HTTP 415) is returned. An absent Content-Type is accepted for
// compatibility.
func bindForm(ctx *request.Context, plan Plan, target any) error {
	if ct := ctx.Header("Content-Type"); ct != "" {
		if !isFormContentType(ct) {
			return &BindingError{
				Status:  http.StatusUnsupportedMediaType,
				Cause:   ErrInvalidContentType,
				Message: fmt.Sprintf("content type %q is not a form media type", ct),
			}
		}
	}

	body, err := ctx.BodyBytes(plan.MaxBodyBytes)
	if err != nil {
		return &BindingError{
			Status:  http.StatusRequestEntityTooLarge,
			Cause:   ErrBodyTooLarge,
			Message: err.Error(),
		}
	}
	if len(body) == 0 {
		return nil
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return &BindingError{
			Status:  http.StatusBadRequest,
			Cause:   ErrInvalidJSON,
			Message: fmt.Sprintf("invalid form data: %v", err),
		}
	}
	// Form binding falls back to the `query` tag if no `form` tag is
	// present, since form fields and query fields share the same
	// encoding (url.Values).
	return bindTaggedWithFallback(func(name string) []string {
		return vals[name]
	}, "form", "query", target)
}

// bindTaggedWithFallback binds from a multi-value source using the
// primary tag, falling back to fallbackTag (then json) if the primary
// tag is absent.
func bindTaggedWithFallback(getter func(name string) []string, tag, fallbackTag string, target any) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("binding: target must be a pointer")
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("binding: target must point to a struct, got %s", elem.Kind())
	}
	rt := elem.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tagVal, ok := field.Tag.Lookup(tag)
		if !ok || tagVal == "-" {
			if fb, ok2 := field.Tag.Lookup(fallbackTag); ok2 && fb != "-" {
				tagVal = fb
			} else if jsonTag, ok2 := field.Tag.Lookup("json"); ok2 && jsonTag != "-" {
				tagVal = strings.Split(jsonTag, ",")[0]
			}
			if tagVal == "" {
				continue
			}
		}
		parts := strings.Split(tagVal, ",")
		name := parts[0]
		if name == "" {
			continue
		}
		values := getter(name)
		if len(values) == 0 {
			continue
		}
		if err := setField(elem.Field(i), values); err != nil {
			return &BindingError{
				Status:  http.StatusBadRequest,
				Cause:   ErrInvalidJSON,
				Field:   name,
				Message: err.Error(),
			}
		}
	}
	return nil
}

// setField sets a struct field from a slice of string values. Supports
// string, int, int64, float64, bool, and slices of those types.
func setField(field reflect.Value, values []string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	// Handle pointer types by creating the element.
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}

	// Handle slice types.
	if field.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(field.Type(), len(values), len(values))
		for i, v := range values {
			if err := setScalar(slice.Index(i), v); err != nil {
				return err
			}
		}
		field.Set(slice)
		return nil
	}

	// Scalar: use the first value.
	if len(values) == 0 {
		return nil
	}
	return setScalar(field, values[0])
}

// setScalar sets a scalar reflect.Value from a string.
func setScalar(field reflect.Value, s string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q: %w", s, err)
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer %q: %w", s, err)
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", s, err)
		}
		field.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("invalid boolean %q: %w", s, err)
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported field type %s", field.Kind())
	}
	return nil
}
