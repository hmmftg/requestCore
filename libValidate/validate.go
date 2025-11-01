package libValidate

import (
	"log"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/locales/fa"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	fa_translations "github.com/go-playground/validator/v10/translations/fa"
	"github.com/hmmftg/requestCore/libError"
)

// ValidatorType represents the type of validator
type ValidatorType int

const (
	// ValidatorTypeCustom represents a custom validator registered via RegisterValidation
	ValidatorTypeCustom ValidatorType = iota
	// ValidatorTypeSystem represents a system validator (from validator package)
	ValidatorTypeSystem
)

// ValidatorInfo stores information about a registered validator
type ValidatorInfo struct {
	Tag       string
	ErrorCode string
	Type      ValidatorType
}

// ValidatorBuilder provides a fluent interface for registering validators
type ValidatorBuilder struct {
	tag        string
	fn         validator.Func
	registerFn func(ut.Translator) error
	transFn    func(ut.Translator, validator.FieldError) string
	errorCode  string
}

var (
	Validator  *validator.Validate
	Translator ut.Translator
	initOnce   sync.Once

	// Registry for registered validators
	validatorRegistry = make(map[string]ValidatorInfo)
	validatorMutex    sync.RWMutex

	// Custom system validators configuration (can be set before Init)
	customSystemValidators      = make(map[string]string)
	customSystemValidatorsMutex sync.RWMutex
	useDefaultSystemValidators  = true
)

const RegexPaddedIp string = `^((25[0-5]|2[0-4]\d|1\d\d|0\d\d)\.?\b){4}$` //^((25[0-5]|2[0-4]\d|1\d\d|0\d\d)\.?\b){4}$

// Error code constants
const (
	ErrorCodeRequiredField    = "REQUIRED-FIELD"
	ErrorCodeInvalidInputData = "INVALID-INPUT-DATA"
)

func PaddedIpValidator(fl validator.FieldLevel) bool {
	st := fl.Field().String()

	if len(st) != 15 {
		return false
	}

	re := regexp.MustCompile(RegexPaddedIp)
	return re.MatchString(st)
}

// RegisterValidation starts building a validator registration with method chaining.
// Example:
//
//	err := libValidate.RegisterValidation("nationalcode",
//		func(fl validator.FieldLevel) bool {
//			nationalCode := fl.Field().String()
//			return ValidateNationalCode(nationalCode)
//		}).
//		WithTranslation(
//			func(ut ut.Translator) error {
//				return ut.Add("nationalcode", "{0} وارد شده معتبر نمی باشد", true)
//			},
//			func(ut ut.Translator, fe validator.FieldError) string {
//				t, _ := ut.T("nationalcode", fe.Field())
//				return t
//			}).
//		WithErrorCode(libValidate.ErrorCodeInvalidInputData).
//		Build()
//	if err != nil {
//		log.Fatalf("failed to register validation for nationalcode: %v", err)
//	}
func RegisterValidation(tag string, fn validator.Func) *ValidatorBuilder {
	// Only call Init if Validator is not already initialized
	if Validator == nil {
		Init()
	}
	return &ValidatorBuilder{
		tag:       tag,
		fn:        fn,
		errorCode: ErrorCodeInvalidInputData, // default error code
	}
}

// WithTranslation adds translation functions to the validator builder.
func (vb *ValidatorBuilder) WithTranslation(registerFn func(ut.Translator) error, transFn func(ut.Translator, validator.FieldError) string) *ValidatorBuilder {
	vb.registerFn = registerFn
	vb.transFn = transFn
	return vb
}

// WithErrorCode sets the error code for the validator.
// If not called, defaults to ErrorCodeInvalidInputData.
func (vb *ValidatorBuilder) WithErrorCode(errorCode string) *ValidatorBuilder {
	if errorCode != "" {
		vb.errorCode = errorCode
	}
	return vb
}

// Build completes the validator registration.
func (vb *ValidatorBuilder) Build() error {
	// Register the validator
	err := Validator.RegisterValidation(vb.tag, vb.fn)
	if err != nil {
		return libError.Join(err, "error in RegisterValidation(%s)", vb.tag)
	}

	// Register the translation if provided
	if vb.registerFn != nil && vb.transFn != nil {
		err = Validator.RegisterTranslation(vb.tag, Translator, vb.registerFn, vb.transFn)
		if err != nil {
			return libError.Join(err, "error in RegisterTranslation(%s)", vb.tag)
		}
	}

	// Store the validator info in registry
	validatorMutex.Lock()
	validatorRegistry[vb.tag] = ValidatorInfo{
		Tag:       vb.tag,
		ErrorCode: vb.errorCode,
		Type:      ValidatorTypeCustom,
	}
	validatorMutex.Unlock()

	return nil
}

