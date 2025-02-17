package libCallApi

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

type RemoteApi struct {
	Domain         string            `yaml:"domain" json:"domain"`
	Name           string            `yaml:"name" json:"name"`
	AuthData       Auth              `yaml:"auth" json:"-"`
	Options        map[string]string `yaml:"options" json:"-"`
	Auth           AuthSystem        `yaml:"-" json:"-"`
	TokenCacheLock *sync.Mutex       `yaml:"-" json:"-"`
	TokenCache     *TokenCache       `yaml:"-" json:"-"`
}

type RemoteApiModel struct {
	RemoteApiList map[string]RemoteApi
}

type CallApiInterface interface {
	GetApi(apiName string) RemoteApi
	ConsumeRestBasicAuthApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, error)
	ConsumeRestApi(requestJson []byte, apiName, path, contentType, method string, headers map[string]string) ([]byte, string, int, error)
}

const defaultTimeOut = 30 * time.Second

var httpClient = &http.Client{
	Timeout: defaultTimeOut,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}
