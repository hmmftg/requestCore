package libQuery_test

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/testingtools"
	"gotest.tools/v3/assert"
)

func getQueryMock(err error, cols []string, vlaues ...driver.Value) libQuery.QueryRunnerInterface {
	db, mockDb, _ := sqlmock.New(
		sqlmock.ValueConverterOption(testingtools.CustomMockConverter{}))

	if err == nil {
		mockDb.ExpectPrepare("query").ExpectQuery().WillReturnRows(
			sqlmock.NewRows(cols).AddRow(vlaues...))
	} else {
		mockDb.ExpectPrepare("query").ExpectQuery().WillReturnError(err)
	}
	return libQuery.QueryRunnerModel{
		DB: db,
	}
}

type sampleOutput struct {
	String  string    `db:"string"`
	Bool    bool      `db:"bool"`
	Int64   int64     `db:"int64"`
	Float64 float64   `db:"float64"`
	Time    time.Time `db:"time"`
}

func (s sampleOutput) GetID() string { return "" }
func (s sampleOutput) GetValue() any { return "" }

func TestQueryRunner(t *testing.T) {
	type TestCase struct {
		Name    string
		Model   libQuery.QueryRunnerInterface
		Command libQuery.QueryCommand
		Result  sampleOutput
		Error   string
	}
	cols := []string{"string", "bool", "time", "int64", "float64"}
	tm := time.Now()
	testCases := []TestCase{{
		Name:    "Valid Query",
		Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
		Model:   getQueryMock(nil, cols, "a", true, tm, 10, 12.1),
		Result:  sampleOutput{String: "a", Bool: true, Int64: 10, Float64: 12.1, Time: tm},
	}, {
		Name:    "Query On Some Fields",
		Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
		Model:   getQueryMock(nil, []string{"string", "bool"}, "a", true),
		Result:  sampleOutput{String: "a", Bool: true},
	}, {
		Name:    "Replace float64 with int64",
		Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
		Model:   getQueryMock(nil, []string{"string", "float64"}, "a", 10),
		Result:  sampleOutput{String: "a", Float64: 10},
	},
	}
	for _, testCase := range testCases {
		result, err := libQuery.Query[sampleOutput](testCase.Model, testCase.Command)
		if err != nil && testCase.Error == "" {
			t.Fatal("unwanted error", err)
		}
		if err == nil && testCase.Error != "" {
			t.Fatal("error wanted", testCase.Error, "got", err)
		}
		assert.DeepEqual(t, result, testCase.Result)
		if err != nil {
			assert.Equal(t, err.Error(), testCase.Error)
		}
	}
}
