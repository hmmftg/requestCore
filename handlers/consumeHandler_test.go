package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/testingtools"
	"gotest.tools/v3/assert"
)

func mockGetHeader(name string) string {
	return "Header-" + name
}
func mockGetLocal(name string) string {
	return "Local-" + name
}
func mockGetEmpty(name string) string {
	return ""
}

func Test_extractValue(t *testing.T) {
	type testCase struct {
		Name   string
		Value  string
		Source func(string) string
		Dest   map[string]string
	}
	testCases := []testCase{
		{Name: "ValidGetHeader", Value: "user", Source: mockGetHeader, Dest: map[string]string{"user": "Header-user"}},
		{Name: "ValidGetLocal", Value: "user", Source: mockGetLocal, Dest: map[string]string{"user": "Local-user"}},
		{Name: "InvalidGetHeader", Value: "user", Source: mockGetEmpty, Dest: map[string]string{"user": ""}},
		{Name: "InvalidGetLocal", Value: "user", Source: mockGetEmpty, Dest: map[string]string{"user": ""}},
	}
	for _, tc := range testCases {
		result := make(map[string]string, 0)
		extractValue(tc.Value, tc.Source, result)
		assert.DeepEqual(t, result, tc.Dest)
	}
}

func Test_extractHeader(t *testing.T) {
	type testCase struct {
		Name      string
		Locals    []string
		Headers   []string
		HeaderEnv string
		LocalEnv  string
		Dest      map[string]string
	}
	testCases := []testCase{
		{
			Name:      "ValidGetHeader",
			HeaderEnv: "User-Id#a@Person-Id#b",
			Headers:   []string{"User-Id"},
			LocalEnv:  "userId#a@personId#b",
			Locals:    []string{"userId"},
			Dest:      map[string]string{"User-Id": "a", "userId": "a"}},
	}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Setenv(libContext.HeaderEnvKey, tc.HeaderEnv)
			t.Setenv(libContext.LocalEnvKey, tc.LocalEnv)
			w := libContext.InitContext(t)
			result := extractHeaders(w, tc.Headers, tc.Locals)
			assert.DeepEqual(t, result, tc.Dest)
		})
	}
}

type testCallRemoteEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

func (env testCallRemoteEnv) GetInterface() requestCore.RequestCoreInterface {
	return env.Interface
}
func (env testCallRemoteEnv) GetParams() libParams.ParamInterface {
	return env.Params
}
func (env *testCallRemoteEnv) SetInterface(core requestCore.RequestCoreInterface) {
	env.Interface = core
}
func (env *testCallRemoteEnv) SetParams(params libParams.ParamInterface) {
	env.Params = params
}

type testCallReq struct {
}

type githubRespOrg struct {
	Login              string `json:"login"`
	Id                 int    `json:"id"`
	Node_id            string `json:"node_id"`
	Url                string `json:"url"`
	Repos_url          string `json:"repos_url"`
	Events_url         string `json:"events_url"`
	Hooks_url          string `json:"hooks_url"`
	Issues_url         string `json:"issues_url"`
	Members_url        string `json:"members_url"`
	Public_members_url string `json:"public_members_url"`
	Avatar_url         string `json:"avatar_url"`
	Description        string `json:"description"`
}

func ParseGithubRespJson(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp []githubRespOrg
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	return http.StatusOK, nil, resp, nil
}

func (env *testCallRemoteEnv) handler(url, method string, isJSON, hasQuery bool) any {
	return CallRemoteWithRespParser[testCallReq, []githubRespOrg](
		"call_remote_handler", url, "api", method, hasQuery, isJSON, true,
		true,
		libCallApi.TransmitRequestWithAuth,
		env.Interface,
		ParseGithubRespJson,
	)
}

func TestCallRemoteHandler(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:         "Valid",
			Url:          "/",
			DesiredError: "users/hadley/orgs@GET@false@false",
			Status:       200,
			CheckBody:    []string{"result", "login", "ggobi"},
			Model:        testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {}),
		},
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
		isJ, _ := strconv.ParseBool(args[2])
		isQ, _ := strconv.ParseBool(args[3])

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check call remote handler",
				Method:  "GET",
				Handler: env.handler(args[0], args[1], isJ, isQ),
				Silent:  true,
			})
	}
}
