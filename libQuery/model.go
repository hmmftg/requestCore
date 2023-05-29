package libQuery

import "database/sql"

type QueryRunnerModel struct {
	DB          *sql.DB
	ProgramName string
	ModuleName  string
}

type QueryRunnerInterface interface {
	QueryRunner(querySql string, args ...any) (int, []any, error)
	QueryToStruct(querySql string, target any, args ...any) (int, any, error)
	CallDbFunction(callString string, args ...any) (int, string, error)
	GetModule() (string, string)
	InsertRow(insert string, args ...any) (sql.Result, error)
}
