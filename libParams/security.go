package libParams

import "github.com/hmmftg/requestCore/libCrypto"

type SecurityParam struct {
	IsPlain bool   `yaml:"isPlain,omitempty"`
	Value   string `yaml:"value"`
}

type SecurityModule struct {
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
	libCrypto.Sm
}
