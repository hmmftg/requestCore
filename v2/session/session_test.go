package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func mustSecretKey(t *testing.T, n int) []byte {
	t.Helper()
	key := make([]byte, n)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return key
}

func TestSession_SetGet(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("foo", "bar")
	if v := s.GetString("foo"); v != "bar" {
		t.Fatalf("expected 'bar', got %q", v)
	}
	if v := s.GetString("missing"); v != "" {
		t.Fatalf("expected empty string, got %q", v)
	}
}

func TestSession_GetNonString(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("count", 42)
	if v := s.GetString("count"); v != "" {
		t.Fatalf("expected empty for non-string, got %q", v)
	}
	if v := s.Get("count"); v != 42 {
		t.Fatalf("expected 42, got %v", v)
	}
}

func TestSession_GetInto(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("user", map[string]any{"name": "alice"})
	var result struct {
		Name string `json:"name"`
	}
	if err := s.GetInto("user", &result); err != nil {
		t.Fatalf("GetInto: %v", err)
	}
	if result.Name != "alice" {
		t.Fatalf("expected 'alice', got %q", result.Name)
	}
}

func TestSession_GetIntoMissing(t *testing.T) {
	s := NewSession(NoOpStore{})
	var result string
	if err := s.GetInto("missing", &result); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestSession_Delete(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("foo", "bar")
	s.Delete("foo")
	if v := s.Get("foo"); v != nil {
		t.Fatalf("expected nil after delete, got %v", v)
	}
	if !s.IsDirty() {
		t.Fatal("expected dirty after delete")
	}
}

func TestSession_Clear(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("a", 1)
	s.Set("b", 2)
	s.Clear()
	if v := s.Get("a"); v != nil {
		t.Fatalf("expected nil after clear, got %v", v)
	}
}

func TestSession_Data(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("a", 1)
	s.Set("b", 2)
	data := s.Data()
	if len(data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(data))
	}
	data["c"] = 3
	if s.Get("c") != nil {
		t.Fatal("Data() should return a copy")
	}
}

func TestFlash_AddGet(t *testing.T) {
	f := NewFlash()
	f.Add("success", "Saved!")
	if v := f.Get("success"); v != "Saved!" {
		t.Fatalf("expected 'Saved!', got %q", v)
	}
	if v := f.Get("success"); v != "" {
		t.Fatalf("expected empty after consume, got %q", v)
	}
}

func TestFlash_Peek(t *testing.T) {
	f := NewFlash()
	f.Add("info", "hello")
	if v := f.Peek("info"); v != "hello" {
		t.Fatalf("expected 'hello', got %q", v)
	}
	if v := f.Peek("info"); v != "hello" {
		t.Fatalf("Peek should not consume, got %q", v)
	}
}

func TestFlash_Has(t *testing.T) {
	f := NewFlash()
	if f.Has("missing") {
		t.Fatal("expected false for missing key")
	}
	f.Add("x", "y")
	if !f.Has("x") {
		t.Fatal("expected true for existing key")
	}
}

func TestFlash_GetAll(t *testing.T) {
	f := NewFlash()
	f.Add("a", "1")
	f.Add("b", "2")
	all := f.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	if f.Has("a") {
		t.Fatal("expected GetAll to consume entries")
	}
}

func TestFlash_Clear(t *testing.T) {
	f := NewFlash()
	f.Add("a", "1")
	f.Clear()
	if f.Has("a") {
		t.Fatal("expected Clear to consume entries")
	}
}

func TestFlash_ActiveEntries(t *testing.T) {
	f := NewFlash()
	f.Add("a", "1")
	f.Add("b", "2")
	_ = f.Get("a")
	active := f.ActiveEntries()
	if len(active) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(active))
	}
	if _, ok := active["b"]; !ok {
		t.Fatal("expected 'b' to be active")
	}
}

func TestFlash_LoadFromSession(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set(flashKey, map[string]string{"success": "ok"})
	f := LoadFlashFromSession(s)
	if v := f.Peek("success"); v != "ok" {
		t.Fatalf("expected 'ok', got %q", v)
	}
}

func TestFlash_SaveFlashToSession(t *testing.T) {
	s := NewSession(NoOpStore{})
	f := NewFlash()
	f.Add("error", "failed")
	SaveFlashToSession(s, f)
	raw := s.Get(flashKey)
	entries, ok := raw.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", raw)
	}
	if entries["error"] != "failed" {
		t.Fatalf("expected 'failed', got %q", entries["error"])
	}
}

func TestCookieStore_ShortKey(t *testing.T) {
	_, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("short"),
	})
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestCookieStore_BadEncryptionKey(t *testing.T) {
	_, err := NewCookieStore(CookieStoreConfig{
		SecretKey:     mustSecretKey(t, 32),
		EncryptionKey: []byte("short"),
	})
	if err == nil {
		t.Fatal("expected error for bad encryption key length")
	}
}

