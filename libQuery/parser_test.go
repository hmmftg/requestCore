package libQuery_test

import (
	"testing"

	"github.com/hmmftg/requestCore/libQuery"
)

type Person struct {
	Name   string
	Age    int
	Emails []string
	Extra  map[string]string
}

func Benchmark_Decode(b *testing.B) {
	input := map[string]interface{}{
		"name":   "Mitchell",
		"age":    91,
		"emails": []string{"one", "two", "three"},
		"extra": map[string]string{
			"twitter": "mitchellh",
		},
	}

	var err error
	for i := 0; i < b.N; i++ {
		_, err = libQuery.ParseMap[Person](input)
		if err != nil {
			b.Error(err)
		}
	}
}
