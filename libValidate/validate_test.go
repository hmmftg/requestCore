package libValidate_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/hmmftg/requestCore/libValidate"
)

type Customer struct {
	ID   string `json:"id,omitempty" validate:"omitempty,numeric,len=10" name:"شناسه"`
	Name string `json:"name,omitempty" validate:"omitempty,startswith=name." name:"نام"`
}

func TestValidate(t *testing.T) {
	type Test struct {
		Struct   any
		Expected validator.ValidationErrorsTranslations
	}
	testList := []Test{
		{
			Struct: Customer{ID: "222"},
			Expected: validator.ValidationErrorsTranslations{
				"Customer.شناسه": "طول شناسه باید 10 کاراکتر باشد",
			},
		},
		{
			Struct: Customer{Name: "aname.aa"},
			Expected: validator.ValidationErrorsTranslations{
				"Customer.نام": "Key: 'Customer.نام' Error:Field validation for 'نام' failed on the 'startswith' tag",
			},
		},
	}
	for id := range testList {
		request := testList[id].Struct
		err, errValidate := libValidate.ValidateStruct(request)
		if err != nil {
			t.Fatalf("err: %+v\n", err)
		}
		list := errValidate.Translate(libValidate.GetTranslator())
		for k, v := range list {
			if v != testList[id].Expected[k] {
				t.Fatalf("validate error on [%s]: %s, expected: %s\n", k, v, testList[id].Expected[k])
			}
		}
	}
}
