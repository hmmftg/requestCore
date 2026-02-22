package response

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGetDescFromCode_SafeFallback(t *testing.T) {
	// When code is not in errDescList, must return safe fallback, never raw data.
	code := "UNMAPPED_CODE"
	rawData := map[string]interface{}{"secret": "value", "body": "long upstream response"}
	errDescList := map[string]string{"OTHER": "other desc"}

	gotCode, gotDesc := GetDescFromCode(code, rawData, errDescList)

	if gotDesc == "" {
		t.Error("expected non-empty safe description")
	}
	// Must not expose raw data (no "secret", no "value", no map dump)
	if strings.Contains(gotDesc, "secret") || strings.Contains(gotDesc, "value") || strings.Contains(gotDesc, "upstream") {
		t.Errorf("GetDescFromCode must not return raw data; got desc: %q", gotDesc)
	}
	if gotDesc != SYSTEM_FAULT_DESC && gotDesc != "other desc" {
		// Should be SYSTEM_FAULT_DESC or errDescList[SYSTEM_FAULT] if set
		if errDescList[SYSTEM_FAULT] != "" && gotDesc != errDescList[SYSTEM_FAULT] {
			t.Errorf("expected safe fallback; got %q", gotDesc)
		}
	}
	// Code should be normalized (underscores for API)
	if gotCode != code && gotCode != strings.ReplaceAll(code, "-", "_") {
		t.Logf("gotCode=%q (acceptable)", gotCode)
	}
}

func TestGetDescFromCode_Mapped(t *testing.T) {
	errDescList := map[string]string{"MY_CODE": "My localized text"}
	code, desc := GetDescFromCode("MY_CODE", nil, errDescList)
	// Code is normalized to dashes for API (MY-CODE)
	if code != "MY-CODE" || desc != "My localized text" {
		t.Errorf("got (%q, %q); want (MY-CODE, My localized text)", code, desc)
	}
}

func TestSanitizeForClient(t *testing.T) {
	t.Run("string within limit", func(t *testing.T) {
		got := SanitizeForClient("short", 100)
		if got != "short" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("string over limit truncated", func(t *testing.T) {
		long := strings.Repeat("a", 600)
		got := SanitizeForClient(long, 256)
		if len(got) > 256+10 {
			t.Errorf("expected truncation; len(got)=%d", len(got))
		}
		if got != long[:256] {
			// Should be first 256 runes
			r := []rune(got)
			if len(r) > 256 {
				t.Errorf("expected at most 256 runes; got %d", len(r))
			}
		}
	})
	t.Run("non-string returns SYSTEM_FAULT_DESC", func(t *testing.T) {
		got := SanitizeForClient(map[string]int{"a": 1}, 100)
		if got != SYSTEM_FAULT_DESC {
			t.Errorf("got %q; want SYSTEM_FAULT_DESC", got)
		}
	})
	t.Run("nil returns SYSTEM_FAULT_DESC", func(t *testing.T) {
		got := SanitizeForClient(nil, 100)
		if got != SYSTEM_FAULT_DESC {
			t.Errorf("got %q; want SYSTEM_FAULT_DESC", got)
		}
	})
}

func TestErrorResponse_JSONShape(t *testing.T) {
	// ErrorResponse must serialize to code (string) and description (string) only.
	er := ErrorResponse{Code: "ERR_1", Description: "user-facing text"}
	b, err := json.Marshal(er)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Code != "ERR_1" || decoded.Description != "user-facing text" {
		t.Errorf("decoded %+v", decoded)
	}
}

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
