package libParams

type DbParams struct {
	DataBaseType    string         `yaml:"dbType"`
	DataBaseAddress *SecurityParam `yaml:"dbAddress"`
}
