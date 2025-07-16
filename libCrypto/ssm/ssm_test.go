package ssm

import (
	"testing"

	"github.com/hmmftg/requestCore/libCrypto"
)

func TestPinBlock(t *testing.T) {
	sm := Ssm{
		Tpk: "HBwcHBwcHBwcHBwcHBwcHA==",
		Pvk: "QEYQEBUjYYNSVwQ0hjgQUg==",
		Cvk: "c3VRCJiAc2EBEAE3RZFUQA==",
		Csd: "HBwcHBwcHBwcHBwcHBwcHA==",
	}
	resp, err := sm.PinBlock("1111111111111111", "1111")
	want := "3BB2D96604C3D1D7"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}
	resp, err = sm.PinBlock("5894631889333878", "4321")
	want = "5773869AA07E4260"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}
	resp, err = sm.PinBlock("5894631889333878", "52416378")
	want = "52C97037137BF51E"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}

	resp, err = sm.Pvv("5894631801747064", "B80DC6BA6A9C8E4F")
	want = "0286"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}

	resp, err = sm.Offset("5894631801747064", "E5A3FDA49D2D6FFC")
	want = "894007"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}

	resp, err = sm.Cvv("5894631801747064", "1234", libCrypto.Cvv1)
	want = "309"
	if err != nil || resp != want {
		t.Fatalf(`Cvv(pan,exp) = %s, %v, want: %s`, resp, err, want)
	}

	resp, err = sm.Cvv("5894631801747064", "1234", libCrypto.Cvv2)
	want = "666"
	if err != nil || resp != want {
		t.Fatalf(`Cvv(pan,exp) = %s, %v, want: %s`, resp, err, want)
	}

	resp, err = sm.Cvv2Padding("1234")
	want = "D2CBF605E0978213585220B9B7D1BFC4"
	if err != nil || resp != want {
		t.Fatalf(`CvvPadding(cvv2) = %s, %v, want: %s`, resp, err, want)
	}
}
