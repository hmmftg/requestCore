// Package validation provides struct-tag validation that returns
// response.Violation slices for integration with the v2 problem
// mapper.
//
// The Validator wraps go-playground/validator/v10 and resolves field
// names to their wire names (json/query/path/header tags) rather than
// Go struct field names. Violation ordering is deterministic: sorted
// by field name, then by rule.
//
// validation imports response (for the Violation type),
// go-playground/validator/v10, and the standard library. It does not
// import request, handlers, app, or any v1 package.
package validation

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/hmmftg/requestCore/v2/response"
)

// Validator wraps a go-playground/validator Validate instance and
// produces response.Violation slices with deterministic ordering and
// wire-name field resolution.
type Validator struct {
	v *validator.Validate
}

// New creates a Validator with the default configuration. The default
// validator uses struct tags as documented by go-playground/validator.
func New() *Validator {
	return &Validator{v: validator.New()}
}

// Validate validates target and returns a slice of violations. If
// validation succeeds, the slice is empty and the error is nil.
//
// A non-nil error indicates an internal failure (e.g. target is not a
// struct), not a validation failure. Validation failures are always
// returned as violations.
//
// Field names are resolved to their wire names by inspecting the
// `json`, `query`, `path`, and `header` struct tags in that order. If
// no tag is present, the Go field name is used.
//
// Violations are sorted deterministically by field name, then by rule.
func (val *Validator) Validate(target any) ([]response.Violation, error) {
	if target == nil {
		return nil, fmt.Errorf("validation: nil target")
	}
	err := val.v.Struct(target)
	if err == nil {
		return nil, nil
	}
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		// Not a ValidationErrors — this is an internal failure.
		return nil, fmt.Errorf("validation: %w", err)
	}

	rt := reflect.TypeOf(target)
	// Dereference pointers.
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	violations := make([]response.Violation, 0, len(errs))
	for _, fe := range errs {
		field := resolveFieldName(rt, fe.StructField())
		rule := fe.Tag()
		message := formatMessage(field, rule, fe)
		violations = append(violations, response.Violation{
			Field:   field,
			Rule:    rule,
			Message: message,
		})
	}

	// Deterministic ordering: sort by field, then rule.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Field != violations[j].Field {
			return violations[i].Field < violations[j].Field
		}
		return violations[i].Rule < violations[j].Rule
	})

	return violations, nil
}

// resolveFieldName resolves a Go struct field name to its wire name
// by inspecting struct tags. The order of preference is json, query,
// path, header. If no tag is present, the Go field name is returned.
func resolveFieldName(rt reflect.Type, goName string) string {
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return goName
	}
	f, ok := rt.FieldByName(goName)
	if !ok {
		return goName
	}
	for _, tag := range []string{"json", "query", "path", "header"} {
		if v, ok := f.Tag.Lookup(tag); ok && v != "-" {
			name := strings.Split(v, ",")[0]
			if name != "" {
				return name
			}
		}
	}
	return goName
}

// formatMessage builds a human-readable message for a validation
// violation.
func formatMessage(field, rule string, fe validator.FieldError) string {
	switch rule {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, fe.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters long", field, fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	default:
		return fmt.Sprintf("%s failed %s validation", field, rule)
	}
}
