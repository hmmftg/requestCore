package libParams

// Constants holds error and message description maps used for response formatting.
type Constants struct {
	ErrorDesc   map[string]string `yaml:"errorDesc"`
	MessageDesc map[string]string `yaml:"messageDesc"`
}

// GetConstants returns the constants block with the given name.
func (m ApplicationParams[SpecialParams]) GetConstants(name string) *Constants {
	return GetValueFromMap(name, m.Constants)
}
