package libQuery

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

func (m QueryRunnerModel) SetVariableCommand() string {
	return m.SetVariable
}

const (
	ERROR_CALLING_DB_FUNCTION = "ERROR_CALLING_DB_FUNCTION"
)

func (m QueryRunnerModel) CallDbFunction(callString string, args ...any) (int, string, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		log.Println("error in ping", errPing)
	}
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

func (command DmlCommand) Execute(core QueryRunnerInterface, moduleName, methodName string) (any, response.ErrorState) {
	return command.ExecuteWithContext(context.Background(), moduleName, methodName, core)
}

func GetDmlResult(resultDb sql.Result, rows map[string]string) DmlResult {
	resp := DmlResult{
		Rows: rows,
	}
	resp.LastInsertId, _ = resultDb.LastInsertId()
	resp.RowsAffected, _ = resultDb.RowsAffected()
	return resp
}

func (command DmlCommand) ExecuteWithContext(ctx context.Context, moduleName, methodName string, core QueryRunnerInterface) (any, response.ErrorState) {
	switch command.Type {
	case QueryCheckExists:
		_, desc, data, resp, err := CallSql[QueryData](command.Command, core, command.Args...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
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
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(desc, data, libError.Join(err, "CheckNotExists: %s", command.Name))
		}
		return nil, response.ToError(DUPLICATE_FOUND, DUPLICATE_FOUND_DESC, fmt.Errorf("CheckNotExists: %s=> %s", command.Name, DUPLICATE_FOUND))
	case Insert:
		resp, err := core.Dml(ctx, moduleName, methodName, command.Command, command.Args...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return GetDmlResult(resp, nil), nil
	case Update:
		resp, err := core.Dml(ctx, moduleName, methodName, command.Command, command.Args...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return GetDmlResult(resp, nil), nil
	case Delete:
		resp, err := core.Dml(ctx, moduleName, methodName, command.Command, command.Args...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		return GetDmlResult(resp, nil), nil
	}
	return nil, nil
}
