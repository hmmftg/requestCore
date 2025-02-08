package libValidate

import (
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/locales/fa"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	fa_translations "github.com/go-playground/validator/v10/translations/fa"
	"github.com/hmmftg/requestCore/libError"
)

var (
	Validator  *validator.Validate
	Translator ut.Translator
)

const RegexPaddedIp string = `^((25[0-5]|2[0-4]\d|1\d\d|0\d\d)\.?\b){4}$` //^((25[0-5]|2[0-4]\d|1\d\d|0\d\d)\.?\b){4}$

func PaddedIpValidator(fl validator.FieldLevel) bool {
	st := fl.Field().String()

	if len(st) != 15 {
		return false
	}

	re := regexp.MustCompile(RegexPaddedIp)
	return re.MatchString(st)
}

func addTranslation(tag string, errMessage string, trans ut.Translator) {
	registerFn := func(ut ut.Translator) error {
		return ut.Add(tag, errMessage, false)
	}

	transFn := func(ut ut.Translator, fe validator.FieldError) string {
		param := fe.Param()
		tag := fe.Tag()

		t, err := ut.T(tag, fe.Field(), param)
		if err != nil {
			return fe.(error).Error()
		}
		return t
	}

	err := Validator.RegisterTranslation(tag, trans, registerFn, transFn)
	if err != nil {
		log.Fatalln("error in RegisterTranslation:", err)
	}
}

func firstTime() (ut.Translator, *validator.Validate, error) {
	uni := ut.New(fa.New())
	Translator, _ := uni.GetTranslator("fa")
	Validator = validator.New() //(config)
	err := Validator.RegisterValidation("padded_ip", PaddedIpValidator)
	if err != nil {
		return nil, nil, libError.Join(err, "error in RegisterValidation(padded_ip)")
	}
	Validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		faName := fld.Tag.Get("name")
		if len(faName) > 0 {
			return faName
		}
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	//Validate.RegisterStructValidation(CustomerStructLevelValidation, GetCustomerInfoRequest{})
	err = fa_translations.RegisterDefaultTranslations(Validator, Translator)
	if err != nil {
		return nil, nil, libError.Join(err, "error in RegisterDefaultTranslations(fa_translations)")
	}

	addTranslation("padded_ip", "{0} بایستی به فرمت 000.000.000.000 باشد", Translator)

	return Translator, Validator, nil
}

func ValidateStruct(in any) (*validator.InvalidValidationError, validator.ValidationErrors) {
	Init()
	err := Validator.Struct(in)
	if err != nil {
		switch casted := err.(type) {
		case validator.ValidationErrors:
			return nil, casted
		case *validator.InvalidValidationError:
			return casted, nil
		}
	}
	return nil, nil
}

func GetTranslator() ut.Translator {
	return Translator
}

func Init() {
	if Validator == nil {
		var err error
		Translator, Validator, err = firstTime()
		if err != nil {
			log.Fatalln("Error Initializing Validator:", err)
		}
	}
}
