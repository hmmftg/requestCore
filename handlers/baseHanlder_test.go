package handlers

import (
	"log"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/testingtools"
)

type testReq struct {
	ID string `json:"id" validate:"required"`
}

type testResp struct {
	Result string `json:"result"`
}

type testHandlerType[Req testReq, Resp testResp] struct {
	Title        string
	Path         string
	Mode         libRequest.Type
	VerifyHeader bool
	SaveRequest  bool
}

func (h testHandlerType[Req, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{
		h.Title,
		h.Mode,
		h.VerifyHeader,
		h.SaveRequest,
		h.Path,
		false,
		nil,
		false,
		nil,
		nil,
	}
}
func (h testHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	log.Println("Initializer")
	return nil
}
func (h testHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	log.Println("Handler")
	result := testResp{Result: "a"}
	return Resp(result), nil
}
func (h testHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	log.Println("Simulation")
	result := testResp{Result: "a"}
	return Resp(result), nil
}
func (h testHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
	log.Println("Finalizer")
}
func TestBaseHandler(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/",
			Request:   testReq{ID: "1"},
			Status:    200,
			CheckBody: []string{"result", `"a"`},
		},
		{
			Name:    "Invalid Request",
			Url:     "/",
			Request: map[string]any{"ss": "a"},
			Status:  400,
		},
	}

	env := testingtools.GetEnvWithDB[testingtools.TestEnv](
		testingtools.SampleRequestModelMock(t, nil).DB,
		testingtools.DefaultAPIList,
	)

	handler := BaseHandler(
		env.Interface,
		testHandlerType[testReq, testResp]{
			Title:        "test",
			Path:         "/path/to/api",
			SaveRequest:  true,
			Mode:         libRequest.JSON,
			VerifyHeader: true,
		},
		false,
	)
	gin.SetMode(gin.ReleaseMode)
	testingtools.TestAPI(t, testCases, &testingtools.TestOptions{
		Path:    "/",
		Name:    "check base handler",
		Method:  "POST",
		Handler: handler,
		Silent:  true,
	})
}
