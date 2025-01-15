package libCallApi

import (
	"crypto/tls"
	"net/http"
	"time"
)

type RemoteApi struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Domain   string `yaml:"domain"`
	Name     string `yaml:"name"`
	Options  map[string]string
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
