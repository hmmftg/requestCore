package request

import (
	"errors"
	"io"
	"sync"
)

// BodySource provides lazy, bounded, single-read access to the request
// body. It is consumed exactly once; subsequent reads return the cached
// result. Adapters that have access to the raw request body reader
// (e.g. *http.Request.Body) supply a BodySource instead of eagerly
// buffering the entire body as a string.
//
// A nil BodySource (or a BodySource that returns io.EOF on first read)
// is treated as "no body" by the binding package.
type BodySource interface {
	// Read returns the body bytes up to maxBytes. If the body exceeds
	// maxBytes, Read returns ErrBodyTooLarge without returning any
	// bytes. If the body has already been consumed, Read returns the
	// cached result from the first call. A maxBytes of 0 means no
	// limit.
	Read(maxBytes int64) ([]byte, error)
}

// ErrBodyTooLarge is returned by BodySource.Read when the body exceeds
// the requested limit.
var ErrBodyTooLarge = errors.New("request: body too large")

// bodySource wraps an io.Reader and provides lazy, bounded, single-read
// access. It is safe for concurrent use.
type bodySource struct {
	mu       sync.Mutex
	reader   io.Reader
	consumed bool
	cached   []byte
	cacheErr error
}

// NewBodySource creates a BodySource from an io.Reader. The reader is
// not read until Read is called. If reader is nil, Read returns
// (nil, nil) indicating no body.
func NewBodySource(r io.Reader) BodySource {
	if r == nil {
		return &emptyBodySource{}
	}
	return &bodySource{reader: r}
}

// Read implements BodySource.
func (b *bodySource) Read(maxBytes int64) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.consumed {
		return b.cached, b.cacheErr
	}
	b.consumed = true

	if maxBytes > 0 {
		b.reader = io.LimitReader(b.reader, maxBytes+1)
	}
	data, err := io.ReadAll(b.reader)
	if err != nil {
		b.cacheErr = err
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		b.cacheErr = ErrBodyTooLarge
		return nil, ErrBodyTooLarge
	}
	b.cached = data
	return data, nil
}

// emptyBodySource is a BodySource with no body.
type emptyBodySource struct{}

func (emptyBodySource) Read(int64) ([]byte, error) { return nil, nil }

// stringBodySource wraps a pre-loaded string body. It is used by the
// fake transport and tests that set the body via WithBody.
type stringBodySource struct {
	once   sync.Once
	body   string
	cached []byte
}

// NewStringBodySource creates a BodySource from a pre-loaded string.
func NewStringBodySource(body string) BodySource {
	return &stringBodySource{body: body}
}

func (s *stringBodySource) Read(maxBytes int64) ([]byte, error) {
	s.once.Do(func() {
		s.cached = []byte(s.body)
	})
	if maxBytes > 0 && int64(len(s.cached)) > maxBytes {
		return nil, ErrBodyTooLarge
	}
	return s.cached, nil
}
