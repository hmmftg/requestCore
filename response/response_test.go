package response

import (
	"encoding/json"
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
