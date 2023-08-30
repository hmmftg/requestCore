package libQuery

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

func (m QueryRunnerModel) InsertRow(insert string, args ...any) (sql.Result, error) {
	result, err := m.DB.Exec(
		insert,
		args...)
	if err != nil {
		return nil, libError.Join(err, "InsertRow(%s,%s)=>%v", insert, args, result)
	}
	return result, nil
}

type ContextKey string

const (
	APP                        = "request.APP"
	USER                       = "request.USER"
	MODULE                     = "request.MODULE"
	METHOD                     = "request.METHOD"
	SetCommandError            = "error in Dml->SetTrxVariable(%s,%s,%s)"
	ERROR_EXECUTE_DML          = "ERROR_EXECUTE_DML"
	OracleSetVariableCommand   = "BEGIN DBMS_SESSION.SET_CONTEXT('request', :1, :2); END"
	PostgresSetVariableCommand = "SELECT set_config($1,$2,true);"
)

func SetVariable(ctx context.Context, tx *sql.Tx, command, key, value string) error {
	_, err := tx.Exec(command, key, value)
	if err != nil {
		return libError.Join(err, SetCommandError, command, key, value)
	}
	return nil
}

func (m QueryRunnerModel) SetModifVariables(ctx context.Context, methodName string, tx *sql.Tx) error {
	command := m.SetVariable
	err := SetVariable(ctx, tx, command, APP, m.ProgramName)
	if err != nil {
		return err
	}
	user := ctx.Value(ContextKey(USER))
	var userString string
	switch userCasted := user.(type) {
	case string:
		userString = userCasted
	default:
		userString = ""
	}
	err = SetVariable(ctx, tx, command, USER, userString)
	if err != nil {
		return err
	}
	err = SetVariable(ctx, tx, command, MODULE, m.ModuleName)
	if err != nil {
		return err
	}
	err = SetVariable(ctx, tx, command, METHOD, methodName)
	if err != nil {
		return err
	}
	return nil
}

func (m QueryRunnerModel) Dml(ctx context.Context, methodName, command string, args ...any) (sql.Result, error) {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, libError.Join(err, "error in Dml->BeginTrx()")
	}

	err = m.SetModifVariables(ctx, methodName, tx)
	if err != nil {
		return nil, err
	}

	//result, err := tx.ExecContext(ctx, command, args...)
	result, err := tx.Exec(command, args...)
	if err != nil {
		return nil, libError.Join(err, "error in Dml->Exec(%s,%s)=>%v", command, args, result)
	}
	err = tx.Commit()
	if err != nil {
		return nil, libError.Join(err, "error in Dml->Commit(%s,%s)", command, args)
	}
	return result, nil
}

func (m QueryRunnerModel) SetVariableCommand() string {
	return m.SetVariable
}

const (
	ERROR_CALLING_DB_FUNCTION = "ERROR_CALLING_DB_FUNCTION"
)

func (m QueryRunnerModel) CallDbFunction(callString string, args ...any) (int, string, error) {
	_, err := m.DB.Exec(callString, args...)
	if err != nil {
		return -3, ERROR_CALLING_DB_FUNCTION, libError.Join(err, "CallDbFunction[Exec](%s,%v)", callString, args)
	}

	return 0, "OK", nil
}

const (
	QueryCheckNotExists DmlCommandType = iota
	QueryCheckExists
	Insert
	Update
	Delete
)

func (command DmlCommand) Execute(core QueryRunnerInterface, methodName string) (any, *response.ErrorState) {
	return command.ExecuteWithContext(context.Background(), methodName, core)
}

func (command DmlCommand) ExecuteWithContext(ctx context.Context, methodName string, core QueryRunnerInterface) (any, *response.ErrorState) {
	switch command.Type {
	case QueryCheckExists:
		_, desc, data, resp, err := CallSql[QueryData](command.Command, core, command.Args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "CheckExists: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.ToError(NO_DATA_FOUND, NO_DATA_FOUND_DESC, fmt.Errorf("CheckExists: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		return resp, nil
	case QueryCheckNotExists:
		_, desc, data, resp, err := CallSql[QueryData](command.Command, core, command.Args...)
		if len(desc) > 0 && desc == NO_DATA_FOUND && resp == nil {
			return nil, nil // OK
		}
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "CheckNotExists: %s", command.Name))
		}
		return nil, response.ToError(DUPLICATE_FOUND, DUPLICATE_FOUND_DESC, fmt.Errorf("CheckNotExists: %s=> %s", command.Name, DUPLICATE_FOUND))
	case Insert:
		resp, err := core.Dml(ctx, methodName, command.Command, command.Args...)
		if err != nil {
			return nil, response.ToError(ERROR_EXECUTE_DML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return resp, nil
	case Update:
		resp, err := core.Dml(ctx, methodName, command.Command, command.Args...)
		if err != nil {
			return nil, response.ToError(ERROR_EXECUTE_DML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return resp, nil
	case Delete:
		resp, err := core.Dml(ctx, methodName, command.Command, command.Args...)
		if err != nil {
			return nil, response.ToError(ERROR_EXECUTE_DML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return resp, nil
	}
	return nil, nil
}
