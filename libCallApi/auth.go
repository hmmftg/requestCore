package libCallApi

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

// Auth holds OAuth2/basic authentication credentials and configuration.
type Auth struct {
	GrantType    string `yaml:"grant-type"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	ClientID     string `yaml:"client-id"`
	ClientSecret string `yaml:"client-secret"`
	AuthURI      string `yaml:"auth-uri"`
}

// OAuth2Token represents an OAuth2 access or refresh token with validity timing.
type OAuth2Token struct {
	Token      string
	Type       string
	Scope      string
	TimeTaken  time.Time
	ValidUntil time.Duration
}

// TokenCache stores the current access and refresh tokens for a remote API.
type TokenCache struct {
	AccessToken  *OAuth2Token
	RefreshToken *OAuth2Token
}

// Expired reports whether the cached access token has expired.
func (t TokenCache) Expired() bool {
	if t.AccessToken == nil {
		return true
	}
	return time.Now().After(t.AccessToken.TimeTaken.Add(t.AccessToken.ValidUntil))
}

// InitTokenCache initializes a token cache which will be used across all APIs.
// It should be called once per remote-api.
func InitTokenCache() (*TokenCache, *sync.Mutex) {
	return &TokenCache{}, &sync.Mutex{}
}

// AuthSystem defines the interface for login and token refresh operations.
type AuthSystem interface {
	Login(w webFramework.WebFramework) (*TokenCache, libError.Error)
	Refresh(w webFramework.WebFramework, refreshToken string) (*TokenCache, libError.Error)
}

// GetBasicAuthHeader returns the Basic authentication header value for this API.
func (api RemoteAPI) GetBasicAuthHeader() string {
	usr := fmt.Sprintf("%s:%s", api.AuthData.User, api.AuthData.Password)
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(usr)))
}

// AddBasicAuthHeader sets the Basic Authorization header in the given map and returns it.
func (api RemoteAPI) AddBasicAuthHeader(headers map[string]string) map[string]string {
	headers["Authorization"] = api.GetBasicAuthHeader()
	return headers
}

// GetAuthHeader returns the bearer-style Authorization header from the cached token.
func (api RemoteAPI) GetAuthHeader() (string, error) {
	if api.TokenCache == nil {
		return "", fmt.Errorf("empty token cache")
	}
	if api.TokenCache.AccessToken == nil || len(api.TokenCache.AccessToken.Token) == 0 {
		return "", fmt.Errorf("empty token data")
	}
	if api.TokenCache.Expired() {
		return "", fmt.Errorf("expired token")
	}
	return fmt.Sprintf("%s %s", api.TokenCache.AccessToken.Type, api.TokenCache.AccessToken.Token), nil
}

func (api *RemoteAPI) handleToken(w webFramework.WebFramework) libError.Error {
	api.TokenCacheLock.Lock()
	defer api.TokenCacheLock.Unlock()

	if api.TokenCache.AccessToken != nil && !api.TokenCache.Expired() {
		return nil
	}

	if api.Auth == nil {
		return libError.NewWithDescription(
			status.InternalServerError,
			"AUTH_SYSTEM_NOT_CONFIGURED",
			"auth system of api %s is not configured",
			api.Name,
		)
	}

	if api.TokenCache.RefreshToken != nil && api.TokenCache.RefreshToken.Token != "" {
		tokens, err := api.Auth.Refresh(w, api.TokenCache.RefreshToken.Token)
		if err == nil {
			api.TokenCache.AccessToken = tokens.AccessToken
			if tokens.RefreshToken != nil {
				api.TokenCache.RefreshToken = tokens.RefreshToken
			}
			return nil
		}
	}

	api.TokenCache.AccessToken = nil
	api.TokenCache.RefreshToken = nil

	tokens, err := api.Auth.Login(w)
	if err != nil {
		return err
	}
	api.TokenCache.AccessToken = tokens.AccessToken
	api.TokenCache.RefreshToken = tokens.RefreshToken
	return nil
}

// Authenticate ensures the API has a valid access token, refreshing or logging in as needed.
func (api *RemoteAPI) Authenticate(w webFramework.WebFramework) libError.Error {
	if api.TokenCacheLock == nil {
		return libError.NewWithDescription(status.InternalServerError, "TOKEN_CACHE_NOT_INITIALIZED", "token cache lock of api %s is null", api.Name)
	}
	if api.TokenCache == nil {
		return libError.NewWithDescription(status.InternalServerError, "TOKEN_CACHE_NOT_INITIALIZED", "token cache of api %s is null", api.Name)
	}
	if api.TokenCache.AccessToken == nil || api.TokenCache.Expired() {
		return api.handleToken(w)
	}
	return nil
}
