// Package requestcore is the v2 module of requestCore.
//
// It provides seven Buffalo-inspired capabilities on top of the existing
// root module while preserving full backward compatibility:
//
//   - Error handler registry with per-status-code handlers
//   - Framework-agnostic route groups and middleware
//   - Resource-style seven-operation CRUD registration
//   - Pluggable renderers (JSON, XML, text, CSV)
//   - Bounded in-process worker pool with mandatory observability
//   - Pluggable session and flash management
//   - CLI code generators for handlers, resources, and services
//
// The v2 module imports the root module ([github.com/hmmftg/requestCore])
// for delegation to existing query, persistence, response, logging, and
// tracing infrastructure. The root module never imports v2.
//
// See MIGRATION.md for the v1-to-v2 migration guide.
package requestcore
