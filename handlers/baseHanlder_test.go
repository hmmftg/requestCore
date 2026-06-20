package handlers

import (
	"log"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/testingtools"
)

type testReq struct {
	ID string `json:"id" validate:"required"`
}

type testResp struct {
	Result string `json:"result"`
}

type capturingPersister[Req, Resp any] struct {
	updateCalled *atomic.Bool
	lastUpdated  **HandlerRequest[Req, Resp]
}

func (p capturingPersister[Req, Resp]) Insert(path string, req *HandlerRequest[Req, Resp]) error {
	return req.Core.RequestTools().InitRequest(req.W, req.Title, path)
}

func (p capturingPersister[Req, Resp]) Update(_ string, req *HandlerRequest[Req, Resp]) error {
	if p.updateCalled != nil {
		p.updateCalled.Store(true)
	}
	if p.lastUpdated != nil {
		*p.lastUpdated = req
	}
	return nil
}

type testHandlerType[Req testReq, Resp testResp] struct {
	Title           string
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	Persistence     RequestPersister[Req, Resp]
	RecoveryHandler func(any)
	InitErr         error
	HandlerErr      error
	PanicInHandler  bool
}

func (h testHandlerType[Req, Resp]) Parameters() HandlerParameters[Req, Resp] {
	return HandlerParameters[Req, Resp]{
		Title:           h.Title,
		Body:            h.Mode,
		ValidateHeader:  h.VerifyHeader,
		Persistence:     h.Persistence,
		Path:            h.Path,
		HasReceipt:      false,
		RecoveryHandler: h.RecoveryHandler,
		FileResponse:    false,
		LogArrays:       nil,
		LogTags:         nil,
		EnableTracing:   false,
		TracingSpanName: "",
	}
}
func (h testHandlerType[Req, Resp]) Initializer(req HandlerRequest[Req, Resp]) error {
	log.Println("Initializer")
	return h.InitErr
}
func (h testHandlerType[Req, Resp]) Handler(req HandlerRequest[Req, Resp]) (Resp, error) {
	log.Println("Handler")
	if h.PanicInHandler {
		panic("handler panic")
	}
	if h.HandlerErr != nil {
		return Resp{}, h.HandlerErr
	}
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

func testEnv(t *testing.T) *testingtools.TestEnv {
	return testingtools.GetEnvWithDB[testingtools.TestEnv](
		testingtools.SampleRequestModelMock(t, nil).DB,
		testingtools.DefaultAPIList,
	)
}

func runPersistenceHandlerTest(
	t *testing.T,
	handler testHandlerType[testReq, testResp],
	request any,
	expectedStatus int,
) {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	testingtools.TestDB(t, &testingtools.TestCase{
		Name:      "persistence",
		Url:       "/",
		Request:   request,
		Status:    expectedStatus,
		CheckBody: nil,
	}, &testingtools.TestOptions{
		Path:       "/",
		Name:       "persistence outcome",
		Method:     "POST",
		Handler:    BaseHandler(testEnv(t).Interface, handler, false),
		Middleware: gin.Recovery(),
		Silent:     true,
	})
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

	env := testEnv(t)

	handler := BaseHandler(
		env.Interface,
		testHandlerType[testReq, testResp]{
			Title:        "test",
			Path:         "/path/to/api",
			Persistence:  capturingPersister[testReq, testResp]{},
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
	var lastUpdated *HandlerRequest[testReq, testResp]

	runPersistenceHandlerTest(t, testHandlerType[testReq, testResp]{
		Title:        "test",
		Path:         "/path/to/api",
		Persistence:  capturingPersister[testReq, testResp]{updateCalled: &updateCalled, lastUpdated: &lastUpdated},
		Mode:         libRequest.JSON,
		VerifyHeader: true,
	}, testReq{ID: "1"}, 200)

	if !updateCalled.Load() {
		t.Fatal("expected Persistence.Update to be called after successful Insert")
	}
	if lastUpdated == nil {
		t.Fatal("expected captured HandlerRequest from Update")
	}
	if lastUpdated.Outcome.Error != nil {
		t.Fatalf("expected nil Outcome.Error on success, got %v", lastUpdated.Outcome.Error)
	}
	if lastUpdated.Outcome.HTTPStatus != http.StatusOK {
		t.Fatalf("expected HTTPStatus 200, got %d", lastUpdated.Outcome.HTTPStatus)
	}
}

func TestBaseHandlerPersistenceUpdateInitializerFailure(t *testing.T) {
	var updateCalled atomic.Bool
	var lastUpdated *HandlerRequest[testReq, testResp]
	initErr := libError.NewWithDescription(status.BadRequest, "INIT_FAIL", "initializer failed")

	runPersistenceHandlerTest(t, testHandlerType[testReq, testResp]{
		Title:        "test",
		Path:         "/path/to/api",
		Persistence:  capturingPersister[testReq, testResp]{updateCalled: &updateCalled, lastUpdated: &lastUpdated},
		Mode:         libRequest.JSON,
		VerifyHeader: true,
		InitErr:      initErr,
	}, testReq{ID: "1"}, http.StatusBadRequest)

	if !updateCalled.Load() {
		t.Fatal("expected Update after initializer failure")
	}
	if lastUpdated.Outcome.Error == nil {
		t.Fatal("expected Outcome.Error after initializer failure")
	}
	if lastUpdated.Outcome.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected HTTPStatus 400, got %d", lastUpdated.Outcome.HTTPStatus)
	}
}

func TestBaseHandlerPersistenceUpdateHandlerFailure(t *testing.T) {
	var updateCalled atomic.Bool
	var lastUpdated *HandlerRequest[testReq, testResp]
	handlerErr := libError.NewWithDescription(status.BadRequest, "HANDLER_FAIL", "handler failed")

	runPersistenceHandlerTest(t, testHandlerType[testReq, testResp]{
		Title:        "test",
		Path:         "/path/to/api",
		Persistence:  capturingPersister[testReq, testResp]{updateCalled: &updateCalled, lastUpdated: &lastUpdated},
		Mode:         libRequest.JSON,
		VerifyHeader: true,
		HandlerErr:   handlerErr,
	}, testReq{ID: "1"}, http.StatusBadRequest)

	if !updateCalled.Load() {
		t.Fatal("expected Update after handler failure")
	}
	if lastUpdated.Outcome.Error == nil {
		t.Fatal("expected Outcome.Error after handler failure")
	}
	if lastUpdated.Outcome.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected HTTPStatus 400, got %d", lastUpdated.Outcome.HTTPStatus)
	}
}

func TestBaseHandlerPersistenceParseFailureSkipsUpdate(t *testing.T) {
	var updateCalled atomic.Bool

	runPersistenceHandlerTest(t, testHandlerType[testReq, testResp]{
		Title:        "test",
		Path:         "/path/to/api",
		Persistence:  capturingPersister[testReq, testResp]{updateCalled: &updateCalled},
		Mode:         libRequest.JSON,
		VerifyHeader: true,
	}, map[string]any{"ss": "a"}, http.StatusBadRequest)

	if updateCalled.Load() {
		t.Fatal("expected Update not to be called when parse fails before Insert")
	}
}

func TestBaseHandlerPersistenceUpdatePanic(t *testing.T) {
	var updateCalled atomic.Bool
	var lastUpdated *HandlerRequest[testReq, testResp]

	runPersistenceHandlerTest(t, testHandlerType[testReq, testResp]{
		Title:          "test",
		Path:           "/path/to/api",
		Persistence:    capturingPersister[testReq, testResp]{updateCalled: &updateCalled, lastUpdated: &lastUpdated},
		Mode:           libRequest.JSON,
		VerifyHeader:   true,
		PanicInHandler: true,
	}, testReq{ID: "1"}, http.StatusInternalServerError)

	if !updateCalled.Load() {
		t.Fatal("expected Update on panic path")
	}
	if lastUpdated.Outcome.Error == nil {
		t.Fatal("expected Outcome.Error on panic path")
	}
	if lastUpdated.Outcome.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("expected HTTPStatus 500, got %d", lastUpdated.Outcome.HTTPStatus)
	}
}

func TestBaseHandlerNoPersistence(t *testing.T) {
	env := testEnv(t)

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
