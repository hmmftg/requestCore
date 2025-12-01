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

type Auth struct {
	GrantType    string `yaml:"grant-type"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	ClientID     string `yaml:"client-id"`
	ClientSecret string `yaml:"client-secret"`
	AuthURI      string `yaml:"auth-uri"`
}

type OAuth2Token struct {
	Token      string
	Type       string
	Scope      string
	TimeTaken  time.Time
	ValidUntil time.Duration
}

type TokenCache struct {
	AccessToken  *OAuth2Token
	RefreshToken *OAuth2Token
}

func (t TokenCache) Expired() bool {
	if t.AccessToken == nil {
		return true
	}
	return time.Now().After(t.AccessToken.TimeTaken.Add(t.AccessToken.ValidUntil))
}

// initilaizes a token cache which will be used across all APIs
// should be called once per remote-api
func InitTokenCache() (*TokenCache, *sync.Mutex) {
	return &TokenCache{}, &sync.Mutex{}
}

type AuthSystem interface {
	Login(w webFramework.WebFramework) (*TokenCache, libError.Error)
}

func (api RemoteApi) GetBasicAuthHeader() string {
	usr := fmt.Sprintf("%s:%s", api.AuthData.User, api.AuthData.Password)
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(usr)))
}

func (api RemoteApi) AddBasicAuthHeader(headers map[string]string) map[string]string {
	headers["Authorization"] = api.GetBasicAuthHeader()
	return headers
}

func (api RemoteApi) GetAuthHeader() (string, error) {
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

func (api *RemoteApi) handleToken(w webFramework.WebFramework) libError.Error {
	api.TokenCacheLock.Lock()
	defer api.TokenCacheLock.Unlock()

	if api.TokenCache.AccessToken != nil && !api.TokenCache.Expired() {
		// another thread handles login before us
		return nil
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

func (api *RemoteApi) Authenticate(w webFramework.WebFramework) libError.Error {
	if api.TokenCacheLock == nil {
		return libError.NewWithDescription(status.InternalServerError, "TOKEN_CACHE_NOT_INITIALIZED", "token cache lock of api %s is null", api.Name)
	}
	if api.TokenCache.AccessToken == nil {
		err := api.handleToken(w)
		if err != nil {
			return err
		}
	}
	if api.TokenCache.Expired() {
		err := api.handleToken(w)
		if err != nil {
			return err
		}
	}
	return nil
}
