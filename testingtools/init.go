package testingtools

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type TestingParams struct {
}

const Default = "default"

// columns are prefixed with "o" since we used sqlstruct to generate them
func InitTesting(t *testing.T,
	errDesc map[string]string,
	remoteApis map[string]libCallApi.RemoteApi,
	query string,
	columns []string,
	csv string,
	module string,
) (requestCore.RequestCoreModel, libParams.ParamInterface) {
	wsParams := libParams.ApplicationParams[TestingParams]{
		Constants: map[string]libParams.Constants{
			Default: {
				ErrorDesc: errDesc,
			},
		},
		RemoteApis: remoteApis,
	}

	// open database stub
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Errorf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	// expect transaction begin
	mock.ExpectBegin()
	// expect query to fetch order and user, match it with regexp
	mock.ExpectQuery(query).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows(columns).FromCSVString(csv))
	mock.ExpectCommit()

	queryRunner := libQuery.QueryRunnerModel{
		DB:          db,
		ProgramName: filepath.Base(os.Args[0]),
		ModuleName:  module,
	}
	requestHandler := libRequest.RequestModel{
		QueryInterface: queryRunner,
		InsertInDb:     "insert",
		UpdateInDb:     "update",
		QueryInDb:      "select",
	}

	return requestCore.RequestCoreModel{
		QueryInterface:   queryRunner,
		RequestInterface: requestHandler,
		RespHandler: response.WebHanlder{
			ErrorDesc:   wsParams.Constants[Default].ErrorDesc,
			MessageDesc: wsParams.Constants[Default].MessageDesc,
		},
		ParamMap: wsParams,
	}, wsParams
}

func DefaultAccessRoles() map[string]string {
	return map[string]string{
		"/cardType_GET": "get_card",
	}
}

func DefaultErrorDesc() map[string]string {
	return map[string]string{
		"AUTH_BAD_USER":     "sss",
		"AUTH_BAD_PASS":     "ttt",
		"AUTH_BAD_METHOD":   "fff",
		"DUPLICATE_REQUEST": "dup",
	}
}

func DefaultAPIList() map[string]libCallApi.RemoteApi {
	return map[string]libCallApi.RemoteApi{
		"simulation": {
			Domain: "http://local.simulation.dev/simulation/api",
			// Domain: "http://localhost:9055/simulation/api",
			Name: "simulation",
		},
		"gorest": {
			Domain: "https://gorest.co.in/public/v2",
			// Domain: "http://localhost:9055/simulation/api",
			Name: "gorest",
		},
	}
}

func DefaultDB(t *testing.T) *sql.DB {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Errorf("An error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	return db
}

func InitTestingNoDB(t *testing.T,
	errDesc map[string]string,
	remoteApis map[string]libCallApi.RemoteApi,
) (requestCore.RequestCoreModel, libParams.ParamInterface) {
	return InitTestingWithDB(errDesc, remoteApis, DefaultDB(t))
}

func InitTestingWithDB(
	errDesc map[string]string,
	remoteApis map[string]libCallApi.RemoteApi,
	db *sql.DB,
) (requestCore.RequestCoreModel, libParams.ApplicationParams[TestingParams]) {
	wsParams := libParams.ApplicationParams[TestingParams]{
		Constants: map[string]libParams.Constants{
			Default: {
				ErrorDesc: errDesc,
			},
		},
		RemoteApis: remoteApis,
	}

	queryRunner := libQuery.QueryRunnerModel{
		DB:          db,
		ProgramName: filepath.Base(os.Args[0]),
		ModuleName:  "",
		Mode:        libQuery.MockDB,
	}
	requestHandler := libRequest.RequestModel{
		QueryInterface: queryRunner,
		InsertInDb:     "insert",
		UpdateInDb:     "update",
		QueryInDb:      "query",
	}

	return requestCore.RequestCoreModel{
		QueryInterface:   queryRunner,
		OrmInterface:     nil,
		RequestInterface: requestHandler,
		RespHandler: response.WebHanlder{
			ErrorDesc:   wsParams.Constants[Default].ErrorDesc,
			MessageDesc: wsParams.Constants[Default].MessageDesc,
		},
		ParamMap: wsParams,
	}, wsParams
}
