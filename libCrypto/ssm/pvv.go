package ssm

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"

	"github.com/hmmftg/requestCore/response"

	"golang.org/x/exp/slices"
)

func ExtractPanFromCard(card string) (string, error) {
	if len(card) != 16 {
		return "", errors.New("Invalid Card length: " + card)
	}

	return card[3:15], nil
}

var BadPins []int

func IsPinEasy(pin int) bool {

	if BadPins == nil {
		BadPins = []int{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
			//0
			111, 222, 333, 444, 555, 666, 777, 888, 999,
			1110, 2220, 3330, 4440, 5550, 6660, 7770, 8880, 9990,
			//1
			1000, 1222, 1333, 1444, 1555, 1666, 1777, 1888, 1999,
			1111, 2221, 3331, 4441, 5551, 6661, 7771, 8881, 9991,
			//2
			2000, 2111, 2333, 2444, 2555, 2666, 2777, 2888, 2999,
			1112, 2222, 3332, 4442, 5552, 6662, 7772, 8882, 9992,
			//3
			3000, 3111, 3222, 3444, 3555, 3666, 3777, 3888, 3999,
			1113, 2223, 3333, 4443, 5553, 6663, 7773, 8883, 9993,
			//4
			4000, 4111, 4222, 4333, 4555, 4666, 4777, 4888, 4999,
			1114, 2224, 3334, 4444, 5554, 6664, 7774, 8884, 9994,
			//5
			5000, 5111, 5222, 5333, 5444, 5666, 5777, 5888, 5999,
			1115, 2225, 3335, 4445, 5555, 6665, 7775, 8885, 9995,
			//6
			6000, 6111, 6222, 6333, 6444, 6555, 6777, 6888, 6999,
			1116, 2226, 3336, 4446, 5556, 6666, 7776, 8886, 9996,
			//7
			7000, 7111, 7222, 7333, 7444, 7555, 7666, 7888, 7999,
			1117, 2227, 3337, 4447, 5557, 6667, 7777, 8887, 9997,
			//8
			8000, 8111, 8222, 8333, 8444, 8555, 8666, 8777, 8999,
			1118, 2228, 3338, 4448, 5558, 6668, 7778, 8888, 9998,
			//9
			9000, 9111, 9222, 9333, 9444, 9555, 9666, 9777, 9888,
			1119, 2229, 3339, 4449, 5559, 6669, 7779, 8889, 9999,
			//seq
			123, 234, 345, 456, 567, 678, 789,
			987, 876, 765, 654, 543, 432, 321,
			1234, 2345, 3456, 4567, 5678, 6789, 7890, 8901, 9012,
			3210, 4321, 5432, 6543, 7654, 8765, 9876, 1098, 2109,
		}
	}
	pinS := fmt.Sprintf("%04d", pin)
	pinFirst4 := pinS[:4]
	pinLast4 := pinS[len(pinS)-4:]
	pinMid4 := pinS[1 : len(pinS)-1]
	pinDiv10, _ := strconv.Atoi(pinFirst4)
	pinMod10, _ := strconv.Atoi(pinLast4)
	pinMid10, _ := strconv.Atoi(pinMid4)
	return !(slices.IndexFunc(BadPins, func(badPin int) bool { return pin == badPin }) == -1 &&
		slices.IndexFunc(BadPins, func(badPin int) bool { return pinDiv10 == badPin }) == -1 &&
		slices.IndexFunc(BadPins, func(badPin int) bool { return pinMod10 == badPin }) == -1 &&
		slices.IndexFunc(BadPins, func(badPin int) bool { return pinMid10 == badPin }) == -1)
}

func GenerateRandomPin(len int) string {
	if len == 4 {
		nPin := rand.Intn(9999)
		for IsPinEasy(nPin) {
			nPin = rand.Intn(9999)
		}
		return fmt.Sprintf("%04d", nPin)
	}
	max := (int)(math.Pow10(len))
	nPin := rand.Intn(max)
	for IsPinEasy(nPin) {
		nPin = rand.Intn(max)
	}
	return fmt.Sprintf("%0*d", len, nPin)
}

