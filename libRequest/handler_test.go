package libRequest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

func TestGinReq(t *testing.T) {
	type TestCase struct {
		Name          string
		Body          string
		Header        webFramework.HeaderInterface
		DesiredBody   string
		DesiredHeader string
	}
	type SampleBody struct {
		ID string `json:"id"`
	}
	table := []TestCase{
		{
			Name:          "valid",
			Body:          `{"id":"222222"}`,
			DesiredBody:   `{"id":"222222"}`,
			Header:        &RequestHeader{RequestID: "1111111111", User: "tester"},
			DesiredHeader: `{"id":"1111111111"}`,
		},
	}
	for _, v := range table {
		c := gin.Context{
			Request: &http.Request{
				Method: "POST",
				Body:   io.NopCloser(strings.NewReader(v.Body)),
				Header: make(http.Header),
			},
		}
		c.Request.Header.Add("Request-Id", v.Header.GetID())
		c.Request.Header.Add("User-Id", v.Header.GetUser())
		w := libContext.InitContext(&c)

		result, err := Req[SampleBody, RequestHeader](ParseParams{W: w, Mode: JSON, ValidateHeader: true})
		if err != nil {
			t.Fatal(err.Error())
		}

		b, errJSON := json.Marshal(result.Request)
		if errJSON != nil {
			t.Fatal(errJSON)
		}
		if string(b) != v.DesiredBody {
			t.Fatal("want:", v.DesiredBody, "got:", string(b))
		}
	}
}

func TestGinReq_ValidationFailureSetsSlogRequestBody(t *testing.T) {
	type validatedBody struct {
		Pin2   string `json:"pin2" validate:"required"`
		Cvv2   string `json:"cvv2" validate:"required"`
		Expiry string `json:"expiry" validate:"required"`
	}

	c := gin.Context{
		Request: &http.Request{
			Method: "POST",
			Body:   io.NopCloser(strings.NewReader(`{}`)),
			Header: make(http.Header),
		},
	}
	c.Request.Header.Add("Request-Id", "1111111111")
	c.Request.Header.Add("User-Id", "tester")
	w := libContext.InitContext(&c)

	_, err := Req[validatedBody, RequestHeader](ParseParams{W: w, Mode: JSON, ValidateHeader: false})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var errData libError.ErrorData
	if !errors.As(err, &errData) {
		t.Fatalf("expected libError.ErrorData, got %T", err)
	}
	if errData.ActionData.Description != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %q", errData.ActionData.Description)
	}

	body := w.Parser.GetLocal(libLogger.SlogRequestBody)
	if body == nil {
		t.Fatal("expected SlogRequestBody to be set on validation failure")
	}
	typed, ok := body.(validatedBody)
	if !ok {
		t.Fatalf("expected validatedBody, got %T", body)
	}
	if typed.Pin2 != "" || typed.Cvv2 != "" || typed.Expiry != "" {
		t.Fatalf("expected empty bound fields, got %+v", typed)
	}
}

func newGinRequestCtx(body string, params map[string]string) *gin.Context {
	c := &gin.Context{
		Request: &http.Request{
			Method: "POST",
			Body:   io.NopCloser(strings.NewReader(body)),
			Header: make(http.Header),
		},
	}
	c.Request.Header.Add("Request-Id", "1111111111")
	c.Request.Header.Add("User-Id", "tester")
	for k, v := range params {
		c.Params = append(c.Params, gin.Param{Key: k, Value: v})
	}
	return c
}

func assertErrorDescription(t *testing.T, err error, wantDesc string) {
	t.Helper()
	var errData libError.ErrorData
	if !errors.As(err, &errData) {
		t.Fatalf("expected libError.ErrorData, got %T (%v)", err, err)
	}
	if errData.ActionData.Description != wantDesc {
		t.Fatalf("want description %q, got %q", wantDesc, errData.ActionData.Description)
	}
	if errData.ActionData.Status != status.BadRequest {
		t.Fatalf("want status %d, got %d", status.BadRequest, errData.ActionData.Status)
	}
}