func TestCookieStore_RoundTrip(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: mustSecretKey(t, 32),
		MaxAge:    time.Hour,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	sess.Set("user_id", "12345")
	sess.Set("role", "admin")

	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	loaded, err := store.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.GetString("user_id") != "12345" {
		t.Fatalf("expected user_id '12345', got %q", loaded.GetString("user_id"))
	}
	if loaded.GetString("role") != "admin" {
		t.Fatalf("expected role 'admin', got %q", loaded.GetString("role"))
	}
}

func TestCookieStore_TamperedToken(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: mustSecretKey(t, 32),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	sess.Set("data", "secret")
	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	tampered := token[:len(token)-4] + "AAAA"
	_, err = store.Load(context.Background(), tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestCookieStore_EmptyToken(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: mustSecretKey(t, 32),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	_, err = store.Load(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestCookieStore_KeyRotation(t *testing.T) {
	oldKey := mustSecretKey(t, 32)
	newKey := mustSecretKey(t, 32)

	oldStore, err := NewCookieStore(CookieStoreConfig{SecretKey: oldKey})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	sess := NewSession(oldStore)
	sess.Set("data", "value")
	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	newStore, err := NewCookieStore(CookieStoreConfig{
		SecretKey:        newKey,
		VerificationKeys: [][]byte{oldKey},
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	loaded, err := newStore.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("Load with rotated key: %v", err)
	}
	if loaded.GetString("data") != "value" {
		t.Fatalf("expected 'value', got %q", loaded.GetString("data"))
	}
}

func TestCookieStore_MaxPayloadSize(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey:      mustSecretKey(t, 32),
		MaxPayloadSize: 100,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	largeData := make(map[string]any)
	for i := 0; i < 50; i++ {
		largeData[string(rune('a'+i))] = string(make([]byte, 100))
	}
	sess.Set("big", largeData)

	_, err = sess.Save(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func TestCookieStore_ExpiredToken(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: mustSecretKey(t, 32),
		MaxAge:    1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	sess.Set("data", "value")
	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = store.Load(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestCookieStore_Defaults(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: mustSecretKey(t, 32),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	cfg := store.Config()
	if cfg.CookieName != "requestcore_session" {
		t.Fatalf("expected default cookie name, got %q", cfg.CookieName)
	}
	if cfg.MaxAge != 24*time.Hour {
		t.Fatalf("expected default 24h max age, got %v", cfg.MaxAge)
	}
	if cfg.Path != "/" {
		t.Fatalf("expected default path '/', got %q", cfg.Path)
	}
	if !cfg.HttpOnly {
		t.Fatal("expected default HttpOnly=true")
	}
	if cfg.MaxPayloadSize != 3800 {
		t.Fatalf("expected default max payload 3800, got %d", cfg.MaxPayloadSize)
	}
}

func TestManager_LoadFromCookie_Empty(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{SecretKey: mustSecretKey(t, 32)})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	m := NewManager(store)
	sess, flash, err := m.LoadFromCookie(context.Background(), "test_cookie", "")
	if err != nil {
		t.Fatalf("LoadFromCookie: %v", err)
	}
	if sess == nil || flash == nil {
		t.Fatal("expected non-nil session and flash")
	}
	if sess.GetString("anything") != "" {
		t.Fatal("expected empty new session")
	}
}

func TestManager_LoadFromCookie_RoundTrip(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey:  mustSecretKey(t, 32),
		CookieName: "test_session",
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	m := NewManager(store)

	sess := NewSession(store)
	sess.Set("user", "alice")

	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _, err := m.LoadFromCookie(context.Background(), "test_session", token)
	if err != nil {
		t.Fatalf("LoadFromCookie: %v", err)
	}
	if loaded.GetString("user") != "alice" {
		t.Fatalf("expected 'alice', got %q", loaded.GetString("user"))
	}
}

func TestManager_SaveToCookie_NotDirty(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{SecretKey: mustSecretKey(t, 32)})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	m := NewManager(store)
	sess := NewSession(store)
	cookie, err := m.SaveToCookie(context.Background(), sess, nil, "test")
	if err != nil {
		t.Fatalf("SaveToCookie: %v", err)
	}
	if cookie != nil {
		t.Fatal("expected nil cookie for non-dirty session")
	}
}

func TestManager_SaveToCookie_Dirty(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey:  mustSecretKey(t, 32),
		CookieName: "test_session",
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	m := NewManager(store)
	sess := NewSession(store)
	sess.Set("data", "value")
	cookie, err := m.SaveToCookie(context.Background(), sess, nil, "test_session")
	if err != nil {
		t.Fatalf("SaveToCookie: %v", err)
	}
	if cookie == nil {
		t.Fatal("expected non-nil cookie for dirty session")
	}
	if cookie.Name != "test_session" {
		t.Fatalf("expected cookie name 'test_session', got %q", cookie.Name)
	}
	if cookie.Value == "" {
		t.Fatal("expected non-empty cookie value")
	}
}

func TestManager_ExpireCookie(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{SecretKey: mustSecretKey(t, 32)})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	m := NewManager(store)
	cookie := m.ExpireCookie("test_session")
	if cookie.Name != "test_session" {
		t.Fatalf("expected name 'test_session', got %q", cookie.Name)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
}

func TestNoOpStore(t *testing.T) {
	store := NoOpStore{}
	sess, err := store.Load(context.Background(), "any")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	token, err := store.Save(context.Background(), sess)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if err := store.Delete(context.Background(), "any"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestSession_JSONRoundTrip(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("nested", map[string]any{"a": 1, "b": "two"})
	data := s.Data()
	raw, _ := json.Marshal(data)
	var restored map[string]any
	_ = json.Unmarshal(raw, &restored)
	nested, ok := restored["nested"].(map[string]any)
	if !ok {
		t.Fatal("expected nested map")
	}
	if nested["a"].(float64) != 1 {
		t.Fatalf("expected a=1, got %v", nested["a"])
	}
	if nested["b"].(string) != "two" {
		t.Fatalf("expected b=two, got %v", nested["b"])
	}
}

func TestCookieStore_EncryptionRoundTrip(t *testing.T) {
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey:     mustSecretKey(t, 32),
		EncryptionKey: encKey,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	sess.Set("user_id", "alice")
	sess.Set("role", "admin")

	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the token is not readable (encrypted).
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	// The payload portion (before the HMAC signature) should not contain
	// plaintext "alice" or "admin".
	payload := decoded[:len(decoded)-32]
	if bytes.Contains(payload, []byte("alice")) || bytes.Contains(payload, []byte("admin")) {
		t.Fatal("encrypted payload should not contain plaintext user data")
	}

	loaded, err := store.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.GetString("user_id") != "alice" {
		t.Fatalf("expected user_id 'alice', got %q", loaded.GetString("user_id"))
	}
	if loaded.GetString("role") != "admin" {
		t.Fatalf("expected role 'admin', got %q", loaded.GetString("role"))
	}
}

func TestCookieStore_EncryptionTamperedToken(t *testing.T) {
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey:     mustSecretKey(t, 32),
		EncryptionKey: encKey,
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}

	sess := NewSession(store)
	sess.Set("user_id", "alice")

	token, err := sess.Save(context.Background())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Tamper with the token by flipping a character.
	tampered := token[:len(token)-5] + "X" + token[len(token)-4:]
	_, err = store.Load(context.Background(), tampered)
	if err == nil {
		t.Fatal("expected error loading tampered encrypted token")
	}
}

func TestCookieStore_EncryptionKeyWrongSize(t *testing.T) {
	_, err := NewCookieStore(CookieStoreConfig{
		SecretKey:     mustSecretKey(t, 32),
		EncryptionKey: make([]byte, 16), // wrong size
	})
	if err == nil {
		t.Fatal("expected error for 16-byte encryption key")
	}
}

// slowStore simulates a store with latency to exercise the Save snapshot
// race window.
type slowStore struct {
	NoOpStore
	delay time.Duration
}

func (s slowStore) Save(_ context.Context, sess *Session) (string, error) {
	time.Sleep(s.delay)
	return sess.id, nil
}

// TestSession_SaveConcurrentMutation verifies that if a concurrent
// mutation occurs during Save, the dirty flag is NOT cleared — ensuring
// the next Save will persist the new data.
func TestSession_SaveConcurrentMutation(t *testing.T) {
	store := slowStore{delay: 50 * time.Millisecond}
	s := NewSession(store)
	s.Set("k1", "v1")

	// Start Save in a goroutine; it will sleep in the store.
	done := make(chan struct{})
	var token string
	var saveErr error
	go func() {
		token, saveErr = s.Save(context.Background())
		close(done)
	}()

	// While Save is sleeping, mutate the session.
	time.Sleep(10 * time.Millisecond)
	s.Set("k2", "v2")

	<-done
	if saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}
	_ = token

	// The dirty flag should still be set because a concurrent mutation
	// occurred during Save.
	if !s.IsDirty() {
		t.Fatal("expected session to remain dirty after concurrent mutation during Save")
	}
}

// TestSession_SaveNoConcurrentMutation verifies that Save clears the
// dirty flag when no concurrent mutation occurs.
func TestSession_SaveNoConcurrentMutation(t *testing.T) {
	s := NewSession(NoOpStore{})
	s.Set("k1", "v1")
	if !s.IsDirty() {
		t.Fatal("expected dirty after Set")
	}
	if _, err := s.Save(context.Background()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.IsDirty() {
		t.Fatal("expected clean after Save with no concurrent mutation")
	}
}

// TestSession_SaveDoesNotHoldLockDuringStore verifies that the session
// lock is not held during the store's Save call, allowing concurrent
// reads to proceed.
func TestSession_SaveDoesNotHoldLockDuringStore(t *testing.T) {
	store := slowStore{delay: 50 * time.Millisecond}
	s := NewSession(store)
	s.Set("k1", "v1")

	done := make(chan struct{})
	go func() {
		_, _ = s.Save(context.Background())
		close(done)
	}()

	// Wait for Save to enter the store.
	time.Sleep(10 * time.Millisecond)

	// This read should not block because Save should have released the
	// read lock before calling store.Save.
	readDone := make(chan struct{})
	go func() {
		_ = s.GetString("k1")
		close(readDone)
	}()

	select {
	case <-readDone:
		// Success: read completed while Save was in progress.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetString blocked during Save — lock held during store Save")
	}

	<-done
}
