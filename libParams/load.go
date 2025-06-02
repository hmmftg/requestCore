package libParams

import (
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

func Load[T any](path string) (*ApplicationParams[T], error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	paramData := ApplicationParams[T]{}
	if err := yaml.Unmarshal(data, &paramData); err != nil {
		return nil, err
	}
	return &paramData, nil
}

func ParsePrams[T any](paramFile string, keys [][]byte) (*ApplicationParams[T], error) {
	wsParams, err := Load[T](paramFile)
	if err != nil {
		return nil, err
	}
	wsParams, err = DecryptParams(keys[0], keys[1], wsParams)
	if err != nil {
		return nil, err
	}
	return wsParams, nil
}

func Write[T any](params *ApplicationParams[T], path string) error {
	paramData, err := yaml.Marshal(&params)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, paramData, fs.ModeAppend)
	if err != nil {
		return err
	}
	return nil
}
