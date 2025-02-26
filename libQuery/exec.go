package libQuery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
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
		slog.Error("error in ping", slog.Any("error", errPing))
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

func (command DmlCommand) Execute(core QueryRunnerInterface, moduleName, methodName string) (any, error) {
	return command.ExecuteWithContext(nil, context.Background(), moduleName, methodName, core)
}

func GetDmlResult(resultDb sql.Result, rows map[string]string) DmlResult {
	resp := DmlResult{
		Rows: rows,
	}
	resp.LastInsertId, _ = resultDb.LastInsertId()
	resp.RowsAffected, _ = resultDb.RowsAffected()
	return resp
}

func GetLocalArgs(parser webFramework.RequestParser, args []any) []any {
	result := make([]any, len(args))
	for id, arg := range args {
		namedArg, ok := arg.(sql.NamedArg)
		if ok {
			stringArg, ok := namedArg.Value.(string)
			if ok && strings.HasPrefix(stringArg, "w.local:") {
				parts := strings.Split(stringArg, ":")
				namedArg.Value = parser.GetLocal(parts[1])
			}
			result[id] = namedArg
		} else {
			result[id] = arg
		}
	}
	return result
}

func GetOutArgs(parser webFramework.RequestParser, args ...any) map[string]string {
	rows := map[string]string{}
	for id, arg := range args {
		switch dbParameter := arg.(type) {
		case sql.NamedArg:
			switch namedParameter := dbParameter.Value.(type) {
			case sql.Out:
				if namedParameter.Dest != nil {
					switch outValue := namedParameter.Dest.(type) {
					case string:
						rows[dbParameter.Name] = outValue
					case *string:
						rows[dbParameter.Name] = *outValue
					case int64:
						rows[dbParameter.Name] = fmt.Sprintf("%d", outValue)
					case *int64:
						rows[dbParameter.Name] = fmt.Sprintf("%d", *outValue)
					default:
						slog.Error("wrong db-out parameter type", slog.Any("type", fmt.Sprintf("%T", namedParameter.Dest)))
					}
					parser.SetLocal(dbParameter.Name, rows[dbParameter.Name])
				}
			}
		case sql.Out:
			if dbParameter.Dest != nil {
				name := fmt.Sprintf("not named arg %d", id)
				switch outValue := dbParameter.Dest.(type) {
				case string:
					rows[name] = outValue
				case *string:
					rows[name] = *outValue
				case int64:
					rows[name] = fmt.Sprintf("%d", outValue)
				case *int64:
					rows[name] = fmt.Sprintf("%d", *outValue)
				default:
					slog.Error("wrong db-out parameter type", slog.Any("type", fmt.Sprintf("%T", dbParameter.Dest)))
				}
				parser.SetLocal(name, rows[name])
			}
		}
	}
	return rows
}

func (command DmlCommand) ExecuteWithContext(parser webFramework.RequestParser, w context.Context, moduleName, methodName string, core QueryRunnerInterface) (any, error) {
	switch command.Type {
	case QueryCheckExists:
		result, err := GetQuery[QueryData](command.Command, core, GetLocalArgs(parser, command.Args)...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			if ok, err := libError.Unwrap(err); ok && err.Action().Description == NO_DATA_FOUND {
				return nil, errors.Join(err, fmt.Errorf("checkExists: %s=> %s", command.Name, NO_DATA_FOUND))
			}
			return nil, errors.Join(err, fmt.Errorf("checkExists: %s=> failed", command.Name))
		}
		return result, nil
	case QueryCheckNotExists:
		_, err := GetQuery[QueryData](command.Command, core, GetLocalArgs(parser, command.Args)...)
		if err != nil {
			if ok, err := libError.Unwrap(err); ok && err.Action().Description == NO_DATA_FOUND {
				return nil, nil
			}
			return nil, errors.Join(err, fmt.Errorf("CheckNotExists: %s=> failed", command.Name))
		}
		return nil, response.ToError(DUPLICATE_FOUND, DUPLICATE_FOUND_DESC, fmt.Errorf("CheckNotExists: %s=> %s", command.Name, DUPLICATE_FOUND))
	case Insert:
		resp, err := core.Dml(w, moduleName, methodName, command.Command, GetLocalArgs(parser, command.Args)...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		outValues := GetOutArgs(parser, command.Args...)
		return GetDmlResult(resp, outValues), nil
	case Update:
		resp, err := core.Dml(w, moduleName, methodName, command.Command, GetLocalArgs(parser, command.Args)...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		outValues := GetOutArgs(parser, command.Args...)
		return GetDmlResult(resp, outValues), nil
	case Delete:
		resp, err := core.Dml(w, moduleName, methodName, command.Command, GetLocalArgs(parser, command.Args)...)
		if err != nil {
			if command.CustomError != nil {
				return nil, command.CustomError
			}
			return nil, response.ToError(ErrorExecuteDML, nil, libError.Join(err, "%s: %s", command.Type, command.Name))
		}
		outValues := GetOutArgs(parser, command.Args...)
		return GetDmlResult(resp, outValues), nil
	}
	return nil, nil
}
