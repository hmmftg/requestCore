package handlers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/handlers"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/testingtools"
)

type SimpleTestData struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SimpleTestResponse struct {
	Data   []SimpleTestData `json:"data"`
	Status string           `json:"status"`
	Count  int              `json:"count"`
}

func (s SimpleTestResponse) SetStatus(a int)                {}
func (s SimpleTestResponse) SetHeaders(a map[string]string) {}

func (env *testCallRemoteEnv) handlerCallAPI(method string, queryStack *[]string) any {
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		req, _, err := libRequest.ParseRequest[libCallApi.CallParamData](
			w, libRequest.JSON, false)
		if err != nil {
			env.Interface.Responder().Error(w, err)
			return
		}
		req.QueryStack = queryStack
		result, errCall := handlers.CallApiInternal[SimpleTestResponse](w, env.Interface, method, req)
		if errCall != nil {
			env.Interface.Responder().Error(w, errCall)
			return
		}
		queryStack = req.QueryStack
		env.Interface.Responder().OK(w, result)
	}
}

func TestCallAPI(t *testing.T) {
	// Create fake API server
	fakeServer := libCallApi.NewFakeAPIServer()
	defer fakeServer.Close()

	queryStack := []string{"test1", "test2", "test3"}
	testCases := []testingtools.TestCase{
		{
			Name:         "Step1",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 1", "Value 1"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step2",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 3", "Value 3"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step3",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 5", "Value 5"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		/*{
			Name:         "Step4",
			Url:          "/",
			DesiredError: "v4/anime@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
				Method: "GET",
			},
			Status:    200,
			CheckBody: []string{"https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/1", "Outlaw World"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},*/
	}

	for id := range testCases {
		env := testingtools.GetEnvWithDB[testCallRemoteEnv](
			testCases[id].Model.DB,
			testingtools.TestAPIList,
		)
		args := strings.Split(testCases[id].DesiredError, "@")
		if len(args) != 4 {
			t.Fatalf("invalid test declaration for url: %s\n", args)
		}

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check call api",
				Method:  "GET",
				Handler: env.handlerCallAPI(args[1], &queryStack),
				Silent:  true,
			})
	}
}

func (env *testCallRemoteEnv) handlerCallAPIJSON(method string, queryStack *[]string, fakeServer *libCallApi.FakeAPIServer) any {
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		req, _, err := libRequest.ParseRequest[libCallApi.CallParamData](
			w, libRequest.JSON, false)
		if err != nil {
			env.Interface.Responder().Error(w, err)
			return
		}
		req.QueryStack = queryStack
		result, errCall := handlers.CallApiJSON(
			w,
			env.Interface,
			method,
			&libCallApi.RemoteCallParamData[any, SimpleTestResponse]{
				Api:        libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				QueryStack: queryStack,
				Method:     "GET",
				Path:       "/",
			},
		)
		if errCall != nil {
			env.Interface.Responder().Error(w, errCall)
			return
		}
		queryStack = req.QueryStack
		env.Interface.Responder().OK(w, result)
	}
}
func TestCallAPIJSON(t *testing.T) {
	// Create fake API server
	fakeServer := libCallApi.NewFakeAPIServer()
	defer fakeServer.Close()

	testCases := []testingtools.TestCase{
		{
			Name:         "Step1",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.RemoteCallParamData[any, any]{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 1", "Value 1"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step2",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.RemoteCallParamData[any, any]{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 3", "Value 3"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step3",
			Url:          "/",
			DesiredError: "api@GET@false@false",
			Request: libCallApi.RemoteCallParamData[any, any]{
				Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
				Method: "GET",
				Path:   "/",
			},
			Status:    200,
			CheckBody: []string{"Test Item 5", "Value 5"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		/*{
			Name:         "Step4",
			Url:          "/",
			DesiredError: "v4/anime@GET@false@false",
			Request: libCallApi.RemoteCallParamData[any]{
				Api:    libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
				Method: "GET",
			},
			Status:    200,
			CheckBody: []string{"https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/1", "Outlaw World"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},*/
	}

	queryStack := []string{"test1", "test2", "test3"}
	for id := range testCases {
		env := testingtools.GetEnvWithDB[testCallRemoteEnv](
			testCases[id].Model.DB,
			testingtools.TestAPIList,
		)
		args := strings.Split(testCases[id].DesiredError, "@")
		if len(args) != 4 {
			t.Fatalf("invalid test declaration for url: %s\n", args)
		}

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check call api",
				Method:  "GET",
				Handler: env.handlerCallAPIJSON("", &queryStack, fakeServer),
				Silent:  true,
			})
	}
}
