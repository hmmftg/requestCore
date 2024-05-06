package libCrypto

import "github.com/hmmftg/requestCore/response"

type Sm interface {
	Cvv(pan, exp, cvvType string) (string, response.ErrorState)
	Pvv(pan, pinBlock string) (string, response.ErrorState)
	Offset(pan, pinBlock string) (string, response.ErrorState)
}

const (
	Cvv1 = "Cvv1"
	Cvv2 = "Cvv2"
)
