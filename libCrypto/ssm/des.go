package ssm

import (
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/hmmftg/requestCore/response"
)

type DesMode int

const (
	Ecb DesMode = iota + 1
	Cbc
)

var ZeroIv []byte = []byte{0, 0, 0, 0, 0, 0, 0, 0}

func EncryptDes(textB64, keyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", err
	}

	plaintext, err := base64.StdEncoding.DecodeString(textB64)
	if err != nil {
		return "", err
	}

	ciphertext, err := encryptDes(plaintext, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func EncryptDesHex(textHex, keyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	plaintext, err := hex.DecodeString(textHex)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	ciphertext, err := encryptDes(plaintext, key)
	if err != nil {
		return "", response.ToErrorState(err)
	}
	return strings.ToUpper(hex.EncodeToString(ciphertext)), nil
}

func DecryptDes(cipherB64, keyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", err
	}

	cipherByte, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return "", err
	}
	plaintext, err := decryptDes(cipherByte, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(plaintext), nil
}

func DecryptDesHex(cipherHex, keyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	cipherByte, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", response.ToErrorState(err)
	}
	plaintext, err := decryptDes(cipherByte, key)
	if err != nil {
		return "", response.ToErrorState(err)
	}
	return strings.ToUpper(hex.EncodeToString(plaintext)), nil
}

func DecryptDesBothHex(cipherHex, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", errors.Join(err, fmt.Errorf("unable to parse hex key"))
	}

	cipherByte, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", errors.Join(err, fmt.Errorf("unable to parse hex cipher"))
	}
	plaintext, err := decryptDes(cipherByte, key)
	if err != nil {
		return "", errors.Join(err, fmt.Errorf("unable to decrypt"))
	}
	return string(plaintext), nil
}

func decryptDes(data []byte, key []byte) ([]byte, error) {
	var desCipher cipher.Block
	var err error
	if len(key) == 8 {
		desCipher, err = des.NewCipher(key)
	} else if len(key) == 16 {
		var keyData []byte
		keyData = append(keyData, key...)
		keyData = append(keyData, key[:8]...)
		desCipher, err = des.NewTripleDESCipher(keyData)
	} else {
		desCipher, err = des.NewTripleDESCipher(key)
	}
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("unable to init cipher"))
	}

	result := make([]byte, len(data))
	for i := 0; i < len(data); i += 8 {
		desCipher.Decrypt(result[i:], data[i:])
	}
	return result, nil

}

func encryptDes(data []byte, key []byte) ([]byte, error) {
	var desCipher cipher.Block
	var err error
	if len(key) == 8 {
		desCipher, err = des.NewCipher(key)
	} else if len(key) == 16 {
		var keyData []byte
		keyData = append(keyData, key...)
		keyData = append(keyData, key[:8]...)
		desCipher, err = des.NewTripleDESCipher(keyData)
	} else {
		desCipher, err = des.NewTripleDESCipher(key)
	}
	if err != nil {
		return nil, err
	}

	result := make([]byte, len(data))
	for i := 0; i < len(data); i += 8 {
		desCipher.Encrypt(result[i:], data[i:])
	}
	return result, nil

}
