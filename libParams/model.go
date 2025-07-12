package libParams

import (
	"github.com/Depado/ginprom"
	"github.com/hmmftg/requestCore/libCallApi"
)

type ParametersMap struct {
	Params map[string]string `yaml:"params"`
}

type SecureParametersMap struct {
	SecureParams map[string]SecurityParam `yaml:"secureParams"`
}

// Application can be designed with below params as default
type ApplicationParams[SpecialParams any] struct {
	Network               map[string]NetworkParams        `yaml:"networks"` // Definition of all networks, all networks will be started at startup
	Logging               LogParams                       `yaml:"logging"`
	DB                    map[string]DbParams             `yaml:"db"`                    // Database connection strings
	SecurityModule        map[string]SecurityModule       `yaml:"securityModule"`        // Security modules if exists
	RemoteApis            map[string]libCallApi.RemoteApi `yaml:"remoteApis"`            // List of remote-api definition
	Constants             map[string]Constants            `yaml:"constants"`             // Constants used in app [response constants should be placed here]
	ParameterGroups       map[string]ParametersMap        `yaml:"parameterGroups"`       // Simple string parameters groupped by names
	SecureParameterGroups map[string]SecureParametersMap  `yaml:"secureParameterGroups"` // Encrypted string parameters, will be parsed at startup
	Specific              *SpecialParams                  `yaml:"specific"`              // Application specific args, should be parsed as yaml
	Metrics               *ginprom.Prometheus             `json:"-"`                     // Applications metrics storage
}

type ParamInterface interface {
	GetNetwork(name string) *NetworkParams
	GetLogging() LogParams
	GetDB(name string) *DbParams
	SetDB(name string, db *DbParams)
	GetSecurityModule(name string) *SecurityModule
	GetRemoteApi(name string) *libCallApi.RemoteApi
	GetParam(group, name string) *string
	GetSecureParam(group, name string) *SecurityParam
	GetConstants(name string) *Constants
	GetSpecificParams(name string) any
}

func (m ApplicationParams[SpecialParams]) GetRemoteApi(name string) *libCallApi.RemoteApi {
	return GetValueFromMap(name, m.RemoteApis)
}

func (m ApplicationParams[SpecialParams]) GetParam(group, name string) *string {
	gr := GetValueFromMap(group, m.ParameterGroups)
	if gr == nil {
		return nil
	}
	return GetValueFromMap(name, gr.Params)
}

func (m ApplicationParams[SpecialParams]) GetSpecificParams(name string) any {
	return m.Specific
}

func GetValueFromMap[T any](name string, mp map[string]T) *T {
	val, ok := mp[name]
	if ok {
		return &val
	}
	return nil
}