// RegisterSystemValidator registers a system validator (from validator package) in the registry.
// This can be called by library users to register custom system validators.
// Example:
//
//	libValidate.RegisterSystemValidator("email", libValidate.ErrorCodeInvalidInputData)
func RegisterSystemValidator(tag string, errorCode string) {
	validatorMutex.Lock()
	defer validatorMutex.Unlock()

	validatorRegistry[tag] = ValidatorInfo{
		Tag:       tag,
		ErrorCode: errorCode,
		Type:      ValidatorTypeSystem,
	}
}

// registerSystemValidator is the internal version used during initialization.
func registerSystemValidator(tag string, errorCode string) {
	RegisterSystemValidator(tag, errorCode)
}

// GetErrorCode returns the error code for a given tag, or a default if not found.
func GetErrorCode(tag string) string {
	validatorMutex.RLock()
	defer validatorMutex.RUnlock()

	if info, ok := validatorRegistry[tag]; ok {
		return info.ErrorCode
	}

	// Default error code for unregistered validators
	return ErrorCodeInvalidInputData
}

// IsCustomValidator checks if a tag is a custom validator by checking the registry.
func IsCustomValidator(tag string) bool {
	validatorMutex.RLock()
	defer validatorMutex.RUnlock()

	if info, ok := validatorRegistry[tag]; ok {
		return info.Type == ValidatorTypeCustom
	}

	// If not in registry, assume it's a custom validator
	return true
}

// GetValidatorInfo returns the validator information for a given tag, or nil if not found.
func GetValidatorInfo(tag string) *ValidatorInfo {
	validatorMutex.RLock()
	defer validatorMutex.RUnlock()

	if info, ok := validatorRegistry[tag]; ok {
		return &info
	}

	return nil
}

func firstTime() (ut.Translator, *validator.Validate, error) {
	uni := ut.New(fa.New())
	Translator, _ := uni.GetTranslator("fa")
	Validator = validator.New() //(config)

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
	err := fa_translations.RegisterDefaultTranslations(Validator, Translator)
	if err != nil {
		return nil, nil, libError.Join(err, "error in RegisterDefaultTranslations(fa_translations)")
	}

	// Register system validators in the registry
	registerSystemValidators()

	return Translator, Validator, nil
}

// SetSystemValidators allows library users to configure which system validators to register.
// This should be called before Init() to take effect.
// If validators is nil or empty, default validators will be used.
// Example:
//
//	libValidate.SetSystemValidators(map[string]string{
//		"required": libValidate.ErrorCodeRequiredField,
//		"email":    libValidate.ErrorCodeInvalidInputData,
//	})
func SetSystemValidators(validators map[string]string) {
	customSystemValidatorsMutex.Lock()
	defer customSystemValidatorsMutex.Unlock()

	if len(validators) == 0 {
		useDefaultSystemValidators = true
		customSystemValidators = make(map[string]string)
	} else {
		useDefaultSystemValidators = false
		customSystemValidators = make(map[string]string)
		for tag, errorCode := range validators {
			customSystemValidators[tag] = errorCode
		}
	}
}

// DisableDefaultSystemValidators disables registration of default system validators.
// Custom system validators can still be registered using SetSystemValidators or RegisterSystemValidator.
// This should be called before Init() to take effect.
func DisableDefaultSystemValidators() {
	customSystemValidatorsMutex.Lock()
	defer customSystemValidatorsMutex.Unlock()
	useDefaultSystemValidators = false
}

// registerSystemValidators registers system validators in the registry.
// Uses custom validators if configured, otherwise uses default validators.
func registerSystemValidators() {
	customSystemValidatorsMutex.RLock()
	useDefaults := useDefaultSystemValidators
	customValidators := make(map[string]string)
	for tag, code := range customSystemValidators {
		customValidators[tag] = code
	}
	customSystemValidatorsMutex.RUnlock()

	if !useDefaults && len(customValidators) > 0 {
		// Register custom system validators
		for tag, errorCode := range customValidators {
			registerSystemValidator(tag, errorCode)
		}
	} else if useDefaults {
		// Register default system validators
		registerDefaultSystemValidators()
	}
}

// registerDefaultSystemValidators registers known default system validators in the registry.
func registerDefaultSystemValidators() {
	// Required validators
	registerSystemValidator("required", ErrorCodeRequiredField)
	registerSystemValidator("required_unless", ErrorCodeRequiredField)
	registerSystemValidator("required_if", ErrorCodeRequiredField)

	// Input validation validators
	registerSystemValidator("numeric", ErrorCodeInvalidInputData)
	registerSystemValidator("len", ErrorCodeInvalidInputData)
	registerSystemValidator("min", ErrorCodeInvalidInputData)
	registerSystemValidator("max", ErrorCodeInvalidInputData)
	registerSystemValidator("startswith", ErrorCodeInvalidInputData)
	registerSystemValidator("alphanum", ErrorCodeInvalidInputData)
	registerSystemValidator("oneof", ErrorCodeInvalidInputData)
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
	initOnce.Do(func() {
		var err error
		Translator, Validator, err = firstTime()
		if err != nil {
			log.Fatalln("Error Initializing Validator:", err)
		}
	})
}
