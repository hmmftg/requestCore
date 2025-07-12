package ssm

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"testing"
)

func TestPvv(t *testing.T) {
	pvk, _ := hex.DecodeString("6E54A2F167F8CDFB292C2CBF5B0B8C34")
	pvkB64 := base64.StdEncoding.EncodeToString(pvk)
	pvki := "1"
	tables := []struct {
		pan  string
		pvk  string
		pvki string
		pin  string
		pvv  string
	}{
		{"6037998700001497", pvkB64, pvki, "3503", "9582"},
		{"6037998700001497", pvkB64, pvki, "1234", "5813"},
	}

	for _, table := range tables {
		pvv, err := GeneratePvv(table.pan, table.pvk, table.pvki, table.pin)
		//t.Logf("Pvv(%s,%s) => %s ? %s", table.pan, table.pin, pvv, table.pvv)
		if err != nil {
			t.Error(err)
		} else if pvv != table.pvv {
			t.Errorf("PVV of (%s,%s) was incorrect, got: %s, want: %s.", table.pan, table.pin, pvv, table.pvv)
		}
	}
}

func TestGenPin(t *testing.T) {
	tables := []struct {
		len int
		pin string
	}{
		{4, "1111"},
		{4, "1111"},
		{4, "1111"},
		{5, "11111"},
		{5, "11111"},
		{5, "11111"},
		{6, "11111"},
		{6, "11111"},
		{6, "11111"},
	}

	for _, table := range tables {
		pin := GenerateRandomPin(table.len)
		t.Log(pin)
		nPin, err := strconv.Atoi(pin)
		if err != nil {
			t.Error(err)
		} else if IsPinEasy(nPin) {
			t.Errorf("Pin of length (%d) was weak: %s", table.len, pin)
		}
	}
}

func TestWeakPin(t *testing.T) {
	tables := []struct {
		pin  int
		weak bool
	}{
		{411111, true},
		{4111111, true},
		{41111111, true},
		{41111, true},
		{11114, true},
		{111114, true},
		{1111114, true},
		{11111114, true},
		{1475, false},
		{1111, true},
	}

	for _, table := range tables {
		weak := IsPinEasy(table.pin)
		if weak != table.weak {
			t.Errorf("Weak detection for (%d) incorrect, got: %v, want: %v.", table.pin, weak, table.weak)
		}
	}
}

func TestOffset(t *testing.T) {
	pvk, _ := hex.DecodeString("6E54A2F167F8CDFB292C2CBF5B0B8C34")
	pvkB64 := base64.StdEncoding.EncodeToString(pvk)
	pvk2, _ := hex.DecodeString("0123456789ABCDEFFEDCBA9876543210")
	pvk2B64 := base64.StdEncoding.EncodeToString(pvk2)
	tables := []struct {
		pan    string
		pvk    string
		pin    string
		pinlen int
		offset string
	}{
		{"456789987654FFFF", pvk2B64, "3196", 4, "0859"},
		{"6037998700001497", pvkB64, "123456", 6, "860140"},
		{"6037998700001497", pvkB64, "654321", 6, "391015"},
		{"6037998700001497", pvkB64, "000000", 6, "747794"},
	}

	for _, table := range tables {
		offset, err := GenerateOffset(table.pan, table.pvk, table.pin, table.pinlen)
		t.Logf("Offset(%s,%s) => %s ? %s", table.pan, table.pin, offset, table.offset)
		if err != nil {
			t.Error(err)
		} else if offset != table.offset {
			t.Errorf("Offset of (%s,%s) was incorrect, got: %s, want: %s.", table.pan, table.pin, offset, table.offset)
		}
	}
}
