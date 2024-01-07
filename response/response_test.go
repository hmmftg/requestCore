package response

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestWsResponse(t *testing.T) {
	type TestCase struct {
		Name    string
		Resp    WsResponse
		Desired string
	}
	testCases := []TestCase{
		{
			Name:    "emptyReceipt",
			Resp:    WsResponse{Status: 1, Description: "aa", Result: "bb", ErrorData: nil, PrintReceipt: nil},
			Desired: `{"status":1,"description":"aa","result":"bb"}`,
		},
		{
			Name:    "filledReceipt",
			Resp:    WsResponse{Status: 1, Description: "aa", Result: "bb", ErrorData: nil, PrintReceipt: &Receipt{Id: "pp", Title: "rr"}},
			Desired: `{"status":1,"description":"aa","result":"bb","printReceipt":{"id":"pp","title":"rr","rows":null}}`,
		},
	}
	for _, tst := range testCases {
		result, err := json.Marshal(tst.Resp)
		if err != nil {
			t.Fatal(tst.Name, tst.Resp, err)
		}
		if string(result) != tst.Desired {
			t.Fatal(string(result), "!=", tst.Desired)
		}
	}
}

func fakeErrorCaller(depth int) ErrorState {
	if depth == 0 {
		err := fmt.Errorf("err: %d", depth)
		return toErrorState(err, depth)
	}
	return Errors(-1, "faker", depth, fakeErrorCaller(depth-1))
}

func TestErrorState(t *testing.T) {
	type TestCase struct {
		Name          string
		CallDepth     int
		DesiredSrc    string
		NotDesiredSrc string
	}
	testCases := []TestCase{
		{
			Name:          "depth1",
			CallDepth:     1,
			DesiredSrc:    "response/response_test.go",
			NotDesiredSrc: "response/response.go",
		},
		{
			Name:          "depth2",
			CallDepth:     2,
			DesiredSrc:    "response/response_test.go",
			NotDesiredSrc: "response/response.go",
		},
		{
			Name:          "depth3",
			CallDepth:     3,
			DesiredSrc:    "response/response_test.go",
			NotDesiredSrc: "response/response.go",
		},
	}
	for _, tst := range testCases {
		err := fakeErrorCaller(tst.CallDepth)
		result := err.Error()
		if !strings.Contains(result, tst.DesiredSrc) {
			t.Fatal(result, " does not contain ", tst.DesiredSrc)
		}
		if strings.Contains(result, tst.NotDesiredSrc) {
			t.Fatal(result, " contains ", tst.NotDesiredSrc)
		}
	}
}
