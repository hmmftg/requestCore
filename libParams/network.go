package libParams

// NetworkParams holds network-related configuration parameters.
type NetworkParams struct {
	Port       string `yaml:"port"`
	StaticPort string `yaml:"staticPort"`
	StaticPath string `yaml:"staticPath"`
	///////////////////// TLS ////////////////////////////////////////////
	TLSPort string `yaml:"tlsPort"`
	TLSKey  string `yaml:"tlsKey"`
	TLSCert string `yaml:"tlsCert"`
}

// GetNetwork returns the network parameters for the given name.
func (m ApplicationParams[SpecialParams]) GetNetwork(name string) *NetworkParams {
	return GetValueFromMap(name, m.Network)
}
