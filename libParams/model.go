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
	GetSecurityModule(name string) *SecurityModule
	GetRemoteApi(name string) *libCallApi.RemoteApi
	GetParamGroup(name string) *ParametersMap
	GetSecureParamGroup(name string) *SecureParametersMap
	GetConstants(name string) *Constants
	GetSpecificParams(name string) any
}

func (m ApplicationParams[SpecialParams]) GetNetwork(name string) *NetworkParams {
	return GetValueFromMap(name, m.Network)
}
func (m ApplicationParams[SpecialParams]) GetLogging() LogParams {
	return m.Logging
}
func (m ApplicationParams[SpecialParams]) GetDB(name string) *DbParams {
	return GetValueFromMap(name, m.DB)
}
func (m ApplicationParams[SpecialParams]) GetSecurityModule(name string) *SecurityModule {
	return GetValueFromMap(name, m.SecurityModule)
}
func (m ApplicationParams[SpecialParams]) GetRemoteApi(name string) *libCallApi.RemoteApi {
	return GetValueFromMap(name, m.RemoteApis)
}
func (m ApplicationParams[SpecialParams]) GetParamGroup(name string) *ParametersMap {
	return GetValueFromMap(name, m.ParameterGroups)
}
func (m ApplicationParams[SpecialParams]) GetSecureParamGroup(name string) *SecureParametersMap {
	return GetValueFromMap(name, m.SecureParameterGroups)
}
func (m ApplicationParams[SpecialParams]) GetConstants(name string) *Constants {
	return GetValueFromMap(name, m.Constants)
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
