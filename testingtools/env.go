package testingtools

import (
	"database/sql"
	"testing"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libParams"

	"github.com/hmmftg/requestCore"
)

// TestingEnv defines the interface for a test environment that holds request-core and parameter interfaces.
type TestingEnv interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
}

// PrepareEnvWithDB initializes a TestingEnv with default error descriptions, API list, and the given database.
func PrepareEnvWithDB(env TestingEnv, db *sql.DB) {
	model, wsParams := InitTestingWithDB(
		DefaultErrorDesc(),
		DefaultAPIList(),
		db,
	)
	env.SetInterface(model)
	env.SetParams(&wsParams)
}

// GetEnv creates and initializes a typed test environment with a default mock database.
func GetEnv[Env any, PT interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
	*Env
}](t *testing.T, defaultAPIList func() map[string]libCallApi.RemoteAPI) PT {
	return GetEnvWithDB[Env, PT](DefaultDB(t), defaultAPIList)
}

// GetEnvWithDB creates and initializes a typed test environment with the given database and API list.
func GetEnvWithDB[Env any, PT interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
	*Env
}](db *sql.DB, defaultAPIList func() map[string]libCallApi.RemoteAPI) PT {
	model, wsParams := InitTestingWithDB(
		DefaultErrorDesc(),
		defaultAPIList(),
		db,
	)
	env := PT(new(Env))
	env.SetInterface(model)
	env.SetParams(&wsParams)
	return env
}
