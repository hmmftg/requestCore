package libQuery

import (
	"context"
	"database/sql"

	"github.com/hmmftg/requestCore/response"
)

type QueryRunnerModel struct {
	DB          *sql.DB
	ProgramName string
	ModuleName  string
	SetVariable string
}

//go:generate enumer -type=DBMode -json -output dbModeEnum.go
type DBMode int

const (
	Oracle DBMode = iota
	Postgres
	MockDB
)

func Init(
	DB *sql.DB,
	ProgramName string,
	ModuleName string,
	mode DBMode) QueryRunnerModel {
	model := QueryRunnerModel{
		DB:          DB,
		ProgramName: ProgramName,
		ModuleName:  ModuleName,
	}
	switch mode {
	case Oracle:
		model.SetVariable = OracleSetVariableCommand
	case Postgres:
		model.SetVariable = PostgresSetVariableCommand
	default:
		model.SetVariable = "none"
	}

	return model
}

type QueryRunnerInterface interface {
	QueryRunner(querySql string, args ...any) (int, []any, error)
	QueryToStruct(querySql string, target any, args ...any) (int, any, error)
	CallDbFunction(callString string, args ...any) (int, string, error)
	GetModule() (string, string)
	InsertRow(insert string, args ...any) (sql.Result, error)
	Dml(ctx context.Context, moduleName, methodName, command string, args ...any) (sql.Result, error)
	SetVariableCommand() string
	//Used in mock db for test
	Close()
}

type DmlModel interface {
	PreControlCommands() map[string][]DmlCommand
	DmlCommands() map[string][]DmlCommand
	FinalizeCommands() map[string][]DmlCommand
}

type Updatable interface {
	SetParams(args map[string]string) any
	GetUniqueId() []any
	GetCountCommand() string
	GetUpdateCommand() (string, []any)
	Finalize(QueryRunnerInterface) (string, error)
}

//go:generate enumer -type=DmlCommandType -json -output dmlEnum.go
type DmlCommandType int

type DmlCommand struct {
	Name        string
	Command     string
	Args        []any
	Type        DmlCommandType
	CustomError *response.ErrorState
}

//go:generate enumer -type=QueryCommandType -json -output queryEnum.go
type QueryCommandType int

type QueryCommand struct {
	Name    string
	Command string
	Type    QueryCommandType
}

type QueryRequest interface {
	QueryArgs() map[string][]any
}

type QueryResult interface {
	GetID() string
	GetValue() any
}

type QueryWithDeps interface {
	GetFillable(core QueryRunnerInterface) (map[string]any, error)
}

type DmlResult struct {
	Rows         map[string]string `json:"rows" form:"rows"`
	LastInsertId int64             `json:"lastId" form:"lastId"`
	RowsAffected int64             `json:"rowsAffected" form:"rowsAffected"`
}

type QueryData struct {
	DataRaw    string   `json:"result,omitempty" db:"result"`
	Key        string   `json:"key,omitempty" db:"key"`
	Value      string   `json:"value,omitempty" db:"value"`
	ValueArray []string `json:"valueArray,omitempty" db:"values"`
	MapList    string   `json:"mapList,omitempty" db:"map_list"`
}

type RecordDataGet interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	GetSubCategory() string
	GetValue() any
}

type RecordDataDml interface {
	SetId(string)
	CheckDuplicate(core QueryRunnerInterface) (int, string, error)
	Filler(headers map[string][]string, core QueryRunnerInterface, args ...any) (string, error)
	Post(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
	CheckExistence(core QueryRunnerInterface) (int, string, error)
	PreControl(core QueryRunnerInterface) (int, string, error)
	Put(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
}
