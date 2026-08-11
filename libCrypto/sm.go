// Package libCrypto provides cryptographic utility wrappers.
package libCrypto

// Sm is the interface for security module cryptographic operations (CVV, PVV, MAC, encryption).
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
	// Cvv1 is the constant identifying CVV1 card verification value generation.
	Cvv1 = "Cvv1"
	// Cvv2 is the constant identifying CVV2 card verification value generation.
	Cvv2 = "Cvv2"
	// Encrypt is the mode constant for encryption operations.
	Encrypt = "E"
	// Decrypt is the mode constant for decryption operations.
	Decrypt = "D"
)
