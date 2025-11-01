package libValidate

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// RegisterPaddedIpValidator registers the padded_ip validator.
// This must be called after Init() to register the padded_ip validator.
// Example:
//
//	libValidate.Init()
//	err := libValidate.RegisterPaddedIpValidator()
//	if err != nil {
//		log.Fatalf("failed to register padded_ip validator: %v", err)
//	}
func RegisterPaddedIpValidator() error {
	return RegisterValidation("padded_ip", PaddedIpValidator).
		WithTranslation(
			func(ut ut.Translator) error {
				return ut.Add("padded_ip", "{0} بایستی به فرمت 000.000.000.000 باشد", true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, err := ut.T("padded_ip", fe.Field())
				if err != nil {
					return fe.(error).Error()
				}
				return t
			}).
		WithErrorCode(ErrorCodeInvalidInputData).
		Build()
}
