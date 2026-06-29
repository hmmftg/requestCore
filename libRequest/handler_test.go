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
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/valyala/fasthttp"
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
			Header:        &RequestHeader{RequestId: "1111111111", User: "tester"},
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
		c.Request.Header.Add("Request-Id", v.Header.GetId())
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
			Header:        &RequestHeader{RequestId: "1111111111"},
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

		c.Request().Header.Add("Request-Id", v.Header.GetId())
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
