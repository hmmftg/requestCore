// Package libCallApi provides HTTP client utilities for consuming remote REST APIs.
package libCallApi

import (
	"net/http"
	"sync"
	"time"
)

// RemoteAPI represents a remote API configuration including domain, auth, and options.
type RemoteAPI struct {
	Domain         string            `yaml:"domain" json:"domain"`
	Name           string            `yaml:"name" json:"name"`
	AuthData       Auth              `yaml:"auth" json:"-"`
	Options        map[string]string `yaml:"options" json:"-"`
	Auth           AuthSystem        `yaml:"-" json:"-"`
	TokenCacheLock *sync.Mutex       `yaml:"-" json:"-"`
	TokenCache     *TokenCache       `yaml:"-" json:"-"`
}

// RemoteAPIModel holds a map of named remote API configurations.
type RemoteAPIModel struct {
	RemoteAPIList map[string]RemoteAPI
}

// CallAPIInterface defines the contract for retrieving a RemoteAPI by name.
type CallAPIInterface interface {
	GetAPI(apiName string) RemoteAPI
}

const defaultTimeOut = 30 * time.Second

var httpClient = &http.Client{
	Timeout:   defaultTimeOut,
	Transport: &http.Transport{},
}
