package libCallApi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hmmftg/requestCore/libCallApi"
	"gotest.tools/v3/assert"
)

func TestOAuth2Auth_ClientCredentialsLogin(t *testing.T) {
	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		assert.Equal(t, r.Method, http.MethodPost)
		err := r.ParseForm()
		assert.NilError(t, err)
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"scope":        "api.read",
		})
	}))
	defer server.Close()

	auth, err := libCallApi.NewOAuth2AuthFromAuthData(libCallApi.Auth{
		GrantType:    libCallApi.GrantTypeClientCredentials,
		AuthURI:      server.URL + "/token",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, server.Client())
	assert.NilError(t, err)

	cache, loginErr := auth.Login(context.Background())
	assert.NilError(t, loginErr)
	assert.Equal(t, tokenCalls.Load(), int32(1))
	assert.Equal(t, cache.AccessToken.Token, "test-access-token")
	assert.Equal(t, cache.AccessToken.Type, "Bearer")
	assert.Equal(t, cache.AccessToken.Scope, "api.read")
	assert.Assert(t, cache.AccessToken.ValidUntil > 0)
	assert.Assert(t, !cache.Expired())
}

func TestOAuth2Auth_RefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		assert.NilError(t, err)
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "old-refresh", r.Form.Get("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "new-refresh",
		})
	}))
	defer server.Close()

	auth, err := libCallApi.NewOAuth2AuthFromAuthData(libCallApi.Auth{
		GrantType:    libCallApi.GrantTypeClientCredentials,
		AuthURI:      server.URL + "/token",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, server.Client())
	assert.NilError(t, err)

	cache, refreshErr := auth.Refresh(context.Background(), "old-refresh")
	assert.NilError(t, refreshErr)
	assert.Equal(t, cache.AccessToken.Token, "new-access-token")
	assert.Equal(t, cache.RefreshToken.Token, "new-refresh")
}

func TestNewOAuth2AuthFromAuthData_Validation(t *testing.T) {
	_, err := libCallApi.NewOAuth2AuthFromAuthData(libCallApi.Auth{}, nil)
	assert.ErrorContains(t, err, "grant-type is required")

	_, err = libCallApi.NewOAuth2AuthFromAuthData(libCallApi.Auth{
		GrantType: libCallApi.GrantTypeClientCredentials,
	}, nil)
	assert.ErrorContains(t, err, "auth-uri is required")
}
