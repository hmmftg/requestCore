package libsql_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libsql"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/testingtools"
	"gotest.tools/v3/assert"
)

func TestQuery(t *testing.T) {
	type result struct {
		ID     string     `db:"ID"`
		Name   string     `db:"NAME"`
		Number int        `db:"NUM"`
		Time   *time.Time `db:"TIME"`
	}
	type testCase struct {
		DB      *sql.DB
		Query   string
		Args    []any
		Results []result
		Error   response.ErrorState
	}
	tm := time.Date(1, 2, 3, 4, 5, 6, 0, time.Local)
	testCases := []testCase{{
		Query: "selet id, name, num, tm from dual;",
		DB: testingtools.SampleMockDB(t, func(mockDB sqlmock.Sqlmock) {
			mockDB.ExpectPrepare("selet id, name, num, tm from dual;").
				ExpectQuery().WillReturnRows(
				sqlmock.NewRows([]string{"ID", "NAME", "NUM", "TIME"}).
					AddRow("1", "2", 12, &tm))
		}),
		Results: []result{{
			ID: "1", Name: "2", Number: 12, Time: &tm,
		}},
	}}
	for id := range testCases {
		results, err := libsql.Query[result](testCases[id].DB, testCases[id].Query, testCases[id].Args...)
		if err != nil {
			if testCases[id].Error == nil {
				t.Fatal("desired no error got", err)
			}
			if testCases[id].Error.Error() != err.Error() {
				t.Fatal("desired error", testCases[id].Error, "got", err)
			}
		}
		if testCases[id].Error != nil {
			t.Fatal("desired error", err, "got nil")
		}
		assert.DeepEqual(t, testCases[id].Results, results)
	}
}
