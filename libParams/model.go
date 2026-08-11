// Package libParams provides application parameter configuration management.
package libParams

import (
	"github.com/Depado/ginprom"

	"github.com/hmmftg/requestCore/libCallApi"
)

// ParametersMap holds a group of simple string parameters keyed by name.
type ParametersMap struct {
	Params map[string]string `yaml:"params"`
}

// SecureParametersMap holds a group of encrypted security parameters keyed by name.
type SecureParametersMap struct {
	SecureParams map[string]SecurityParam `yaml:"secureParams"`
}

// ApplicationParams holds the default application parameters.
type ApplicationParams[SpecialParams any] struct {
	Network               map[string]NetworkParams        `yaml:"networks"` // Definition of all networks, all networks will be started at startup
	Logging               LogParams                       `yaml:"logging"`
	DB                    map[string]DbParams             `yaml:"db"`                    // Database connection strings
	SecurityModule        map[string]SecurityModule       `yaml:"securityModule"`        // Security modules if exists
	RemoteAPIs            map[string]libCallApi.RemoteAPI `yaml:"remoteApis"`            // List of remote-api definition
	Constants             map[string]Constants            `yaml:"constants"`             // Constants used in app [response constants should be placed here]
	ParameterGroups       map[string]ParametersMap        `yaml:"parameterGroups"`       // Simple string parameters groupped by names
	SecureParameterGroups map[string]SecureParametersMap  `yaml:"secureParameterGroups"` // Encrypted string parameters, will be parsed at startup
	Specific              *SpecialParams                  `yaml:"specific"`              // Application specific args, should be parsed as yaml
	Metrics               *ginprom.Prometheus             `json:"-"`                     // Applications metrics storage
}

// ParamInterface defines the accessor methods for application parameters.
type ParamInterface interface {
	GetNetwork(name string) *NetworkParams
	GetLogging() LogParams
	GetDB(name string) *DbParams
	SetDB(name string, db *DbParams)
	GetSecurityModule(name string) *SecurityModule
	GetRemoteAPI(name string) *libCallApi.RemoteAPI
	GetParam(group, name string) *string
	GetSecureParam(group, name string) *SecurityParam
	GetConstants(name string) *Constants
	GetSpecificParams(name string) any
}

// GetRemoteAPI returns the remote API definition with the given name.
func (m ApplicationParams[SpecialParams]) GetRemoteAPI(name string) *libCallApi.RemoteAPI {
	return GetValueFromMap(name, m.RemoteAPIs)
}

// GetParam returns the string parameter value for the given group and name.
func (m ApplicationParams[SpecialParams]) GetParam(group, name string) *string {
	gr := GetValueFromMap(group, m.ParameterGroups)
	if gr == nil {
		return nil
	}
	return GetValueFromMap(name, gr.Params)
}

// GetSpecificParams returns the application-specific parameters.
func (m ApplicationParams[SpecialParams]) GetSpecificParams(_ string) any {
	return m.Specific
}

// GetValueFromMap returns a pointer to the value associated with name in mp, or nil if not found.
func GetValueFromMap[T any](name string, mp map[string]T) *T {
	val, ok := mp[name]
	if ok {
		return &val
	}
	return nil
}
