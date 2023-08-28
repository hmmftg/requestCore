package requestCore

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
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

var Validate *validator.Validate

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

	err := Validate.RegisterTranslation(tag, trans, registerFn, transFn)
	if err != nil {
		log.Fatalln("error in RegisterTranslation:", err)
	}
}

func Init() (ut.Translator, *validator.Validate, error) {
	uni := ut.New(fa.New())
	trans, _ := uni.GetTranslator("fa")
	Validate = validator.New() //(config)
	err := Validate.RegisterValidation("padded_ip", PaddedIpValidator)
	if err != nil {
		return nil, nil, err
	}
	Validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
	//Validate.RegisterStructValidation(CustomerStructLevelValidation, GetCustomerInfoRequest{})
	err = fa_translations.RegisterDefaultTranslations(Validate, trans)
	if err != nil {
		return nil, nil, libError.Join(err, "error calling fa_translations.RegisterDefaultTranslations")
	}

	addTranslation("padded_ip", "{0} بایستی به فرمت 000.000.000.000 باشد", trans)

	return trans, Validate, nil
}

func InitReqLog(w webFramework.WebFramework, reqLog *libRequest.Request, core RequestCoreInterface, method, path string) *response.ErrorState {
	w.Parser.SetLocal("reqLog", reqLog)
	status, result, err := core.RequestTools().Initialize(w, method, path, reqLog)
	if err != nil {
		return response.Error(status, result["desc"], result["message"], err)
	}
	return nil
}
