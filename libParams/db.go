package libParams

type DbParams struct {
	DataBaseType    string        `yaml:"dbType"`
	DataBaseAddress SecurityParam `yaml:"dbAddress"`
}

func (m ApplicationParams[SpecialParams]) GetDB(name string) *DbParams {
	return GetValueFromMap(name, m.DB)
}
