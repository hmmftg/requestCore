package session

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// generateID returns a cryptographically random 16-byte hex string.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("session: crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x", b)
}

// cookieTokenVersion is the current token format version. It is embedded
// in every saved token and verified on load so that future format changes
// (e.g. new encryption schemes, key rotation markers) can be distinguished
// safely. Tokens with an unknown version are rejected on load.
const cookieTokenVersion = 1

// CookieStoreConfig configures a CookieStore.
type CookieStoreConfig struct {
	// SecretKey is the HMAC signing key. Must be at least 32 bytes.
	SecretKey []byte

	// VerificationKeys are additional keys for key rotation.
	// The store verifies against SecretKey first, then each verification key.
	VerificationKeys [][]byte

	// EncryptionKey, when set (must be 32 bytes), enables AES-GCM encryption.
	// When nil, the payload is signed but readable by the client.
	EncryptionKey []byte

	// CookieName is the HTTP cookie name. Default: "requestcore_session".
	CookieName string

	// MaxAge is the session lifetime. Default: 24 hours.
	MaxAge time.Duration

	// Path is the cookie path. Default: "/".
	Path string

	// Domain is the cookie domain.
	Domain string

	// Secure sets the cookie Secure flag. Default: false (set true in production).
	Secure bool

	// HttpOnly sets the cookie HttpOnly flag. Default: true.
	// Use a pointer to distinguish "not set" (nil → defaults to true)
	// from "explicitly set to false" (allows disabling HttpOnly for
	// cross-site JavaScript access when needed).
	HttpOnly *bool

	// SameSite sets the cookie SameSite attribute.
	// Values: "lax", "strict", "none". Default: "lax".
	SameSite string

	// MaxPayloadSize is the maximum encoded cookie payload size in bytes.
	// Default: 3800 (safe for most browsers).
	MaxPayloadSize int
}

// CookieStore implements Store using HMAC-signed cookies.
type CookieStore struct {
	config CookieStoreConfig
}

// NewCookieStore creates a CookieStore with the given configuration.
// Returns an error if SecretKey is shorter than 32 bytes.
func NewCookieStore(config CookieStoreConfig) (*CookieStore, error) {
	if len(config.SecretKey) < 32 {
		return nil, errors.New("session: secret key must be at least 32 bytes")
	}
	if config.EncryptionKey != nil && len(config.EncryptionKey) != 32 {
		return nil, errors.New("session: encryption key must be exactly 32 bytes")
	}
	if config.CookieName == "" {
		config.CookieName = "requestcore_session"
	}
	if config.MaxAge == 0 {
		config.MaxAge = 24 * time.Hour
	}
	if config.Path == "" {
		config.Path = "/"
	}
	if config.SameSite == "" {
		config.SameSite = "lax"
	}
	// HttpOnly defaults to true when nil. An explicit *false honors
	// the caller's intent to disable HttpOnly (e.g. for cross-site
	// JavaScript cookie access).
	if config.HttpOnly == nil {
		t := true
		config.HttpOnly = &t
	}
	if config.MaxPayloadSize == 0 {
		config.MaxPayloadSize = 3800
	}
	return &CookieStore{config: config}, nil
}

