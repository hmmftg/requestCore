package libQuery

import (
	"context"
	"database/sql"
	"database/sql/driver"
)

// QueryRunnerModel holds the database connection and metadata for query execution.
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
	// Oracle is the DBMode for Oracle databases.
	Oracle DBMode = iota
	// Postgres is the DBMode for PostgreSQL databases.
	Postgres
	// Sqlite is the DBMode for SQLite databases.
	Sqlite
	// MockDB is the DBMode for mock databases used in testing.
	MockDB
	// MySql is the DBMode for MySQL databases.
	MySql
)

// Init creates a QueryRunnerModel with the database-specific set variable command.
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

// QueryRunnerInterface defines the methods for executing database queries and DML operations.
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

// DmlModel defines the interface for models that provide pre-control, DML, and finalize commands.
type DmlModel interface {
	PreControlCommands() map[string][]DmlCommand
	DmlCommands() map[string][]DmlCommand
	FinalizeCommands() map[string][]DmlCommand
}

// Updatable defines the interface for models that support update operations.
type Updatable interface {
	SetParams(args map[string]string) any
	GetUniqueId() []any
	GetCountCommand() string
	GetUpdateCommand() (string, []any)
	Finalize(QueryRunnerInterface) (string, error)
}

//go:generate enumer -type=DmlCommandType -json -output dmlEnum.go
type DmlCommandType int

// DmlCommand represents a single DML command with optional database-specific variants.
type DmlCommand struct {
	Name        string
	Command     string
	CommandMap  map[DBMode]string
	Args        []any
	Type        DmlCommandType
	CustomError error
}

// GetCommand returns the SQL command for the given database mode.
func (d DmlCommand) GetCommand(mode DBMode) string {
	query := d.Command
	if len(d.CommandMap) > 0 && len(d.CommandMap[mode]) > 0 {
		query = d.CommandMap[mode]
	}
	return query
}

// GetArgs returns the arguments for the DML command.
func (d DmlCommand) GetArgs() []any {
	return d.Args
}

// GetType returns the integer type of the DML command.
func (d DmlCommand) GetType() int {
	return int(d.Type)
}

//go:generate enumer -type=QueryCommandType -json -output queryEnum.go
type QueryCommandType int

// QueryCommand represents a single query command with optional database-specific variants.
type QueryCommand struct {
	Name       string
	Command    string
	CommandMap map[DBMode]string
	Type       QueryCommandType
	Args       []any
}

// GetCommand returns the SQL command for the given database mode.
func (q QueryCommand) GetCommand(mode DBMode) string {
	query := q.Command
	if len(q.CommandMap) > 0 && len(q.CommandMap[mode]) > 0 {
		query = q.CommandMap[mode]
	}
	return query
}

// GetArgs returns the arguments for the query command.
func (q QueryCommand) GetArgs() []any {
	return q.Args
}

// GetType returns the integer type of the query command.
func (q QueryCommand) GetType() int {
	return int(q.Type)
}

// GetDriverArgs resolves form-tagged arguments from the request into driver values.
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

// QueryRequest defines the interface for requests that provide query arguments.
type QueryRequest interface {
	QueryArgs() map[string][]any
}

// QueryResult defines the interface for query results that expose an ID and value.
type QueryResult interface {
	GetID() string
	GetValue() any
}

// QueryWithDeps defines the interface for queries with fillable dependencies.
type QueryWithDeps interface {
	GetFillable(core QueryRunnerInterface) (map[string]any, error)
}

// DmlResult holds the result of a DML operation including affected rows and output parameters.
type DmlResult struct {
	Rows         map[string]string `json:"rows" form:"rows"`
	LastInsertId int64             `json:"lastId" form:"lastId"`
	RowsAffected int64             `json:"rowsAffected" form:"rowsAffected"`
}

// QueryData holds a generic query result row with key-value and array fields.
type QueryData struct {
	DataRaw    string   `json:"result,omitempty" db:"result"`
	Key        string   `json:"key,omitempty" db:"key"`
	Value      string   `json:"value,omitempty" db:"value"`
	ValueArray []string `json:"valueArray,omitempty" db:"values"`
	MapList    string   `json:"mapList,omitempty" db:"map_list"`
}

// RecordDataGet defines the interface for reading record data from query results.
type RecordDataGet interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	GetSubCategory() string
	GetValue() any
}

// RecordDataDml defines the interface for DML operations on record data.
type RecordDataDml interface {
	SetId(string)
	CheckDuplicate(core QueryRunnerInterface) (int, string, error)
	Filler(headers map[string][]string, core QueryRunnerInterface, args ...any) (string, error)
	Post(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
	CheckExistence(core QueryRunnerInterface) (int, string, error)
	PreControl(core QueryRunnerInterface) (int, string, error)
	Put(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
}
