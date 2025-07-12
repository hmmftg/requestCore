package ssm

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestDes(t *testing.T) {
	tables := []struct {
		key    string
		data   string
		cipher string
	}{
		{"161916DA861AE3107AAE5816C25126A8", "0000000000000000", "FF9A124A6744AC36"},
		{"6E54A2F167F8CDFB292C2CBF5B0B8C34", "0000000000000000", "B36BA7C0374D0B22"},
	}

	for _, table := range tables {
		keyByte, _ := hex.DecodeString(table.key)
		dataByte, _ := hex.DecodeString(table.data)
		keyB64 := base64.StdEncoding.EncodeToString(keyByte)
		dataB64 := base64.StdEncoding.EncodeToString(dataByte)
		cipherB64, err := EncryptDes(dataB64, keyB64)
		if err != nil {
			t.Error(err)
		} else {
			cipherByte, err := base64.StdEncoding.DecodeString(cipherB64)
			cipher := strings.ToUpper(hex.EncodeToString(cipherByte))
			if err != nil {
				t.Error(err)
			} else if cipher != table.cipher {
				t.Errorf("DES of (%s,%s) was incorrect, got: %s, want: %s.", table.data, table.key, cipher, table.cipher)
			}
		}
	}
}
