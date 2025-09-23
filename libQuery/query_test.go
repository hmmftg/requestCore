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

func getQueryMock(err error, cols []string, values ...driver.Value) libQuery.QueryRunnerInterface {
	db, mockDb, _ := sqlmock.New(
		sqlmock.ValueConverterOption(testingtools.CustomMockConverter{}))

	if err == nil {
		mockDb.ExpectPrepare("query").ExpectQuery().WillReturnRows(
			sqlmock.NewRows(cols).AddRow(values...))
	} else {
		mockDb.ExpectPrepare("query").ExpectQuery().WillReturnError(err)
	}
	return libQuery.QueryRunnerModel{
		DB: db,
	}
}

// SimpleTestOutput represents simplified test data structure
type SimpleTestOutput struct {
	ID      int       `db:"id"`
	Name    string    `db:"name"`
	Active  bool      `db:"active"`
	Count   int64     `db:"count"`
	Price   float64   `db:"price"`
	Created time.Time `db:"created"`
}

func (s SimpleTestOutput) GetID() string { return "" }
func (s SimpleTestOutput) GetValue() any { return "" }

func TestVeryOldQueryRunner(t *testing.T) {
	type TestCase struct {
		Name    string
		Model   libQuery.QueryRunnerInterface
		Command string
		Result  []SimpleTestOutput
		Error   string
	}

	cols := []string{"id", "name", "active", "count", "price", "created"}
	tm := time.Now()
	testCases := []TestCase{
		{
			Name:    "Valid Query",
			Command: "query",
			Model:   getQueryMock(nil, cols, 1, "test", true, int64(10), 12.5, tm),
			Result:  []SimpleTestOutput{{ID: 1, Name: "test", Active: true, Count: 10, Price: 12.5, Created: tm}},
		},
		{
			Name:    "Query On Some Fields",
			Command: "query",
			Model:   getQueryMock(nil, []string{"id", "name"}, 1, "test"),
			Result:  []SimpleTestOutput{{ID: 1, Name: "test"}},
		},
		{
			Name:    "Replace float64 with int64",
			Command: "query",
			Model:   getQueryMock(nil, []string{"id", "price"}, 1, 10),
			Result:  []SimpleTestOutput{{ID: 1, Price: 10}},
		},
	}

	for _, testCase := range testCases {
		result, err := libQuery.GetQuery[SimpleTestOutput](testCase.Command, testCase.Model)
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

func TestOldQueryRunner(t *testing.T) {
	type TestCase struct {
		Name    string
		Model   libQuery.QueryRunnerInterface
		Command libQuery.QueryCommand
		Result  SimpleTestOutput
		Error   string
	}

	cols := []string{"id", "name", "active", "count", "price", "created"}
	tm := time.Now()
	testCases := []TestCase{
		{
			Name:    "Valid Query",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, cols, 1, "test", true, int64(10), 12.5, tm),
			Result:  SimpleTestOutput{ID: 1, Name: "test", Active: true, Count: 10, Price: 12.5, Created: tm},
		},
		{
			Name:    "Query On Some Fields",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, []string{"id", "name"}, 1, "test"),
			Result:  SimpleTestOutput{ID: 1, Name: "test"},
		},
		{
			Name:    "Replace float64 with int64",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, []string{"id", "price"}, 1, 10),
			Result:  SimpleTestOutput{ID: 1, Price: 10},
		},
	}

	for _, testCase := range testCases {
		result, err := libQuery.QueryOld[SimpleTestOutput](testCase.Model, testCase.Command)
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

func TestQueryRunner(t *testing.T) {
	type TestCase struct {
		Name    string
		Model   libQuery.QueryRunnerInterface
		Command libQuery.QueryCommand
		Result  []SimpleTestOutput
		Error   string
	}

	cols := []string{"id", "name", "active", "count", "price", "created"}
	tm := time.Now()
	testCases := []TestCase{
		{
			Name:    "Valid Query",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, cols, 1, "test", true, int64(10), 12.5, tm),
			Result:  []SimpleTestOutput{{ID: 1, Name: "test", Active: true, Count: 10, Price: 12.5, Created: tm}},
		},
		{
			Name:    "Query On Some Fields",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, []string{"id", "name"}, 1, "test"),
			Result:  []SimpleTestOutput{{ID: 1, Name: "test"}},
		},
		{
			Name:    "Replace float64 with int64",
			Command: libQuery.QueryCommand{Command: "query", Type: libQuery.QuerySingle},
			Model:   getQueryMock(nil, []string{"id", "price"}, 1, 10),
			Result:  []SimpleTestOutput{{ID: 1, Price: 10}},
		},
	}

	for _, testCase := range testCases {
		result, err := libQuery.Query[SimpleTestOutput](testCase.Command, testCase.Model)
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

func TestQueryToStruct(t *testing.T) {
	type TestCase struct {
		Name      string
		Interface libQuery.QueryRunnerInterface
		SQL       string
		Result    []SimpleTestOutput
		Error     string
	}

	cols := []string{"id", "name", "active", "count", "price", "created"}
	tm := time.Now()
	testCases := []TestCase{
		{
			Name:      "Valid Query",
			SQL:       "query",
			Interface: getQueryMock(nil, cols, 1, "test", true, int64(10), 12.5, tm),
			Result:    []SimpleTestOutput{{ID: 1, Name: "test", Active: true, Count: 10, Price: 12.5, Created: tm}},
		},
		{
			Name:      "Query On Some Fields",
			SQL:       "query",
			Interface: getQueryMock(nil, []string{"id", "name"}, 1, "test"),
			Result:    []SimpleTestOutput{{ID: 1, Name: "test"}},
		},
		{
			Name:      "Replace float64 with int64",
			SQL:       "query",
			Interface: getQueryMock(nil, []string{"id", "price"}, 1, 10),
			Result:    []SimpleTestOutput{{ID: 1, Price: 10}},
		},
	}

	for _, testCase := range testCases {
		result, err := libQuery.QueryToStruct[SimpleTestOutput](testCase.Interface, testCase.SQL)
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
