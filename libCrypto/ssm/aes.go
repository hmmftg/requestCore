package ssm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
)

func PKCS7Padding(ciphertext []byte) []byte {
	padding := aes.BlockSize - len(ciphertext)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(plantText []byte) []byte {
	var unpadding int
	length := len(plantText)

	if length == 0 {
		unpadding = 0
	} else {
		unpadding = int(plantText[length-1])
	}

	return plantText[:(length - unpadding)]
}

func AesDecrypt(kaeyB64, ivB64, textB64 string) (string, error) {
	key, _ := base64.StdEncoding.DecodeString(kaeyB64)
	iv, _ := base64.StdEncoding.DecodeString(ivB64)
	ciphertext, _ := base64.StdEncoding.DecodeString(textB64)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "Aes Cipher", err
	}
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext = PKCS7UnPadding(plaintext)
	return base64.StdEncoding.EncodeToString(plaintext), nil
}

func AesEncrypt(kaeyB64, ivB64, text string) (string, error) {
	plaintext := []byte(text)
	key, _ := base64.StdEncoding.DecodeString(kaeyB64)
	iv, _ := base64.StdEncoding.DecodeString(ivB64)

	plaintext = PKCS7Padding(plaintext)
	ciphertext := make([]byte, len(plaintext))
	block, err := aes.NewCipher(key)
	if err != nil {
		return "Aes Cipher", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// 28cEVB4BUE7GKNwjuRhN3szK5E3!&q*y
func RandomData(len int) string {
	key := make([]byte, len)

	rand.Read(key)

	return base64.StdEncoding.EncodeToString(key)
}