// Load decodes and verifies a signed cookie token into a Session.
func (s *CookieStore) Load(_ context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, errors.New("session: empty token")
	}

	payload, err := s.verifyAndDecode(token)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Version   int            `json:"v"`
		ID        string         `json:"id"`
		Data      map[string]any `json:"data"`
		CreatedAt time.Time      `json:"created_at"`
		ExpiresAt time.Time      `json:"expires_at"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("session: unmarshal payload: %w", err)
	}

	// Reject unknown token versions so future format changes can be
	// distinguished safely during key rotation.
	if raw.Version != cookieTokenVersion {
		return nil, fmt.Errorf("session: unsupported token version %d, expected %d", raw.Version, cookieTokenVersion)
	}

	if time.Now().After(raw.ExpiresAt) {
		return nil, errors.New("session: token expired")
	}

	return &Session{
		id:        raw.ID,
		data:      raw.Data,
		store:     s,
		createdAt: raw.CreatedAt,
	}, nil
}

// Save encodes and signs a Session into an opaque cookie token.
func (s *CookieStore) Save(_ context.Context, sess *Session) (string, error) {
	expiresAt := time.Now().Add(s.config.MaxAge)
	raw := struct {
		Version   int            `json:"v"`
		ID        string         `json:"id"`
		Data      map[string]any `json:"data"`
		CreatedAt time.Time      `json:"created_at"`
		ExpiresAt time.Time      `json:"expires_at"`
	}{
		Version:   cookieTokenVersion,
		ID:        sess.id,
		Data:      sess.Data(),
		CreatedAt: sess.createdAt,
		ExpiresAt: expiresAt,
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("session: marshal payload: %w", err)
	}

	token, err := s.signAndEncode(payload)
	if err != nil {
		return "", err
	}

	if len(token) > s.config.MaxPayloadSize {
		return "", fmt.Errorf("session: encoded payload %d bytes exceeds max %d", len(token), s.config.MaxPayloadSize)
	}

	return token, nil
}

// Delete is a no-op for CookieStore; the caller should expire the cookie.
func (s *CookieStore) Delete(_ context.Context, _ string) error {
	return nil
}

// Config returns the store configuration.
func (s *CookieStore) Config() CookieStoreConfig {
	return s.config
}

func (s *CookieStore) signAndEncode(payload []byte) (string, error) {
	// If encryption is enabled, encrypt the payload with AES-GCM before
	// signing. The nonce is prepended to the ciphertext.
	var signedPayload []byte
	if s.config.EncryptionKey != nil {
		encrypted, err := s.encrypt(payload)
		if err != nil {
			return "", fmt.Errorf("session: encrypt: %w", err)
		}
		signedPayload = encrypted
	} else {
		signedPayload = payload
	}

	mac := hmac.New(sha256.New, s.config.SecretKey)
	mac.Write(signedPayload)
	signature := mac.Sum(nil)

	combined := make([]byte, 0, len(signedPayload)+len(signature))
	combined = append(combined, signedPayload...)
	combined = append(combined, signature...)

	return base64.URLEncoding.EncodeToString(combined), nil
}

func (s *CookieStore) verifyAndDecode(token string) ([]byte, error) {
	combined, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("session: base64 decode: %w", err)
	}

	sigLen := sha256.Size
	if len(combined) < sigLen {
		return nil, errors.New("session: token too short")
	}

	signedPayload := combined[:len(combined)-sigLen]
	signature := combined[len(combined)-sigLen:]

	if !s.verifySignature(signedPayload, signature) {
		return nil, errors.New("session: invalid signature")
	}

	// If encryption is enabled, decrypt the payload.
	if s.config.EncryptionKey != nil {
		plaintext, err := s.decrypt(signedPayload)
		if err != nil {
			return nil, fmt.Errorf("session: decrypt: %w", err)
		}
		return plaintext, nil
	}

	return signedPayload, nil
}

// encrypt encrypts plaintext using AES-GCM with the configured EncryptionKey.
// The nonce is prepended to the ciphertext.
func (s *CookieStore) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.config.EncryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// nonce is prepended to ciphertext
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// decrypt decrypts an AES-GCM ciphertext (nonce prepended) using the
// configured EncryptionKey.
func (s *CookieStore) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.config.EncryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("session: ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	ct := ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

func (s *CookieStore) verifySignature(payload, signature []byte) bool {
	keys := make([][]byte, 0, 1+len(s.config.VerificationKeys))
	keys = append(keys, s.config.SecretKey)
	keys = append(keys, s.config.VerificationKeys...)

	for _, key := range keys {
		mac := hmac.New(sha256.New, key)
		mac.Write(payload)
		expected := mac.Sum(nil)
		if subtle.ConstantTimeCompare(signature, expected) == 1 {
			return true
		}
	}
	return false
}
