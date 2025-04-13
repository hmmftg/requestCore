package libParams

type Constants struct {
	ErrorDesc   map[string]string `yaml:"errorDesc"`
	MessageDesc map[string]string `yaml:"messageDesc"`
}

func (m ApplicationParams[SpecialParams]) GetConstants(name string) *Constants {
	return GetValueFromMap(name, m.Constants)
}
