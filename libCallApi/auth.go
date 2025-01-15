package libCallApi

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/hmmftg/requestCore/libError"
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

func (api *RemoteApi) handleToken() libError.Error {
	api.TokenCacheLock.Lock()
	if api.TokenCache.AccessToken != nil && api.TokenCache.RefreshToken != nil {
		return nil
	}
	tokens, err := api.Auth.Login()
	if err != nil {
		return err
	}
	api.TokenCache = tokens
	api.TokenCacheLock.Unlock()
	return nil
}

func (api *RemoteApi) Authenticate() libError.Error {
	if api.TokenCacheLock == nil {
		log.Fatalf("token cache lock of api %s is null", api.Name)
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