const DECIMALIZATION_TABLE string = "0123456789012345"

func Decimalize(notDecimaliz string, length int, immediate bool) string {
	lenTarget := len(notDecimaliz)
	desiredLen := length
	if lenTarget < length {
		desiredLen = lenTarget
	}
	decimalized := ""
	if !immediate {
		for i := 0; i < lenTarget && len(decimalized) < desiredLen; i++ {
			if notDecimaliz[i] >= '0' && notDecimaliz[i] <= '9' {
				id, _ := strconv.Atoi(string(notDecimaliz[i]))
				decimalized += string(DECIMALIZATION_TABLE[id])
			}
		}
		if len(decimalized) == desiredLen {
			return decimalized
		}
		for i := 0; i < lenTarget && len(decimalized) < desiredLen; i++ {
			if notDecimaliz[i] >= 'A' && notDecimaliz[i] <= 'F' {
				id, _ := strconv.Atoi(string(notDecimaliz[i] - 'A' + '0'))
				decimalized += string(DECIMALIZATION_TABLE[id+10])
			}
		}
		return decimalized
	}
	for i := 0; i < lenTarget && len(decimalized) < desiredLen; i++ {
		if notDecimaliz[i] >= '0' && notDecimaliz[i] <= '9' {
			id, _ := strconv.Atoi(string(notDecimaliz[i]))
			decimalized += string(DECIMALIZATION_TABLE[id])
		} else {
			id, _ := strconv.Atoi(string(notDecimaliz[i] - 'A' + '0'))
			decimalized += string(DECIMALIZATION_TABLE[id+10])
		}
	}
	return decimalized
}

func GeneratePinBlock(pan, tpkB64, pin string) (string, error) {
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
	b1, err := hex.DecodeString(block1)
	if err != nil {
		return "Hex B1", err
	}
	block2 := "0000" + pan[3:15]
	b2, err := hex.DecodeString(block2)
	if err != nil {
		return "Hex B2", err
	}

	tspXor := make([]byte, len(b1))
	for i := range b1 {
		tspXor[i] = b1[i] ^ b2[i]
	}

	tspHex := hex.EncodeToString(tspXor)

	return EncryptDesHex(tspHex, tpkB64)
}

func GeneratePvv(card, pvkB64, pvki, pin string) (string, error) {
	pan, err := ExtractPanFromCard(card)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	tsp, err := hex.DecodeString(pan[1:] + pvki + pin)
	if err != nil {
		return "", response.ToErrorState(err)
	}
	tspB64 := base64.StdEncoding.EncodeToString(tsp)

	cipherTspB64, err := EncryptDes(tspB64, pvkB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	cipherTsp, err := base64.StdEncoding.DecodeString(cipherTspB64)
	if err != nil {
		return "", response.ToErrorState(err)
	}

	pvvHex := hex.EncodeToString(cipherTsp[:8])

	return Decimalize(pvvHex, 4, false), nil
}

func SubTen(pin, decimalized string, length int) (string, error) {
	sub := ""
	x1 := "1"
	y1 := ""
	for i := 0; i < length; i++ {
		x1 += pin[i : i+1]
		y1 += decimalized[i : i+1]
		xn1, _ := strconv.Atoi(x1)
		yn1, _ := strconv.Atoi(y1)
		carry := strconv.Itoa(xn1 - yn1)
		sub += carry[len(carry)-1:]
	}
	return sub, nil
}

func GenerateOffset(card, pvkB64, pin string, pinlen int) (string, error) {
	cipherHex, err := EncryptDesHex(card, pvkB64)
	if err != nil {
		return "", err
	}

	cipherDecim := Decimalize(cipherHex, pinlen, true)

	return SubTen(pin, cipherDecim, pinlen)
}
