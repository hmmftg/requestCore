// Package libDictionary provides dictionary/lookup utilities for requestCore.
package libDictionary

// DictionaryModel holds a map of message descriptions for dictionary lookups.
type DictionaryModel struct {
	MessageDesc map[string]string
}

// DictionaryInterface defines the interface for retrieving dictionary values by name.
type DictionaryInterface interface {
	GetDictionaryValue(name string) string
}

// GetDictionaryValue returns the description for the given name from the dictionary.
func (m DictionaryModel) GetDictionaryValue(name string) string {
	return m.MessageDesc[name]
}
