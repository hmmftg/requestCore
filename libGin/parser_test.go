package libGin

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/webFramework"
)

func TestGetBody(t *testing.T) {
	type TestCase struct {
		Name        string
		Body        string
		Target      any
		DesiredJSON any
	}
	type SampleType struct {
		ID string `json:"id"`
	}
	var sampleType SampleType
	table := []TestCase{
		{
			Name:        "valid",
			Body:        `{"id":"1"}`,
			Target:      sampleType,
			DesiredJSON: `{"id":"1"}`,
		},
	}
	for _, v := range table {
		c := gin.Context{Request: &http.Request{Body: io.NopCloser(strings.NewReader(v.Body))}}
		ctx := InitContext(&c)
		err := ctx.GetBody(&v.Target)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(v.Target)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != v.DesiredJSON {
			t.Fatal("want:", v.DesiredJSON, "got:", string(b))
		}
	}
}

type Header struct {
	ID string `header:"id" reqHeader:"id" json:"id"`
}

func (h Header) GetId() string      { return h.ID }
func (h Header) GetUser() string    { return h.ID }
func (h Header) GetBranch() string  { return h.ID }
func (h Header) GetBank() string    { return h.ID }
func (h Header) GetPerson() string  { return h.ID }
func (h Header) GetProgram() string { return h.ID }
func (h Header) GetModule() string  { return h.ID }
func (h Header) GetMethod() string  { return h.ID }
func (h Header) SetUser(string)     {}
func (h Header) SetBranch(string)   {}
func (h Header) SetBank(string)     {}
func (h Header) SetPerson(string)   {}
func (h Header) SetProgram(string)  {}
func (h Header) SetModule(string)   {}
func (h Header) SetMethod(string)   {}

func TestGetHeader(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	type TestCase struct {
		Name        string
		Header      webFramework.HeaderInterface
		DesiredJSON any
	}
	table := []TestCase{
		{
			Name:        "valid",
			Header:      Header{ID: "1"},
			DesiredJSON: `{"id":"1"}`,
		},
	}
	for _, v := range table {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/", nil)
		c.Request.Header.Add("id", v.Header.GetId())
		ctx := InitContext(c)
		var sampleType Header
		err := ctx.GetHeader(&sampleType)
		if err != nil {
			t.Fatal(v.Name, err)
		}
		b, err := json.Marshal(sampleType)
		if err != nil {
			t.Fatal(v.Name, err)
		}
		if string(b) != v.DesiredJSON {
			t.Fatal(v.Name, "want:", v.DesiredJSON, "got:", string(b))
		}
	}
}
