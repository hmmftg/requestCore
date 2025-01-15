package libCallApi

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

type RemoteApi struct {
	Domain         string            `yaml:"domain"`
	Name           string            `yaml:"name"`
	AuthData       Auth              `yaml:"auth"`
	Options        map[string]string `yaml:"options"`
	Auth           AuthSystem
	TokenCacheLock *sync.Mutex
	TokenCache     *TokenCache
}

type RemoteApiModel struct {
	RemoteApiList map[string]RemoteApi
}

type CallApiInterface interface {
	GetApi(apiName string) RemoteApi
	ConsumeRestBasicAuthApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error)
	ConsumeRestApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error)
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}
