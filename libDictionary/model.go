package libDictionary

type DictionaryModel struct {
	MessageDesc map[string]string
}

type DictionaryInterface interface {
	GetDictionaryValue(name string) string
}

func (m DictionaryModel) GetDictionaryValue(name string) string {
	return m.MessageDesc[name]
}
