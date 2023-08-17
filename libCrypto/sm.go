package libCrypto

type Sm interface {
	Cvv(pan, exp, cvvType string) (string, error)
	Pvv(pan, pinBlock string) (string, error)
	Offset(pan, pinBlock string) (string, error)
}

const (
	Cvv1 = "Cvv1"
	Cvv2 = "Cvv2"
)