func TestGinReq_JSONOptional(t *testing.T) {
	type optionalBody struct {
		Cvv2   string `json:"cvv2"   validate:"omitempty,len=4"`
		Expiry string `json:"expiry" validate:"omitempty"`
	}
	type TestCase struct {
		Name     string
		Body     string
		WantErr  string
		WantCvv2 string
	}
	table := []TestCase{
		{Name: "empty body accepted", Body: "", WantCvv2: ""},
		{Name: "valid body bound", Body: `{"cvv2":"1234","expiry":"2603"}`, WantCvv2: "1234"},
		{Name: "bound body still validated", Body: `{"cvv2":"12"}`, WantErr: "VALIDATION_FAILED"},
		{Name: "malformed body rejected", Body: `{invalid`, WantErr: "ERROR_IN_GET_REQUEST_BODY"},
	}
	for _, v := range table {
		t.Run(v.Name, func(t *testing.T) {
			w := libContext.InitContext(newGinRequestCtx(v.Body, nil))

			result, err := Req[optionalBody, RequestHeader](ParseParams{W: w, Mode: JSONOptional, ValidateHeader: false})
			if v.WantErr != "" {
				assertErrorDescription(t, err, v.WantErr)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Request.Cvv2 != v.WantCvv2 {
				t.Fatalf("want cvv2 %q, got %q", v.WantCvv2, result.Request.Cvv2)
			}
		})
	}
}

func TestGinReq_JSONWithURIOptional(t *testing.T) {
	type optionalBodyWithURI struct {
		CardNumber string `uri:"cardNumber" validate:"required"`
		Cvv2       string `json:"cvv2"       validate:"omitempty"`
	}
	type TestCase struct {
		Name           string
		Body           string
		Params         map[string]string
		WantErr        string
		WantCardNumber string
		WantCvv2       string
	}
	table := []TestCase{
		{
			Name:           "empty body with uri param accepted",
			Body:           "",
			Params:         map[string]string{"cardNumber": "5022291044441234"},
			WantCardNumber: "5022291044441234",
		},
		{
			Name:           "body and uri param bound",
			Body:           `{"cvv2":"1234"}`,
			Params:         map[string]string{"cardNumber": "5022291044441234"},
			WantCardNumber: "5022291044441234",
			WantCvv2:       "1234",
		},
		{
			Name:    "missing uri param rejected",
			Body:    "",
			Params:  nil,
			WantErr: "VALIDATION_FAILED",
		},
		{
			Name:    "malformed body rejected",
			Body:    `{invalid`,
			Params:  map[string]string{"cardNumber": "5022291044441234"},
			WantErr: "ERROR_IN_GET_REQUEST_BODY",
		},
	}
	for _, v := range table {
		t.Run(v.Name, func(t *testing.T) {
			w := libContext.InitContext(newGinRequestCtx(v.Body, v.Params))

			result, err := Req[optionalBodyWithURI, RequestHeader](ParseParams{W: w, Mode: JSONWithURIOptional, ValidateHeader: false})
			if v.WantErr != "" {
				assertErrorDescription(t, err, v.WantErr)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Request.CardNumber != v.WantCardNumber {
				t.Fatalf("want cardNumber %q, got %q", v.WantCardNumber, result.Request.CardNumber)
			}
			if result.Request.Cvv2 != v.WantCvv2 {
				t.Fatalf("want cvv2 %q, got %q", v.WantCvv2, result.Request.Cvv2)
			}
		})
	}
}

func TestGinReq_RequiredModesRejectEmptyBody(t *testing.T) {
	type sampleBody struct {
		ID string `json:"id"`
	}
	for _, mode := range []Type{JSON, JSONWithURI} {
		t.Run(mode.String(), func(t *testing.T) {
			w := libContext.InitContext(newGinRequestCtx("", map[string]string{"id": "1"}))

			_, err := Req[sampleBody, RequestHeader](ParseParams{W: w, Mode: mode, ValidateHeader: false})
			assertErrorDescription(t, err, "ERROR_IN_GET_REQUEST_BODY")
		})
	}
}

func TestFiberReq(t *testing.T) {
	type TestCase struct {
		Name          string
		Body          string
		Header        webFramework.HeaderInterface
		DesiredBody   string
		DesiredHeader string
	}
	type SampleBody struct {
		ID string `json:"id"`
	}
	table := []TestCase{
		{
			Name:          "valid",
			Body:          `{"id":"222222"}`,
			DesiredBody:   `{"id":"222222"}`,
			Header:        &RequestHeader{RequestID: "1111111111"},
			DesiredHeader: `{"id":"1111111111"}`,
		},
	}
	app := fiber.New()
	for _, v := range table {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		c.Request().Header.SetContentType(fiber.MIMEApplicationJSON)
		bodyBytes := []byte(v.Body)
		c.Request().SetBody(bodyBytes)
		c.Request().Header.SetContentLength(len(bodyBytes))
		defer app.ReleaseCtx(c)

		c.Request().Header.Add("Request-Id", v.Header.GetID())
		w := libContext.InitContext(c)

		result, err := Req[SampleBody, RequestHeader](ParseParams{W: w, Mode: JSON, ValidateHeader: true})
		if err != nil {
			t.Fatal(err)
		}

		b, errJSON := json.Marshal(result.Request)
		if errJSON != nil {
			t.Fatal(errJSON)
		}
		if string(b) != v.DesiredBody {
			t.Fatal("want:", v.DesiredBody, "got:", string(b))
		}
	}
}
