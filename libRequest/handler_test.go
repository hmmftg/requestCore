package libRequest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libContext"
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

		code, desc, arrErr, req, reqLog, err := Req[SampleBody, RequestHeader](w, JSON, true)
		if err != nil {
			t.Fatal(code, desc, arrErr, req, reqLog, err)
		}

		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != v.DesiredBody {
			t.Fatal("want:", v.DesiredBody, "got:", string(b))
		}
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

		code, desc, arrErr, req, reqLog, err := Req[SampleBody, RequestHeader](w, JSON, true)
		if err != nil {
			t.Fatal(code, desc, arrErr, req, reqLog, err)
		}

		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != v.DesiredBody {
			t.Fatal("want:", v.DesiredBody, "got:", string(b))
		}
	}
}
