package libParams

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hmmftg/requestCore/libCrypto"
	"github.com/hmmftg/requestCore/libCrypto/ssm"
	"github.com/jinzhu/copier"
)

type SecurityParam struct {
	IsPlain bool   `yaml:"isPlain,omitempty"`
	Value   string `yaml:"value"`
}

type SecurityModule struct {
	Type   string            `yaml:"type"`
	Params map[string]string `yaml:"params"`
	libCrypto.Sm
}

func (m ApplicationParams[SpecialParams]) GetSecurityModule(name string) *SecurityModule {
	return GetValueFromMap(name, m.SecurityModule)
}

func (m ApplicationParams[SpecialParams]) GetSecureParam(group, name string) *SecurityParam {
	gr := GetValueFromMap(group, m.SecureParameterGroups)
	if gr == nil {
		return nil
	}
	return GetValueFromMap(name, gr.SecureParams)
}

func FillCipher(key, iv string, field SecurityParam) (*SecurityParam, error) {
	if field.IsPlain {
		cipher, err := ssm.AesEncrypt(key, iv, field.Value)
		if err != nil {
			return nil, err
		}
		field.IsPlain = false
		field.Value = cipher
		return &field, nil
	}
	return &field, nil
}

func EncryptParams[T any](keyByte, ivByte []byte, paramFile string, params *ApplicationParams[T]) error {
	encryptedParams := ApplicationParams[T]{}
	err := copier.Copy(&encryptedParams, params)
	if err != nil {
		return err
	}

	key := base64.StdEncoding.EncodeToString(keyByte)
	iv := base64.StdEncoding.EncodeToString(ivByte)

	for name := range params.DB {
		db := params.DB[name]
		if len(db.DataBaseAddress.Value) > 0 {
			result, err := FillCipher(
				key, iv,
				db.DataBaseAddress)
			if err != nil {
				return err
			}
			db.DataBaseAddress = *result
			params.DB[name] = db
		}
	}

	for group := range params.SecureParameterGroups {
		for id := range params.SecureParameterGroups[group].SecureParams {
			if params.SecureParameterGroups[group].SecureParams[id].IsPlain {
				result, err := FillCipher(
					key, iv,
					params.SecureParameterGroups[group].SecureParams[id])
				if err != nil {
					return err
				}
				params.SecureParameterGroups[group].SecureParams[id] = *result
			}
		}
	}

	return Write(&encryptedParams, paramFile)
}

func Decrypt(key, iv, name string, field SecurityParam) (*SecurityParam, error) {
	if field.IsPlain {
		return nil, fmt.Errorf("security field %s is plain", name)
	}
	if len(field.Value) == 0 {
		return &field, nil
	}
	plainB64, err := ssm.AesDecrypt(key, iv, field.Value)
	if err != nil {
		return nil, err
	}
	bts, err := base64.StdEncoding.DecodeString(plainB64)
	if err != nil {
		return nil, err
	}
	field.Value = string(bts)
	return &field, nil
}

func DecryptParams[T any](keyByte, ivByte []byte, params *ApplicationParams[T]) (*ApplicationParams[T], error) {
	plainParams := &ApplicationParams[T]{}
	err := copier.Copy(&plainParams, params)
	if err != nil {
		return plainParams, err
	}

	key := base64.StdEncoding.EncodeToString(keyByte)
	iv := base64.StdEncoding.EncodeToString(ivByte)

	for name := range params.DB {
		db := params.DB[name]
		result, err := Decrypt(key, iv, "dbAddress", db.DataBaseAddress)
		if err != nil {
			return plainParams, err
		}
		db.DataBaseAddress = *result
		plainParams.DB[name] = db
	}

	for group := range params.SecureParameterGroups {
		for id := range params.SecureParameterGroups[group].SecureParams {
			current := params.SecureParameterGroups[group].SecureParams[id]
			if current.IsPlain {
				return nil, fmt.Errorf("security field %s is plain", id)
			}
			result, err := Decrypt(
				key, iv, id,
				current)
			if err != nil {
				return nil, err
			}
			current = *result
			tags := strings.Split(id, "#")
			switch tags[0] {
			case "remote-api":
				api := params.RemoteApis[tags[1]]
				switch tags[2] {
				case "password":
					api.AuthData.Password = current.Value
				case "client-secret":
					api.AuthData.ClientSecret = current.Value
				}
				params.RemoteApis[tags[1]] = api
			case "security-module-param":
				if tags[3] == "password" {
					params.SecurityModule[tags[1]].Params[tags[2]+"-pass"] = current.Value
				}
			default:
				params.ParameterGroups[group].Params[id] = params.SecureParameterGroups[group].SecureParams[id].Value
			}
		}
	}

	return plainParams, nil
}
