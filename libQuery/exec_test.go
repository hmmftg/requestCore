package libQuery

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/response"
	"gotest.tools/v3/assert"
)

type AnyString string

// Match satisfies sqlmock.Argument interface
func (a AnyString) Match(v driver.Value) bool {
	return true
}

func getMock(sqlType DmlCommandType, err error) QueryRunnerInterface {
	db, mockDb, _ := sqlmock.New() //sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))

	var anyS AnyString
	switch sqlType {
	case QueryCheckExists:
		if err == nil {
			mockDb.ExpectPrepare("queryE").ExpectQuery().WillReturnRows(
				sqlmock.NewRows([]string{"key", "value"}).AddRow("1", "2"))
		} else {
			mockDb.ExpectPrepare("queryE").ExpectQuery().WillReturnError(err)
		}
	case QueryCheckNotExists:
		if err == nil {
			mockDb.ExpectPrepare("queryN").ExpectQuery().WillReturnRows(
				sqlmock.NewRows([]string{"key", "value"}))
		} else {
			mockDb.ExpectPrepare("queryE").ExpectQuery().WillReturnError(err)
		}
	case Insert:
		mockDb.ExpectBegin()
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		if err == nil {
			mockDb.ExpectExec("insert").WillReturnResult(driver.RowsAffected(1))
		} else {
			mockDb.ExpectExec("queryE").WillReturnError(err)
		}
		mockDb.ExpectCommit()
	case Update:
		mockDb.ExpectBegin()
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		if err == nil {
			mockDb.ExpectExec("update").WillReturnResult(driver.RowsAffected(1))
		} else {
			mockDb.ExpectExec("queryE").WillReturnError(err)
		}
		mockDb.ExpectCommit()
	case Delete:
		mockDb.ExpectBegin()
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		mockDb.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
		if err == nil {
			mockDb.ExpectExec("delete").WillReturnResult(driver.RowsAffected(1))
		} else {
			mockDb.ExpectExec("queryE").WillReturnError(err)
		}
		mockDb.ExpectCommit()
	}

	return QueryRunnerModel{
		DB: db,
	}
}

func TestExecuteWithContext(t *testing.T) {
	type TestCase struct {
		Name    string
		Context context.Context
		Command DmlCommand
		Model   QueryRunnerInterface
		Result  any
		Error   response.ErrorState
	}
	testCases := []TestCase{{
		Name: "Valid Query Exists",
		Command: DmlCommand{
			Name:    "q1",
			Command: "queryE",
			Type:    QueryCheckExists,
		},
		Model:  getMock(QueryCheckExists, nil),
		Result: []QueryData{{Key: "1", Value: "2"}},
	}, {
		Name: "Invalid Query Exists",
		Command: DmlCommand{
			Name:        "q2",
			Command:     "queryE",
			Type:        QueryCheckExists,
			CustomError: &response.ErrorData{Description: "eeeerrr", Status: 1, Message: "mmm"},
		},
		Model: getMock(QueryCheckExists, errors.New("error happened")),
		Error: &response.ErrorData{Description: "eeeerrr", Status: 1, Message: "mmm"},
	}, {
		Name: "Valid Query Not Exists",
		Command: DmlCommand{
			Name:    "q1",
			Command: "queryN",
			Type:    QueryCheckNotExists,
		},
		Model: getMock(QueryCheckNotExists, nil),
	}, {
		Name: "Invalid Query Not Exists",
		Command: DmlCommand{
			Name:        "q2",
			Command:     "queryN",
			Type:        QueryCheckNotExists,
			CustomError: &response.ErrorData{Description: "eeeerrr", Status: 1, Message: "mmm"},
		},
		Model: getMock(QueryCheckNotExists, errors.New("error happened")),
		Error: &response.ErrorData{Description: "eeeerrr", Status: 1, Message: "mmm"},
	}}
	for _, testCase := range testCases {
		result, err := testCase.Command.ExecuteWithContext(testCase.Context, "", "", testCase.Model)
		assert.DeepEqual(t, result, testCase.Result)
		if err == nil && testCase.Error != nil {
			t.Fatal("error wanted", testCase.Error, "got", err)
		}
		if err != nil && testCase.Error == nil {
			t.Fatal("error wanted", testCase.Error, "got", err)
		}
		if err != nil {
			assert.Equal(t, err.GetDescription(), testCase.Error.GetDescription())
			assert.Equal(t, err.GetStatus(), testCase.Error.GetStatus())
			assert.Equal(t, err.GetMessage(), testCase.Error.GetMessage())
		}
	}
}
