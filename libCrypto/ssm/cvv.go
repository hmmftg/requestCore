package ssm

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/hmmftg/requestCore/response"
)

func EncryptCvv(tspB64, cvkB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(cvkB64)
	if err != nil {
		return "", err
	}
	plainTsp, err := base64.StdEncoding.DecodeString(tspB64)
	if err != nil {
		return "", err
	}

	var key1Data []byte
	key1Data = append(key1Data, key[:8]...)
	key1Data = append(key1Data, key[:8]...)
	key1DataB64 := base64.StdEncoding.EncodeToString(key1Data)
	tspPart1 := plainTsp[:8]
	tspPart1B64 := base64.StdEncoding.EncodeToString(tspPart1)

	cipherTspPart1B64, err := EncryptDes(tspPart1B64, key1DataB64)
	if err != nil {
		return "", err
	}
	cipherTspPart1, err := base64.StdEncoding.DecodeString(cipherTspPart1B64)
	if err != nil {
		return "", err
	}
	tspPart2 := plainTsp[8:]
	tspPart2Xor := make([]byte, len(cipherTspPart1))
	for i := range cipherTspPart1 {
		tspPart2Xor[i] = cipherTspPart1[i] ^ tspPart2[i]
	}
	tspPart2B64 := base64.StdEncoding.EncodeToString(tspPart2Xor)

	cipherCvv, err := EncryptDes(tspPart2B64, cvkB64)
	if err != nil {
		return "", err
	}
	return cipherCvv, nil
}

func GenCvv(pan, exp, service, cvkB64 string) (string, error) {
	tspHex := pan + exp + service + "000000000"
	tsp, _ := hex.DecodeString(tspHex)
	tspB64 := base64.StdEncoding.EncodeToString(tsp)

	cipherTspB64, err := EncryptCvv(tspB64, cvkB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	cipherTsp, err := base64.StdEncoding.DecodeString(cipherTspB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	cvvHex := hex.EncodeToString(cipherTsp[:8])

	return Decimalize(cvvHex, 3, false), nil
}
