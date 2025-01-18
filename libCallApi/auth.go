package libCallApi

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
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

// initilaizes a token cache which will be used across all APIs
// should be called once per remote-api
func InitTokenCache() (*TokenCache, *sync.Mutex) {
	return &TokenCache{}, &sync.Mutex{}
}

type AuthSystem interface {
	Login() (*TokenCache, libError.Error)
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
	if time.Now().After(api.TokenCache.AccessToken.TimeTaken.Add(api.TokenCache.AccessToken.ValidUntil)) {
		return "", fmt.Errorf("expired token")
	}
	return fmt.Sprintf("%s %s", api.TokenCache.AccessToken.Type, api.TokenCache.AccessToken.Token), nil
}

func (api *RemoteApi) handleToken() libError.Error {
	api.TokenCacheLock.Lock()
	defer api.TokenCacheLock.Unlock()
	if api.TokenCache.AccessToken != nil && api.TokenCache.RefreshToken != nil {
		return nil
	}
	tokens, err := api.Auth.Login()
	if err != nil {
		return err
	}
	api.TokenCache = tokens
	return nil
}

func (api *RemoteApi) Authenticate() libError.Error {
	if api.TokenCacheLock == nil {
		return libError.New(status.InternalServerError, "TOKEN_CACHE_NOT_INITIALIZED", "token cache lock of api %s is null", api.Name)
	}
	if api.TokenCache.AccessToken == nil {
		err := api.handleToken()
		if err != nil {
			return err
		}
	}
	if time.Since(api.TokenCache.AccessToken.TimeTaken) >= api.TokenCache.AccessToken.ValidUntil {
		err := api.handleToken()
		if err != nil {
			return err
		}
	}
	return nil
}
