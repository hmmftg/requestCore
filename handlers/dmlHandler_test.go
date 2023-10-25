package handlers

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/testingtools"
)

type testDMLReq struct {
	ID string `json:"id" validate:"required"`
}

const (
	DML1 = "dml1"
	Pre1 = "pre1"
	Ins1 = "ins1"
)

func (m testDMLReq) PreControlCommands() map[string][]libQuery.DmlCommand {
	return map[string][]libQuery.DmlCommand{
		DML1: {{
			Name:    Pre1,
			Command: Pre1,
			Type:    libQuery.QueryCheckNotExists,
		}}}
}
func (m testDMLReq) DmlCommands() map[string][]libQuery.DmlCommand {
	return map[string][]libQuery.DmlCommand{
		DML1: {{
			Name:    Ins1,
			Command: Ins1,
			Type:    libQuery.Insert,
		}}}
}
func (m testDMLReq) FinalizeCommands() map[string][]libQuery.DmlCommand {
	return nil
}

type testDMLEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

func (env testDMLEnv) GetInterface() requestCore.RequestCoreInterface {
	return env.Interface
}
func (env testDMLEnv) GetParams() libParams.ParamInterface {
	return env.Params
}
func (env *testDMLEnv) SetInterface(core requestCore.RequestCoreInterface) {
	env.Interface = core
}
func (env *testDMLEnv) SetParams(params libParams.ParamInterface) {
	env.Params = params
}

func (env *testDMLEnv) handler() any {
	return DmlHandler[testDMLReq]("dml_handler", DML1, "/",
		env.Interface,
		libRequest.JSON,
		true,
	)
}

func beforeDMLMocks(mockDB sqlmock.Sqlmock) {
	var anyS testingtools.AnyString
	mockDB.ExpectBegin()
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
	mockDB.ExpectExec("").WithArgs(anyS, anyS).WillReturnResult(driver.RowsAffected(1))
}

func TestDMLHandler(t *testing.T) {
	testCases := []testingtools.TestCase{
		{
			Name:      "Valid",
			Url:       "/",
			Request:   testDMLReq{ID: "1"},
			Status:    200,
			CheckBody: []string{Ins1, "rowsAffected", `:1`},
			Model: testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Pre1).ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"result", "key", "value"}))
				beforeDMLMocks(mockDB)
				mockDB.ExpectExec(Ins1).WillReturnResult(driver.RowsAffected(1))
				mockDB.ExpectCommit()
			}),
		},
		{
			Name:      "Precontrol failed",
			Url:       "/",
			Request:   testDMLReq{ID: "1"},
			Status:    500,
			CheckBody: []string{"errors", "error pre1"},
			Model: testingtools.SampleRequestModelMock(t, func(mockDB sqlmock.Sqlmock) {
				mockDB.ExpectPrepare(Pre1).ExpectQuery().WillReturnError(errors.New("error pre1"))
			}),
		},
	}

	for id := range testCases {
		env := testingtools.GetEnvWithDB[testDMLEnv](
			testCases[id].Model.DB,
			testingtools.DefaultAPIList)

		testingtools.TestDB(
			t,
			&testCases[id],
			&testingtools.TestOptions{
				Path:    "/",
				Name:    "check dml handler",
				Method:  "POST",
				Handler: env.handler(),
				Silent:  true,
			})
	}
}
