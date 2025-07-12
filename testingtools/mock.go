package testingtools

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libQuery"
)

// GetSelectModel handles selecting a single or all items in mock DB model.
func GetSelectModel(t *testing.T, m Model) libQuery.QueryRunnerModel {
	db, mock, mockErr := sqlmock.New(sqlmock.ValueConverterOption(CustomMockConverter{}))
	if mockErr != nil {
		t.Fatalf("failed to start mock DB: %v", mockErr)
	}

	args := []driver.Value{}

	for _, v := range m.Args {
		args = append(args, v.(driver.Value))
	}

	rows := sqlmock.NewRows(m.Columns)

	for _, firstElm := range m.Values {
		values := []driver.Value{}

		for _, v := range firstElm {
			values = append(values, v.(driver.Value))
		}

		rows.AddRow(values...)
	}

	if len(args) > 0 && len(m.Values) > 0 {
		mock.ExpectPrepare(m.Query).ExpectQuery().WithArgs(args...).WillReturnRows(rows)
	} else if len(m.Values) > 0 {
		mock.ExpectPrepare(m.Query).ExpectQuery().WillReturnRows(rows)
	} else {
		if m.Err != nil {
			mock.ExpectPrepare(m.Query).ExpectQuery().WillReturnError(m.Err)
		} else {
			mock.ExpectPrepare(m.Query).ExpectQuery().WillReturnError(errors.New("no data found"))
		}
	}

	return libQuery.QueryRunnerModel{
		DB: db,
	}
}

// GetDMLModel handles INSERT, UPDATE, and DELETE in mock DB models.
func GetDMLModel(t *testing.T, m Model) libQuery.QueryRunnerModel {
	db, mock, mockErr := sqlmock.New(sqlmock.ValueConverterOption(CustomMockConverter{}))
	if mockErr != nil {
		t.Fatalf("failed to start mock DB: %v", mockErr)
	}

	argsValue := []driver.Value{}

	for _, v := range m.Args {
		argsValue = append(argsValue, v.(driver.Value))
	}

	if len(m.Args) > 0 {
		mock.ExpectExec(m.Query).WithArgs(argsValue...).WillReturnResult(driver.RowsAffected(1))
	} else {
		if m.Err != nil {
			mock.ExpectExec(m.Query).WillReturnError(m.Err)
		} else {
			mock.ExpectExec(m.Query).WillReturnError(errors.New("unable to insert"))
		}
	}

	return libQuery.QueryRunnerModel{
		DB: db,
	}
}

func TestAPIList() map[string]libCallApi.RemoteApi {
	return map[string]libCallApi.RemoteApi{
		"api": {
			Domain: "https://api.github.com",
			Name:   "test api",
		},
	}
}

func SampleRequestModelMock(t *testing.T, mockList func(sqlmock.Sqlmock)) libQuery.QueryRunnerModel {
	db, mockDB, err := sqlmock.New() // sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	var anyS AnyString
	mockDB.ExpectPrepare("query").ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"not duplicate"}))
	mockDB.ExpectBegin()
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("insert").WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectCommit()
	if mockList != nil {
		mockList(mockDB)
	}
	mockDB.ExpectBegin()
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("update").WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectCommit()

	return libQuery.QueryRunnerModel{
		DB: db,
	}
}

func SampleMockDB(t *testing.T, mockList func(sqlmock.Sqlmock)) *sql.DB {
	db, mockDB, err := sqlmock.New() // sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	if mockList != nil {
		mockList(mockDB)
	}

	return db
}

func SampleQueryMock(t *testing.T, mockList func(sqlmock.Sqlmock)) libQuery.QueryRunnerModel {
	return libQuery.QueryRunnerModel{
		DB:   SampleMockDB(t, mockList),
		Mode: libQuery.MockDB,
	}
}
