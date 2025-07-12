package utimaco

import (
	"testing"

	"github.com/hmmftg/requestCore/libCrypto"
)

func TestInit(t *testing.T) {
	utimaco, err := Init("10.15.1.61", "9007",
		"D2305F457EB73E3DDE47B5A247B56C51E23BB763CFB869BA7446",
		"D2304AA666DF48D38F8B055FDEA39B7B7F7174CD9027E4520D98",
		"D130AECED5596486EB2815126E3B121503F2",
	)
	if err != nil {
		t.Fatalf(`Init(ip,port) => %v`, err)
	}
	pan := "5894631801747064"
	resp, err := utimaco.Pvv(pan, "B80DC6BA6A9C8E4F")
	want := "0286"
	if err != nil || resp != want {
		t.Fatalf(`Pvv(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}
	resp, err = utimaco.Offset(pan, "E5A3FDA49D2D6FFC")
	want = "894007"
	if err != nil || resp != want {
		t.Fatalf(`Offset(pan,pb,tpk) = %s, %v, want: %s`, resp, err, want)
	}
	resp, err = utimaco.Cvv(pan, "1234", libCrypto.Cvv1)
	want = "309"
	if err != nil || resp != want {
		t.Fatalf(`Cvv1(pan,exp) = %s, %v, want: %s`, resp, err, want)
	}
	resp, err = utimaco.Cvv(pan, "1234", libCrypto.Cvv2)
	want = "666"
	if err != nil || resp != want {
		t.Fatalf(`Cvv2(pan,exp) = %s, %v, want: %s`, resp, err, want)
	}
}
