package ssm

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/libCrypto"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

type Ssm struct {
	Cvk  string
	Pvk  string
	Tpk  string
	Csd  string
	Pvki string
}

func (s *Ssm) Cvv(pan, exp, cvvType string) (string, error) {
	switch cvvType {
	case libCrypto.Cvv1:
		return GenCvv(pan, exp, "506", s.Cvk)
	case libCrypto.Cvv2:
		return GenCvv(pan, exp[2:]+exp[:2], "000", s.Cvk)
	}
	return "", libError.NewWithDescription(
		http.StatusInternalServerError,
		"GEN_CVV_ERROR",
		"invalid Cvv Type: %s", cvvType,
	)
}

func (s *Ssm) Pvv(pan, pinBlock string) (string, error) {
	pin, err := s.PinBlockDecode(pan, pinBlock)
	if err != nil {
		return "", err
	}
	return GeneratePvv(pan, s.Pvk, "1", pin)
}

func (s *Ssm) Offset(pan, pinBlock string) (string, error) {
	pin, err := s.PinBlockDecode(pan, pinBlock)
	if err != nil {
		return "", err
	}
	return GenerateOffset(pan, s.Pvk, pin, len(pin))
}

func (s *Ssm) SetKey(id, value string) {
	switch id {
	case "Cvk":
		s.Cvk = value
	case "Pvk":
		s.Pvk = value
	case "Tpk":
		s.Tpk = value
	case "Csd":
		s.Csd = value
	}
}
func (s *Ssm) GetKey(id string) string {
	switch id {
	case "Cvk":
		return s.Cvk
	case "Pvk":
		return s.Pvk
	case "Tpk":
		return s.Tpk
	case "Csd":
		return s.Csd
	}
	return ""
}

func PinBlock(pan, pin, tpk string) (string, error) {
	block1 := ""
	switch len(pin) {
	case 4:
		block1 = "04" + pin + "FFFFFFFFFF"
	case 5:
		block1 = "05" + pin + "FFFFFFFFF"
	case 6:
		block1 = "06" + pin + "FFFFFFFF"
	case 7:
		block1 = "07" + pin + "FFFFFFF"
	case 8:
		block1 = "08" + pin + "FFFFFF"
	case 9:
		block1 = "09" + pin + "FFFFF"
	case 10:
		block1 = "0A" + pin + "FFFF"
	case 11:
		block1 = "0B" + pin + "FFF"
	case 12:
		block1 = "0C" + pin + "FF"
	}
	//log.Println(block1)
	b1, err := hex.DecodeString(block1)
	if err != nil {
		return "Hex T1", response.ToErrorState(err)
	}
	block2 := "0000" + pan[3:15]
	//log.Println(block2)
	b2, err := hex.DecodeString(block2)
	if err != nil {
		return "Hex T2", response.ToErrorState(err)
	}

	tspXor := make([]byte, len(b1))
	for i := range b1 {
		tspXor[i] = b1[i] ^ b2[i]
	}

	tspHex := hex.EncodeToString(tspXor)

	//log.Println(tspHex)
	return EncryptDesHex(tspHex, tpk)
}

func (s *Ssm) PinBlock(pan, pin string) (string, error) {
	return PinBlock(pan, pin, s.Tpk)
}

func (s *Ssm) Mac(data string) (string, error) {
	return "0000000000000000", nil
}

func (s *Ssm) Translate(pan, pinBlock, tpk2nd string) (string, error) {
	pin, err := s.PinBlockDecode(pan, pinBlock)
	if err != nil {
		return "", err
	}
	return PinBlock(pan, pin, tpk2nd)
}

func (s *Ssm) Crypt(data, mode string) (string, error) {
	var result string
	var err error
	if mode == libCrypto.Decrypt {
		result, err = DecryptDesHex(data, s.Tpk)
	} else {
		result, err = EncryptDesHex(data, s.Tpk)
	}
	return result, err
}

func (s *Ssm) Cvv2Padding(cvv2 string) (string, error) {
	tspHex := fmt.Sprintf("%X%s", cvv2, strings.Repeat("30", 16-len(cvv2)))

	//log.Println(tspHex)
	return EncryptDesHex(tspHex, s.Csd)
}

func (s *Ssm) PinBlockDecode(pan, pinBlock string) (string, error) {
	tspHex, errDes := DecryptDesHex(pinBlock, s.Tpk)
	if errDes != nil {
		return "", errDes
	}

	tspXor, err := hex.DecodeString(tspHex)
	if err != nil {
		return "", response.ToErrorState(err)
	}
	block2 := "0000" + pan[3:15]
	//log.Println(block2)
	b2, err := hex.DecodeString(block2)
	if err != nil {
		return "Hex T2", response.ToErrorState(err)
	}

	b1 := make([]byte, len(tspXor))
	for i := range tspXor {
		b1[i] = tspXor[i] ^ b2[i]
	}
	block1 := hex.EncodeToString(b1)
	sLen := block1[:2]
	block1 = block1[2:]
	pin := ""
	switch sLen {
	case "04":
		pin = block1[:4]
	case "05":
		pin = block1[:5]
	case "06":
		pin = block1[:6]
	case "07":
		pin = block1[:7]
	case "08":
		pin = block1[:8]
	case "09":
		pin = block1[:9]
	case "0A":
		pin = block1[:10]
	case "0B":
		pin = block1[:11]
	case "0C":
		pin = block1[:12]
	}

	return pin, nil
}

func Initialize() {
	rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
}

func Init(cvk, pvk, tpk string) (*Ssm, error) {
	Initialize()
	return &Ssm{
		Cvk:  cvk,
		Pvk:  pvk,
		Tpk:  tpk,
		Pvki: "1",
	}, nil
}
