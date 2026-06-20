package libCallApi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/webFramework"
	"gotest.tools/v3/assert"
)

type countingAuth struct {
	logins    atomic.Int32
	refreshes atomic.Int32
}

func (a *countingAuth) Login(w webFramework.WebFramework) (*libCallApi.TokenCache, libError.Error) {
	a.logins.Add(1)
	return &libCallApi.TokenCache{
		AccessToken: &libCallApi.OAuth2Token{
			Token:      "access-token",
			Type:       "Bearer",
			TimeTaken:  time.Now(),
			ValidUntil: time.Hour,
		},
	}, nil
}

func (a *countingAuth) Refresh(w webFramework.WebFramework, refreshToken string) (*libCallApi.TokenCache, libError.Error) {
	a.refreshes.Add(1)
	return &libCallApi.TokenCache{
		AccessToken: &libCallApi.OAuth2Token{
			Token:      "refreshed-access-token",
			Type:       "Bearer",
			TimeTaken:  time.Now(),
			ValidUntil: time.Hour,
		},
	}, nil
}

func TestAuthenticate_CacheHitAvoidsSecondLogin(t *testing.T) {
	auth := &countingAuth{}
	cache, lock := libCallApi.InitTokenCache()
	api := &libCallApi.RemoteApi{
		Name:           "test-api",
		Auth:           auth,
		TokenCache:     cache,
		TokenCacheLock: lock,
	}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)

	err := api.Authenticate(w)
	assert.NilError(t, err)
	err = api.Authenticate(w)
	assert.NilError(t, err)
	assert.Equal(t, auth.logins.Load(), int32(1))
}

func TestAuthenticate_ExpiredTokenTriggersRefresh(t *testing.T) {
	auth := &countingAuth{}
	cache, lock := libCallApi.InitTokenCache()
	cache.AccessToken = &libCallApi.OAuth2Token{
		Token:      "expired-token",
		Type:       "Bearer",
		TimeTaken:  time.Now().Add(-2 * time.Hour),
		ValidUntil: time.Hour,
	}
	cache.RefreshToken = &libCallApi.OAuth2Token{
		Token:      "refresh-token",
		TimeTaken:  time.Now(),
		ValidUntil: time.Hour,
	}
	api := &libCallApi.RemoteApi{
		Name:           "test-api",
		Auth:           auth,
		TokenCache:     cache,
		TokenCacheLock: lock,
	}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)

	err := api.Authenticate(w)
	assert.NilError(t, err)
	assert.Equal(t, auth.refreshes.Load(), int32(1))
	assert.Equal(t, auth.logins.Load(), int32(0))
	assert.Equal(t, api.TokenCache.AccessToken.Token, "refreshed-access-token")
}

func TestAuthenticate_ConcurrentLoginOnce(t *testing.T) {
	auth := &countingAuth{}
	cache, lock := libCallApi.InitTokenCache()
	api := &libCallApi.RemoteApi{
		Name:           "test-api",
		Auth:           auth,
		TokenCache:     cache,
		TokenCacheLock: lock,
	}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := api.Authenticate(w)
			assert.NilError(t, err)
		}()
	}
	wg.Wait()
	assert.Equal(t, auth.logins.Load(), int32(1))
}

func TestEnsureAuthorization_PreservesExplicitHeader(t *testing.T) {
	api := &libCallApi.RemoteApi{
		Name: "test-api",
		Auth: &countingAuth{},
	}
	headers := map[string]string{
		"Authorization": "Bearer explicit-token",
	}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)
	err := api.EnsureAuthorization(w, headers)
	assert.NilError(t, err)
	assert.Equal(t, headers["Authorization"], "Bearer explicit-token")
}

func TestEnsureAuthorization_BasicAuthFallback(t *testing.T) {
	api := &libCallApi.RemoteApi{
		Name: "test-api",
		AuthData: libCallApi.Auth{
			User:     "user",
			Password: "pass",
		},
	}
	headers := map[string]string{}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)
	err := api.EnsureAuthorization(w, headers)
	assert.NilError(t, err)
	assert.Equal(t, headers["Authorization"], api.GetBasicAuthHeader())
}

func TestPrepareCall_OAuthAuthorizationHeader(t *testing.T) {
	var tokenCalls atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "oauth-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer oauth-access-token")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer apiServer.Close()

	auth, err := libCallApi.NewOAuth2AuthFromAuthData(libCallApi.Auth{
		GrantType:    libCallApi.GrantTypeClientCredentials,
		AuthURI:      tokenServer.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, tokenServer.Client())
	assert.NilError(t, err)

	cache, lock := libCallApi.InitTokenCache()
	remoteApi := libCallApi.RemoteApi{
		Name:           "partner-api",
		Domain:         apiServer.URL,
		Auth:           auth,
		TokenCache:     cache,
		TokenCacheLock: lock,
	}

	callData := libCallApi.CallData[map[string]string]{
		Api:      remoteApi,
		Path:     "",
		Method:   http.MethodGet,
		Headers:  map[string]string{},
		BodyType: libCallApi.Empty,
		Context:  context.Background(),
	}

	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	w := libContext.InitContextNoAuditTrail(t)

	req, err := libCallApi.PrepareCall(w, callData)
	assert.NilError(t, err)
	assert.Equal(t, req.Header.Get("Authorization"), "Bearer oauth-access-token")
}
