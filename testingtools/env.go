package testingtools

import (
	"database/sql"
	"testing"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libParams"

	"github.com/hmmftg/requestCore"
)

type TestingEnv interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
}

func PrepareEnvWithDB(env TestingEnv, db *sql.DB) {
	model, wsParams := InitTestingWithDB(
		DefaultErrorDesc(),
		DefaultAPIList(),
		db,
	)
	wsParams.AccessRoles = DefaultAccessRoles()
	env.SetInterface(model)
	env.SetParams(&wsParams)
}

func GetEnv[Env any, PT interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
	*Env
}](t *testing.T, defaultAPIList func() map[string]libCallApi.RemoteApi) PT {
	return GetEnvWithDB[Env, PT](DefaultDB(t), defaultAPIList)
}

func GetEnvWithDB[Env any, PT interface {
	GetInterface() requestCore.RequestCoreInterface
	GetParams() libParams.ParamInterface
	SetInterface(requestCore.RequestCoreInterface)
	SetParams(libParams.ParamInterface)
	*Env
}](db *sql.DB, defaultAPIList func() map[string]libCallApi.RemoteApi) PT {
	model, wsParams := InitTestingWithDB(
		DefaultErrorDesc(),
		defaultAPIList(),
		db,
	)
	wsParams.AccessRoles = DefaultAccessRoles()
	env := PT(new(Env))
	env.SetInterface(model)
	env.SetParams(&wsParams)
	return env
}
