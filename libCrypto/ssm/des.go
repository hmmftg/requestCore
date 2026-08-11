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

// DesMode identifies the DES cipher mode (ECB or CBC).
type DesMode int

const (
	// Ecb indicates Electronic Codebook mode.
	Ecb DesMode = iota + 1
	// Cbc indicates Cipher Block Chaining mode.
	Cbc
)

// ZeroIv is an 8-byte zero initialization vector used for DES operations.
var ZeroIv = []byte{0, 0, 0, 0, 0, 0, 0, 0}

// EncryptDes encrypts base64-encoded plaintext with DES/Triple-DES using a base64-encoded key.
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

// EncryptDesHex encrypts hex-encoded plaintext with DES/Triple-DES using a base64-encoded key.
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

// DecryptDes decrypts a base64-encoded DES/Triple-DES ciphertext using a base64-encoded key.
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

// DecryptDesHex decrypts a hex-encoded DES/Triple-DES ciphertext using a base64-encoded key.
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

// DecryptDesBothHex decrypts a hex-encoded DES/Triple-DES ciphertext using a hex-encoded key.
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
