package libQuery

import (
	"context"
	"database/sql"
	"database/sql/driver"
)

type QueryRunnerModel struct {
	DB          *sql.DB
	ProgramName string
	ModuleName  string
	SetVariable string
	Mode        DBMode
}

//go:generate enumer -type=DBMode -json -output dbModeEnum.go
type DBMode int

const (
	Oracle DBMode = iota
	Postgres
	Sqlite
	MockDB
	MySql
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
	NewStatement(command string) (*sql.Stmt, error)
	CallDbFunction(callString string, args ...any) (int, string, error)
	GetModule() (string, string)
	InsertRow(insert string, args ...any) (sql.Result, error)
	Dml(ctx context.Context, moduleName, methodName, command string, args ...any) (sql.Result, error)
	SetVariableCommand() string
	//Used in mock db for test
	Close()
	GetDbMode() DBMode
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
	CommandMap  map[DBMode]string
	Args        []any
	Type        DmlCommandType
	CustomError error
}

func (d DmlCommand) GetCommand(mode DBMode) string {
	query := d.Command
	if len(d.CommandMap) > 0 && len(d.CommandMap[mode]) > 0 {
		query = d.CommandMap[mode]
	}
	return query
}

func (d DmlCommand) GetArgs() []any {
	return d.Args
}

func (d DmlCommand) GetType() int {
	return int(d.Type)
}

//go:generate enumer -type=QueryCommandType -json -output queryEnum.go
type QueryCommandType int

type QueryCommand struct {
	Name       string
	Command    string
	CommandMap map[DBMode]string
	Type       QueryCommandType
	Args       []any
}

func (q QueryCommand) GetCommand(mode DBMode) string {
	query := q.Command
	if len(q.CommandMap) > 0 && len(q.CommandMap[mode]) > 0 {
		query = q.CommandMap[mode]
	}
	return query
}

func (q QueryCommand) GetArgs() []any {
	return q.Args
}

func (q QueryCommand) GetType() int {
	return int(q.Type)
}

func (q QueryCommand) GetDriverArgs(req any) []driver.Value {
	args := []driver.Value{}
	for id := range q.Args {
		_, val, err := GetFormTagValue(q.Args[id].(string), req)
		if err != nil {
			return nil
		}
		args = append(args, val)
	}
	return args
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
