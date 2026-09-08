// Package idempotency provides reusable contracts for HTTP idempotency
// key handling, request fingerprinting, and replay-safe response
// capture. The package defines interfaces and helpers that allow
// applications to implement durable idempotency storage while the
// reusable library provides validation, fingerprinting, and safe
// response capture.
//
// The application retains ownership of:
//   - Durable storage (Redis, database, etc.)
//   - Transaction boundaries
//   - Replay policy (TTL, eviction)
//   - Concurrency control implementation
//
// The reusable library provides:
//   - Key validation (format, length, allowed characters)
//   - Request fingerprinting (deterministic hash of method, path, body)
//   - Response capture contracts (status, headers, body)
//   - Safe replay (never expose raw keys, never replay partial writes)
//   - Conflict detection interfaces
package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// KeyMaxLength is the maximum allowed length for an idempotency key.
// This prevents abuse via excessively long keys.
const KeyMaxLength = 255

// KeyMinLength is the minimum allowed length for an idempotency key.
const KeyMinLength = 1

// HeaderName is the standard HTTP header for idempotency keys.
const HeaderName = "Idempotency-Key"

// ErrInvalidKey indicates the idempotency key is malformed.
var ErrInvalidKey = errors.New("idempotency: invalid key")

// ErrKeyTooLong indicates the idempotency key exceeds the maximum length.
var ErrKeyTooLong = errors.New("idempotency: key too long")

// ErrKeyEmpty indicates the idempotency key is empty.
var ErrKeyEmpty = errors.New("idempotency: empty key")

// ErrFingerprintMismatch indicates the request fingerprint does not
// match the stored fingerprint for the given idempotency key.
var ErrFingerprintMismatch = errors.New("idempotency: fingerprint mismatch")

// ValidateKey validates an idempotency key. The key must be non-empty,
// no longer than KeyMaxLength, and contain only printable ASCII
// characters (no control characters, no whitespace-only). This
// prevents injection and abuse.
func ValidateKey(key string) error {
	if key == "" {
		return ErrKeyEmpty
	}
	if len(key) > KeyMaxLength {
		return ErrKeyTooLong
	}
	for _, r := range key {
		if r < 0x20 || r > 0x7e {
			return fmt.Errorf("%w: contains non-printable character", ErrInvalidKey)
		}
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%w: whitespace-only key", ErrInvalidKey)
	}
	return nil
}

// RequestFingerprint is a deterministic hash of the request method,
// path, and body. It is used to detect mismatched requests reusing the
// same idempotency key.
type RequestFingerprint struct {
	// Method is the HTTP method (uppercase).
	Method string

	// Path is the request path (including query string).
	Path string

	// BodyHash is the SHA-256 hash of the request body (hex-encoded).
	BodyHash string
}

// Hash returns a deterministic hash of the fingerprint, suitable for
// use as a storage key or comparison value.
func (f RequestFingerprint) Hash() string {
	h := sha256.New()
	h.Write([]byte(f.Method))
	h.Write([]byte{0})
	h.Write([]byte(f.Path))
	h.Write([]byte{0})
	h.Write([]byte(f.BodyHash))
	return hex.EncodeToString(h.Sum(nil))
}

// FingerprintRequest creates a RequestFingerprint from the HTTP method,
// path, and body. The body is hashed (SHA-256) so the fingerprint does
// not retain the raw body.
func FingerprintRequest(method, path string, body []byte) RequestFingerprint {
	bodyHash := sha256.Sum256(body)
	return RequestFingerprint{
		Method:   strings.ToUpper(method),
		Path:     path,
		BodyHash: hex.EncodeToString(bodyHash[:]),
	}
}

// RecordState represents the lifecycle state of an idempotency record.
type RecordState int

const (
	// StateInProgress indicates the request is being processed.
	// A concurrent request with the same key should receive 409 Conflict.
	StateInProgress RecordState = iota

	// StateCompleted indicates the request finished successfully and
	// the stored response should be replayed.
	StateCompleted

	// StateFailed indicates the request failed and should not be
	// replayed (the client should retry with the same key).
	StateFailed
)

