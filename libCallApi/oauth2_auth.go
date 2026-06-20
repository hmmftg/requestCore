package libCallApi

import (
	"context"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const tokenExpirySkew = 30 * time.Second

type OAuth2Auth struct {
	grantType  string
	user       string
	password   string
	cfg        oauth2.Config
	httpClient *http.Client
}

func (a OAuth2Auth) Login(w webFramework.WebFramework) (*TokenCache, libError.Error) {
	switch a.grantType {
	case GrantTypeClientCredentials:
		return a.loginClientCredentials(w)
	case GrantTypePassword:
		return a.loginPassword(w)
	default:
		return nil, libError.NewWithDescription(
			status.InternalServerError,
			"OAUTH2_UNSUPPORTED_GRANT",
			"unsupported grant type %s",
			a.grantType,
		)
	}
}

func (a OAuth2Auth) Refresh(w webFramework.WebFramework, refreshToken string) (*TokenCache, libError.Error) {
	if refreshToken == "" {
		return nil, libError.NewWithDescription(
			status.InternalServerError,
			"OAUTH2_NO_REFRESH_TOKEN",
			"empty refresh token",
		)
	}
	ctx := a.withHTTPClient(w)
	ts := a.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	tok, err := ts.Token()
	if err != nil {
		return nil, libError.NewWithDescription(
			status.InternalServerError,
			"OAUTH2_REFRESH_FAILED",
			"refresh token failed: %v",
			err,
		)
	}
	return oauth2TokenToCache(tok), nil
}

func (a OAuth2Auth) loginClientCredentials(w webFramework.WebFramework) (*TokenCache, libError.Error) {
	cc := &clientcredentials.Config{
		ClientID:     a.cfg.ClientID,
		ClientSecret: a.cfg.ClientSecret,
		TokenURL:     a.cfg.Endpoint.TokenURL,
	}
	ctx := a.withHTTPClient(w)
	tok, err := cc.Token(ctx)
	if err != nil {
		return nil, libError.NewWithDescription(
			status.InternalServerError,
			"OAUTH2_LOGIN_FAILED",
			"client credentials login failed: %v",
			err,
		)
	}
	return oauth2TokenToCache(tok), nil
}

func (a OAuth2Auth) loginPassword(w webFramework.WebFramework) (*TokenCache, libError.Error) {
	ctx := a.withHTTPClient(w)
	tok, err := a.cfg.PasswordCredentialsToken(ctx, a.user, a.password)
	if err != nil {
		return nil, libError.NewWithDescription(
			status.InternalServerError,
			"OAUTH2_LOGIN_FAILED",
			"password grant login failed: %v",
			err,
		)
	}
	return oauth2TokenToCache(tok), nil
}

func (a OAuth2Auth) withHTTPClient(w webFramework.WebFramework) context.Context {
	if a.httpClient == nil {
		return w.Ctx
	}
	return context.WithValue(w.Ctx, oauth2.HTTPClient, a.httpClient)
}

func oauth2TokenToCache(tok *oauth2.Token) *TokenCache {
	tokenType := tok.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	validUntil := time.Until(tok.Expiry) - tokenExpirySkew
	if validUntil < 0 {
		validUntil = 0
	}

	cache := &TokenCache{
		AccessToken: &OAuth2Token{
			Token:      tok.AccessToken,
			Type:       tokenType,
			TimeTaken:  time.Now(),
			ValidUntil: validUntil,
		},
	}
	if scope, ok := tok.Extra("scope").(string); ok {
		cache.AccessToken.Scope = scope
	}
	if tok.RefreshToken != "" {
		cache.RefreshToken = &OAuth2Token{
			Token:      tok.RefreshToken,
			Type:       tokenType,
			TimeTaken:  time.Now(),
			ValidUntil: 365 * 24 * time.Hour,
		}
	}
	return cache
}
