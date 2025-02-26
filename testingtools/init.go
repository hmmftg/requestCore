package testingtools

import (
	"database/sql"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/image/font/opentype"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libDictionary"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type Params struct {
	Roles       map[string]string
	Params      map[string]string
	ErrorDesc   map[string]string
	MessageDesc map[string]string
	AccessRoles map[string]string
	RemoteApis  map[string]libCallApi.RemoteApi
}

func (p Params) GetFonts() map[string]opentype.Font {
	return nil
}
func (p Params) GetImages() map[string]image.Image {
	return nil
}
func (p Params) GetRoles() map[string]string {
	return p.Roles
}
func (p Params) GetParams() map[string]string {
	return p.Params
}

// columns are prefixed with "o" since we used sqlstruct to generate them
func InitTesting(t *testing.T,
	errDesc map[string]string,
	remoteApis map[string]libCallApi.RemoteApi,
	query string,
	columns []string,
	csv string,
	module string,
) (requestCore.RequestCoreModel, libParams.ParamInterface) {
	wsParams := Params{
		ErrorDesc:  errDesc,
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
			ErrorDesc:   wsParams.ErrorDesc,
			MessageDesc: wsParams.MessageDesc,
		},
		RemoteApiInterface: libCallApi.RemoteApiModel{
			RemoteApiList: wsParams.RemoteApis,
		},
		Dict: libDictionary.DictionaryModel{
			MessageDesc: wsParams.MessageDesc,
		},
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
) (requestCore.RequestCoreModel, Params) {
	wsParams := Params{
		ErrorDesc:  errDesc,
		RemoteApis: remoteApis,
	}

	queryRunner := libQuery.QueryRunnerModel{
		DB:          db,
		ProgramName: filepath.Base(os.Args[0]),
		ModuleName:  "",
	}
	requestHandler := libRequest.RequestModel{
		QueryInterface: queryRunner,
		InsertInDb:     "insert",
		UpdateInDb:     "update",
		QueryInDb:      "query",
	}

	return requestCore.RequestCoreModel{
		QueryInterface:   queryRunner,
		RequestInterface: requestHandler,
		RespHandler: response.WebHanlder{
			ErrorDesc:   wsParams.ErrorDesc,
			MessageDesc: wsParams.MessageDesc,
		},
		RemoteApiInterface: libCallApi.RemoteApiModel{
			RemoteApiList: wsParams.RemoteApis,
		},
		Dict: libDictionary.DictionaryModel{
			MessageDesc: wsParams.MessageDesc,
		},
	}, wsParams
}