// Record represents a stored idempotency record. The application is
// responsible for persisting this; the reusable library defines the
// contract.
type Record struct {
	// Key is the idempotency key (already validated).
	Key string

	// Fingerprint is the request fingerprint.
	Fingerprint RequestFingerprint

	// State is the current lifecycle state.
	State RecordState

	// ResponseStatus is the HTTP status of the completed response.
	// Valid only when State == StateCompleted.
	ResponseStatus int

	// ResponseHeaders are the non-sensitive response headers to replay.
	// Sensitive headers (Set-Cookie, Authorization) should be filtered
	// by the application before storage.
	ResponseHeaders http.Header

	// ResponseBody is the response body to replay.
	// Valid only when State == StateCompleted.
	ResponseBody []byte

	// CreatedAt is when the record was created.
	CreatedAt time.Time

	// ExpiresAt is when the record should be evicted.
	ExpiresAt time.Time
}

// IsExpired reports whether the record has expired relative to now.
func (r *Record) IsExpired(now time.Time) bool {
	return !r.ExpiresAt.IsZero() && now.After(r.ExpiresAt)
}

// Store is the contract for idempotency record storage. The application
// implements this interface using its preferred durable storage
// (Redis, database, etc.). The reusable library does not provide a
// production Store implementation.
//
// All methods must be safe for concurrent use. The Reserve operation
// must be atomic to prevent races.
type Store interface {
	// Reserve attempts to create an in-progress record for the given
	// key and fingerprint. If a record already exists:
	//   - If it is in-progress, return the existing record and
	//     ErrConflict.
	//   - If it is completed, return the existing record and
	//     ErrReplayAvailable.
	//   - If it is expired, the implementation should evict it and
	//     allow the reservation.
	//   - If the fingerprint does not match, return
	//     ErrFingerprintMismatch.
	Reserve(key string, fingerprint RequestFingerprint, ttl time.Duration) (existing *Record, err error)

	// Complete marks an in-progress record as completed with the
	// given response. The response headers and body are stored for
	// replay. Sensitive headers must be filtered by the caller before
	// storage.
	Complete(key string, status int, headers http.Header, body []byte) error

	// Fail marks an in-progress record as failed. The record may be
	// retried by the client with the same key.
	Fail(key string) error

	// Get retrieves the record for the given key. Returns nil, nil if
	// the key does not exist.
	Get(key string) (*Record, error)
}

// Sentinel errors returned by Store implementations.
var (
	// ErrConflict indicates a record is in-progress for the given key.
	ErrConflict = errors.New("idempotency: conflict (in-progress)")

	// ErrReplayAvailable indicates a completed record is available
	// for replay.
	ErrReplayAvailable = errors.New("idempotency: replay available")
)

// SafeHeaders returns a copy of the given headers with sensitive
// headers removed. This should be called before storing response
// headers in an idempotency record to prevent replaying credentials
// or session tokens.
func SafeHeaders(h http.Header) http.Header {
	safe := make(http.Header)
	for k, vs := range h {
		switch strings.ToLower(k) {
		case "set-cookie", "authorization", "cookie",
			"www-authenticate", "proxy-authenticate",
			"proxy-authorization":
			continue
		default:
			safe[k] = append([]string(nil), vs...)
		}
	}
	return safe
}

// IsSafeMethod reports whether the HTTP method is idempotent by
// default (GET, HEAD, PUT, DELETE). POST and PATCH require an
// idempotency key to be safely retried.
func IsSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "PUT", "DELETE":
		return true
	default:
		return false
	}
}

// RequiresIdempotencyKey reports whether the HTTP method requires an
// idempotency key for safe retry. POST and PATCH return true; all
// others return false.
func RequiresIdempotencyKey(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PATCH":
		return true
	default:
		return false
	}
}
