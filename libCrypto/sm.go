package libCrypto

type Sm interface {
	SetKey(id, value string)
	GetKey(id string) string
	Cvv(pan, exp, cvvType string) (string, error)
	Cvv2Padding(cvv2 string) (string, error)
	Pvv(pan, pinBlock string) (string, error)
	Offset(pan, pinBlock string) (string, error)
	Mac(data string) (string, error)
	Translate(pan, pinBlock, tpk2nd string) (string, error)
	Crypt(data, mode string) (string, error)
}

const (
	Cvv1    = "Cvv1"
	Cvv2    = "Cvv2"
	Encrypt = "E"
	Decrypt = "D"
)
