package handlers

import (
	"log"
	"sync/atomic"
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

type testPersister[Req, Resp any] struct {
	updateCalled *atomic.Bool
}

func (p testPersister[Req, Resp]) Insert(path string, req *HandlerRequest[Req, Resp]) error {
	return req.Core.RequestTools().InitRequest(req.W, req.Title, path)
}

func (p testPersister[Req, Resp]) Update(_ string, _ *HandlerRequest[Req, Resp]) error {
	if p.updateCalled != nil {
		p.updateCalled.Store(true)
	}
	return nil
}

type testHandlerType[Req testReq, Resp testResp] struct {
	Title        string
	Path         string
	Mode         libRequest.Type
	VerifyHeader bool
	Persistence  RequestPersister[Req, Resp]
}

func (h testHandlerType[Req, Resp]) Parameters() HandlerParameters[Req, Resp] {
	return HandlerParameters[Req, Resp]{
		Title:           h.Title,
		Body:            h.Mode,
		ValidateHeader:  h.VerifyHeader,
		Persistence:     h.Persistence,
		Path:            h.Path,
		HasReceipt:      false,
		RecoveryHandler: nil,
		FileResponse:    false,
		LogArrays:       nil,
		LogTags:         nil,
		EnableTracing:   false,
		TracingSpanName: "",
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
			Persistence:  testPersister[testReq, testResp]{},
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

func TestBaseHandlerPersistenceUpdate(t *testing.T) {
	var updateCalled atomic.Bool

	env := testingtools.GetEnvWithDB[testingtools.TestEnv](
		testingtools.SampleRequestModelMock(t, nil).DB,
		testingtools.DefaultAPIList,
	)

	handler := BaseHandler(
		env.Interface,
		testHandlerType[testReq, testResp]{
			Title:        "test",
			Path:         "/path/to/api",
			Persistence:  testPersister[testReq, testResp]{updateCalled: &updateCalled},
			Mode:         libRequest.JSON,
			VerifyHeader: true,
		},
		false,
	)
	gin.SetMode(gin.ReleaseMode)
	testingtools.TestDB(t, &testingtools.TestCase{
		Name:      "Valid",
		Url:       "/",
		Request:   testReq{ID: "1"},
		Status:    200,
		CheckBody: []string{"result", `"a"`},
	}, &testingtools.TestOptions{
		Path:    "/",
		Name:    "check persistence update",
		Method:  "POST",
		Handler: handler,
		Silent:  true,
	})

	if !updateCalled.Load() {
		t.Fatal("expected Persistence.Update to be called after successful Insert")
	}
}

func TestBaseHandlerNoPersistence(t *testing.T) {
	env := testingtools.GetEnvWithDB[testingtools.TestEnv](
		testingtools.SampleRequestModelMock(t, nil).DB,
		testingtools.DefaultAPIList,
	)

	handler := BaseHandler(
		env.Interface,
		testHandlerType[testReq, testResp]{
			Title:        "test",
			Path:         "/path/to/api",
			Persistence:  nil,
			Mode:         libRequest.JSON,
			VerifyHeader: true,
		},
		false,
	)
	gin.SetMode(gin.ReleaseMode)
	testingtools.TestDB(t, &testingtools.TestCase{
		Name:      "Valid",
		Url:       "/",
		Request:   testReq{ID: "1"},
		Status:    200,
		CheckBody: []string{"result", `"a"`},
	}, &testingtools.TestOptions{
		Path:    "/",
		Name:    "check no persistence",
		Method:  "POST",
		Handler: handler,
		Silent:  true,
	})
}
