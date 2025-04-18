package libParams

// Network related Params
type NetworkParams struct {
	Port       string `yaml:"port"`
	StaticPort string `yaml:"staticPort"`
	StaticPath string `yaml:"staticPath"`
	///////////////////// TLS ////////////////////////////////////////////
	TlsPort string `yaml:"tlsPort"`
	TlsKey  string `yaml:"tlsKey"`
	TlsCert string `yaml:"tlsCert"`
}

func (m ApplicationParams[SpecialParams]) GetNetwork(name string) *NetworkParams {
	return GetValueFromMap(name, m.Network)
}
