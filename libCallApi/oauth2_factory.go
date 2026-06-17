package libCallApi

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	GrantTypeClientCredentials = "client_credentials"
	GrantTypePassword          = "password"
)

func NewTokenHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultTimeOut,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func NewOAuth2AuthFromAuthData(auth Auth, httpClient *http.Client) (AuthSystem, error) {
	if auth.GrantType == "" {
		return nil, fmt.Errorf("grant-type is required")
	}
	if auth.AuthURI == "" {
		return nil, fmt.Errorf("auth-uri is required")
	}
	if auth.ClientID == "" {
		return nil, fmt.Errorf("client-id is required")
	}
	if auth.ClientSecret == "" {
		return nil, fmt.Errorf("client-secret is required")
	}
	if auth.GrantType == GrantTypePassword && (auth.User == "" || auth.Password == "") {
		return nil, fmt.Errorf("user and password are required for password grant")
	}
	if httpClient == nil {
		httpClient = NewTokenHTTPClient()
	}

	return OAuth2Auth{
		grantType: auth.GrantType,
		user:      auth.User,
		password:  auth.Password,
		cfg: oauth2.Config{
			ClientID:     auth.ClientID,
			ClientSecret: auth.ClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: auth.AuthURI,
			},
		},
		httpClient: httpClient,
	}, nil
}
