package libCrypto

import "github.com/hmmftg/requestCore/response"

type Sm interface {
	SetKey(id, value string)
	GetKey(id string) string
	Cvv(pan, exp, cvvType string) (string, response.ErrorState)
	Pvv(pan, pinBlock string) (string, response.ErrorState)
	Offset(pan, pinBlock string) (string, response.ErrorState)
	Mac(data string) (string, response.ErrorState)
	Translate(pan, pinBlock, tpk2nd string) (string, response.ErrorState)
	Crypt(data, mode string) (string, response.ErrorState)
}

const (
	Cvv1    = "Cvv1"
	Cvv2    = "Cvv2"
	Encrypt = "E"
	Decrypt = "D"
)
