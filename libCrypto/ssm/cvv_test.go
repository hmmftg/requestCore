package ssm

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestCvv(t *testing.T) {
	cvk, _ := hex.DecodeString("161916DA861AE3107AAE5816C25126A8")
	cvkB64 := base64.StdEncoding.EncodeToString(cvk)
	service1 := "506"
	service2 := "000"
	tables := []struct {
		pan     string
		exp     string
		service string
		cvk     string
		cvv     string
	}{
		{"5894631800000000", "0001", service1, cvkB64, "142"},
		{"5894631800000000", "0100", service2, cvkB64, "750"},
		{"6037998700001497", "1005", service2, cvkB64, "264"},
	}

	for _, table := range tables {
		cvv, err := GenCvv(table.pan, table.exp, table.service, table.cvk)
		if err != nil {
			t.Error(err)
		} else if cvv != table.cvv {
			t.Errorf("CVV of (%s,%s) was incorrect, got: %s, want: %s.", table.pan, table.exp, cvv, table.cvv)
		}
	}
}
