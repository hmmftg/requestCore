package ssm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"log"
	"os"

	"github.com/pavlo-v-chernykh/keystore-go/v4"
)

func ReadKeyStore(filename string, password []byte) keystore.KeyStore {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal("ReadKeyStore 1 ", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal("ReadKeyStore 2 ", err)
		}
	}()

	ks := keystore.New()
	if err := ks.Load(f, password); err != nil {
		log.Fatal("ReadKeyStore 3 ", err) // nolint: gocritic
	}

	return ks
}

func WriteKeyStore(ks keystore.KeyStore, filename string, password []byte) {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal("WriteKeyStore 1 ", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal("WriteKeyStore 2 ", err)
		}
	}()

	err = ks.Store(f, password)
	if err != nil {
		log.Fatal("WriteKeyStore 3 ", err) // nolint: gocritic
	}
}

func Zeroing(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

func EncryptRsa(ks1 keystore.KeyStore, keyId, data string) (string, error) {
	certBytes, err := ks1.GetTrustedCertificateEntry(keyId)
	if err != nil {
		return "Get Certificate", err
	}
	cert, err := x509.ParseCertificate(certBytes.Certificate.Content)
	if err != nil {
		return "Get Public Key", err
	}
	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	clearByte, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "Get Byte From Base64 Data", err
	}
	cipher, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPublicKey, clearByte)
	if err != nil {
		return "Encrypt Rsa", err
	}
	cipherB64 := base64.StdEncoding.EncodeToString(cipher)
	return cipherB64, nil
}

func DecryptRsa(ks1 keystore.KeyStore, keyId, pass, data string) (string, error) {
	prvBytes, err := ks1.GetPrivateKeyEntry(keyId, []byte(pass))
	if err != nil {
		return "Get PrivateKey", err
	}
	key, err := x509.ParsePKCS8PrivateKey(prvBytes.PrivateKey)
	if err != nil {
		return "Parse Private Key", err
	}
	cipherByte, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "Get Byte From Base64 Data", err
	}
	clearByte, err := rsa.DecryptPKCS1v15(rand.Reader, key.(*rsa.PrivateKey), cipherByte)
	if err != nil {
		return "Decrypt Rsa", err
	}
	clearB64 := base64.StdEncoding.EncodeToString(clearByte)
	return clearB64, nil
}
