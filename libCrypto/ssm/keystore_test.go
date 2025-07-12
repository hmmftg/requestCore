package ssm

import (
	"os"
	"reflect"
	"testing"
)

func TestKeystore(t *testing.T) {
	password := []byte("12345678")
	defer Zeroing(password)

	ks1 := ReadKeyStore("keystore.jks", password)

	WriteKeyStore(ks1, "keystore2.jks", []byte("12345678"))

	ks2 := ReadKeyStore("keystore2.jks", []byte("12345678"))

	if !reflect.DeepEqual(ks1, ks2) {
		t.Fatalf("out puts are not equal: %v, %v", ks1, ks2)
	}

	err := os.Remove("keystore2.jks")
	if err != nil {
		t.Fatalf("can't remove keystore2.jks")
	}
}
