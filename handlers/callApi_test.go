package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/testingtools"
)

func (env *testCallRemoteEnv) handlerCallAPI(method string, queryStack *[]string) any {
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		req, _, err := libRequest.ParseRequest[libCallApi.CallParamData](
			w, libRequest.JSON, false)
		if err != nil {
			env.Interface.Responder().Error(w, err)
			return
		}
		type Data struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		}
		type Pagination struct {
			LastVisiblePage int  `json:"last_visible_page"`
			HasNextPage     bool `json:"has_next_page"`
		}
		type AnimeEpisodes struct {
			Data       []Data     `json:"data"`
			Pagination Pagination `json:"pagination"`
		}
		req.QueryStack = queryStack
		result, errCall := callApi[AnimeEpisodes](w, env.Interface, method, req)
		if errCall != nil {
			env.Interface.Responder().Error(w, errCall)
			return
		}
		queryStack = req.QueryStack
		env.Interface.Responder().OK(w, result)
	}
}

func TestCallAPI(t *testing.T) {
	queryStack := []string{"1/episodes", "200/episodes", "300/episodes", "400/episodes"}
	testCases := []testingtools.TestCase{
		{
			Name:         "Step1",
			Url:          "/",
			DesiredError: "v4/anime@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
				Method: "GET",
			},
			Status:    200,
			CheckBody: []string{"https://myanimelist.net/anime/1/Cowboy_Bebop/episode/1", "Asteroid Blues"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step2",
			Url:          "/",
			DesiredError: "v4/anime@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
				Method: "GET",
			},
			Status:    200,
			CheckBody: []string{"Meeting at Full Speed − Is the Angel Male or Female?"},
			Model:     testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
		{
			Name:         "Step3",
			Url:          "/",
			DesiredError: "v4/anime@GET@false@false",
			Request: libCallApi.CallParamData{
				Api:    libCallApi.RemoteApi{Domain: "https://api.jikan.moe/v4/anime"},
				Method: "GET",
			},
			Status:    200,
			CheckBody: []string{"Transmigration"},
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
