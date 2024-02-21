package handlers

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/testingtools"
)

type testQueryEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

func (env testQueryEnv) GetInterface() requestCore.RequestCoreInterface {
	return env.Interface
}
func (env testQueryEnv) GetParams() libParams.ParamInterface {
	return env.Params
}
func (env *testQueryEnv) SetInterface(core requestCore.RequestCoreInterface) {
	env.Interface = core
}
func (env *testQueryEnv) SetParams(params libParams.ParamInterface) {
	env.Params = params
}

type testQueryReq struct {
	ID   string `form:"id" json:"id" validate:"required" db:"ID"`
	P2   string `form:"p2" json:"-" db:"P2"`
	Data string `json:"data" db:"DATA"`
}

const (
	Query1   = "query1"
	Query2   = "query2"
	Query3   = "query3"
	Command1 = "command1"
	Command2 = "command2"
	Command3 = "command3"
)

var (
	QueryMap = map[string]libQuery.QueryCommand{
		Query1: {
			Name:    "q1",
			Command: Command1,
			Type:    libQuery.QuerySingle,
		},
		Query2: {
			Name:    "q2",
			Command: Command2,
			Type:    libQuery.QueryAll,
		},
		Query3: {
			Name:    "q3",
			Command: Command3,
			Type:    libQuery.QuerySingle,
			Args:    []string{"id", "p2"},
		},
	}
)

func (env *testQueryEnv) handler() any {
	return QueryHandler[testQueryReq](
		"query_handler",
		Query1,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryHandler(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"1","data":"2"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command1).
					ExpectQuery().WillReturnRows(
					sqlmock.NewRows([]string{"ID", "DATA"}).
						AddRow("1", "2"))
			}),
		},
	}

	for id := range testCases {
		env := testingtools.GetEnvWithDB[testQueryEnv](
			testCases[id].Model.DB,
			testingtools.DefaultAPIList)

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check query handler",
				Method:  "GET",
				Handler: env.handler(),
				Silent:  true,
			})
	}
}

func (env *testQueryEnv) handlerAll() any {
	return QueryHandler[testQueryReq](
		"queryAll_handler_all",
		Query2,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryAllHandler(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"1","data":"2"}`, `{"id":"2","data":"3"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command2).
					ExpectQuery().WillReturnRows(
					sqlmock.NewRows([]string{"ID", "DATA"}).
						AddRow("1", "2").
						AddRow("2", "3"))
			}),
		},
	}

	for id := range testCases {
		env := testingtools.GetEnvWithDB[testQueryEnv](
			testCases[id].Model.DB,
			testingtools.DefaultAPIList)

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check query all handler",
				Method:  "GET",
				Handler: env.handlerAll(),
				Silent:  true,
			})
	}
}

func (env *testQueryEnv) handlerWithArgs() any {
	return QueryHandler[testQueryReq](
		"query_handler_with_args",
		Query3,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryHandlerWithArgs(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1&p2=3",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"1","data":"2"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command3).
					ExpectQuery().
					WithArgs(QueryMap[Query3].GetDriverArgs(testQueryReq{ID: "1", P2: "3"})...).
					WillReturnRows(
						sqlmock.NewRows([]string{"ID", "DATA"}).
							AddRow("1", "2"))
			}),
		},
	}

	for id := range testCases {
		env := testingtools.GetEnvWithDB[testQueryEnv](
			testCases[id].Model.DB,
			testingtools.DefaultAPIList)

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check query with args handler",
				Method:  "GET",
				Handler: env.handlerWithArgs(),
				Silent:  true,
			})
	}
}
