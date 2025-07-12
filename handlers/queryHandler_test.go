package handlers

import (
	"fmt"
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
	Query4   = "query4"
	Query5   = "query5"
	Query6   = "query6"
	Command1 = "command1"
	Command2 = "command2"
	Command3 = "command3"
	Command4 = "command4"
	Command5 = "command5"
	Command6 = "command6"
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
			Args:    []any{"id", "p2"},
		},
		Query4: {
			Name:    "q4",
			Command: Command4,
			Type:    libQuery.QuerySingle,
			CommandMap: map[libQuery.DBMode]string{
				libQuery.MockDB: Command4,
				libQuery.Oracle: Command1,
			},
		},
		Query5: {
			Name:    "q5",
			Command: Command5,
			Type:    libQuery.QueryAll,
			CommandMap: map[libQuery.DBMode]string{
				libQuery.MockDB: Command5,
				libQuery.Oracle: Command2,
			},
		},
		Query6: {
			Name:    "q6",
			Command: Command6,
			Type:    libQuery.QuerySingle,
			Args:    []any{"id", "p2"},
			CommandMap: map[libQuery.DBMode]string{
				libQuery.MockDB: Command6,
				libQuery.Oracle: Command3,
			},
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

type testQueryResp struct {
	ID      string `json:"id"`
	API     string `json:"api"`
	Name    string `json:"name" `
	Address string `json:"address"`
}

type testTransformer[Row testQueryReq, Resp []testQueryResp] struct {
}

func (s testTransformer[Row, Resp]) Translate(rows []testQueryReq, req HandlerRequest[Row, Resp]) (QueryResp[Resp], error) {
	result := make([]testQueryResp, len(rows))
	for id := range rows {
		result[id] = testQueryResp{
			ID:      rows[id].ID,
			API:     req.Title,
			Name:    rows[id].P2,
			Address: rows[id].Data,
		}
	}
	return QueryResp[Resp]{Resp: result, TotalRows: len(result)}, nil
}

func (s testTransformer[Row, Resp]) TranslateWithPaginate(rows []testQueryReq, req HandlerRequest[Row, Resp], pd libRequest.PaginationData) (QueryResp[Resp], error) {
	result := make([]testQueryResp, len(rows))
	for id := range rows {
		result[id] = testQueryResp{
			ID:      rows[id].ID,
			API:     req.Title,
			Name:    rows[id].P2,
			Address: rows[id].Data,
		}
	}
	result = Filterate(pd, result, func(f Filter) func(testQueryResp) bool {
		return func(r testQueryResp) bool {
			return r.Address == "filtered"
		}
	})
	totalRows := len(result)
	paginatedResult := Paginate(pd, result, func(string) func(i, j int) bool {
		return func(i, j int) bool {
			return result[i].ID < result[j].ID
		}
	})
	return QueryResp[Resp]{Resp: paginatedResult, TotalRows: totalRows}, nil
}

func (env *testQueryEnv) handlerWithTransform() any {
	return QueryHandlerWithTransform(
		"query_handler_with_transform",
		Query3,
		"/",
		QueryMap,
		env.Interface,
		libRequest.QueryWithPagination,
		true,
		false,
		nil,
		CommandReplacer[libRequest.PaginationData]{Token: "#", Builder: func(a libRequest.PaginationData) string {
			return fmt.Sprintf("Start=%d and End=%d", a.Start, a.End)
		}},
		testTransformer[testQueryReq, []testQueryResp]{},
		nil,
	)
}

func TestQueryHandlerWithTransform(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:        "Valid",
			Url:         "/?id=1&p2=3",
			Header:      testingtools.Header{testingtools.KeyValuePair{Key: "Request-Id", Value: "11111"}},
			Status:      200,
			CheckHeader: map[string]string{"X-Total-Count": "2"},
			CheckBody: []string{`"result":[`,
				`{"id":"1","api":"query_handler_with_transform","name":"2","address":"3"}`,
				`{"id":"4","api":"query_handler_with_transform","name":"5","address":"6"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command3).
					ExpectQuery().
					WithArgs(QueryMap[Query3].GetDriverArgs(testQueryReq{ID: "1", P2: "3"})...).
					WillReturnRows(
						sqlmock.NewRows([]string{"ID", "P2", "DATA"}).
							AddRow("1", "2", "3").
							AddRow("4", "5", "6"))
			}),
		},
		{
			Name:        "ValidWithPagination",
			Url:         "/?id=1&p2=3&_start=0&_end=12",
			Status:      200,
			CheckHeader: map[string]string{"X-Total-Count": "21"},
			CheckBody: []string{`"result":[`,
				`{"id":"1","api":"query_handler_with_transform","name":"2","address":"3"}`,
				`{"id":"c7","api":"query_handler_with_transform","name":"8","address":"9"}`,
				`{"id":"4","api":"query_handler_with_transform","name":"5","address":"6"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command3).
					ExpectQuery().
					WithArgs(QueryMap[Query3].GetDriverArgs(testQueryReq{ID: "1", P2: "3"})...).
					WillReturnRows(
						sqlmock.NewRows([]string{"ID", "P2", "DATA"}).
							AddRow("1", "2", "3").
							AddRow("4", "5", "6").
							AddRow("7", "8", "9").
							AddRow("a1", "2", "3").
							AddRow("a4", "5", "6").
							AddRow("a7", "8", "9").
							AddRow("b1", "2", "3").
							AddRow("b4", "5", "6").
							AddRow("b7", "8", "9").
							AddRow("c1", "2", "3").
							AddRow("c4", "5", "6").
							AddRow("c7", "8", "9").
							AddRow("d1", "2", "3").
							AddRow("d4", "5", "6").
							AddRow("d7", "8", "9").
							AddRow("e1", "2", "3").
							AddRow("e4", "5", "6").
							AddRow("e7", "8", "9").
							AddRow("f1", "2", "3").
							AddRow("f4", "5", "6").
							AddRow("f7", "8", "9"))
			}),
		},
		{
			Name:        "ValidWithPaginationAndFilteration",
			Url:         "/?id=1&p2=3&_start=0&_end=12&_filters=address%20ne%20filtered%20",
			Status:      200,
			CheckHeader: map[string]string{"X-Total-Count": "19"},
			CheckBody: []string{`"result":[`,
				`{"id":"1","api":"query_handler_with_transform","name":"2","address":"3"}`,
				`{"id":"c7","api":"query_handler_with_transform","name":"8","address":"9"}`,
				`{"id":"4","api":"query_handler_with_transform","name":"5","address":"6"}`},
			CheckNotInBody: []string{
				`{"id":"a7","api":"query_handler_with_transform","name":"8","address":"filtered"}`,
				`{"id":"c4","api":"query_handler_with_transform","name":"5","address":"filtered"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command3).
					ExpectQuery().
					WithArgs(QueryMap[Query3].GetDriverArgs(testQueryReq{ID: "1", P2: "3"})...).
					WillReturnRows(
						sqlmock.NewRows([]string{"ID", "P2", "DATA"}).
							AddRow("1", "2", "3").
							AddRow("4", "5", "6").
							AddRow("7", "8", "9").
							AddRow("a1", "2", "3").
							AddRow("a4", "5", "6").
							AddRow("a7", "8", "filtered").
							AddRow("b1", "2", "3").
							AddRow("b4", "5", "6").
							AddRow("b7", "8", "9").
							AddRow("c1", "2", "3").
							AddRow("c4", "5", "filtered").
							AddRow("c7", "8", "9").
							AddRow("d1", "2", "3").
							AddRow("d4", "5", "6").
							AddRow("d7", "8", "9").
							AddRow("e1", "2", "3").
							AddRow("e4", "5", "6").
							AddRow("e7", "8", "9").
							AddRow("f1", "2", "3").
							AddRow("f4", "5", "6").
							AddRow("f7", "8", "9"))
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
				Handler: env.handlerWithTransform(),
				Silent:  true,
			})
	}
}

func (env *testQueryEnv) handlerMultiDb() any {
	return QueryHandler[testQueryReq](
		"query_handler_multi_db",
		Query4,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryHandlerMultiDb(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"4","data":"5"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command4).
					ExpectQuery().WillReturnRows(
					sqlmock.NewRows([]string{"ID", "DATA"}).
						AddRow("4", "5"))
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
				Handler: env.handlerMultiDb(),
				Silent:  true,
			})
	}
}

func (env *testQueryEnv) handlerAllMultiDb() any {
	return QueryHandler[testQueryReq](
		"queryAll_handler_all_multi_db",
		Query5,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryAllHandlerMultiDb(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"5","data":"6"}`, `{"id":"7","data":"8"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command5).
					ExpectQuery().WillReturnRows(
					sqlmock.NewRows([]string{"ID", "DATA"}).
						AddRow("5", "6").
						AddRow("7", "8"))
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
				Name:    "check query all multi db handler",
				Method:  "GET",
				Handler: env.handlerAllMultiDb(),
				Silent:  true,
			})
	}
}

func (env *testQueryEnv) handlerWithArgsMultiDb() any {
	return QueryHandler[testQueryReq](
		"query_handler_with_args_multi_db",
		Query6,
		"/",
		QueryMap,
		env.Interface,
		libRequest.Query,
		true,
		false,
		nil,
	)
}

func TestQueryHandlerWithArgsMultiDb(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/?id=1&p2=3",
			Status:    200,
			CheckBody: []string{`"result":[`, `{"id":"6","data":"7"}`},
			Model: testingtools.SampleQueryMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Command6).
					ExpectQuery().
					WithArgs(QueryMap[Query6].GetDriverArgs(testQueryReq{ID: "1", P2: "3"})...).
					WillReturnRows(
						sqlmock.NewRows([]string{"ID", "DATA"}).
							AddRow("6", "7"))
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
				Name:    "check query with args multi db handler",
				Method:  "GET",
				Handler: env.handlerWithArgsMultiDb(),
				Silent:  true,
			})
	}
}
