package libQuery

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hmmftg/requestCore/libError"
)

type ContextKey string

const (
	APP                      = "request.APP"
	USER                     = "request.USER"
	MODULE                   = "request.MODULE"
	METHOD                   = "request.METHOD"
	SetCommandError          = "error in Dml->SetTrxVariable(%s,%s,%s)"
	ErrorExecuteDML          = "ERROR_EXECUTE_DML"
	OracleSetVariableCommand = `--sql
		BEGIN 
			CARD_ISSUE.AUDIT_TRAIL.SET_MODIF_ARGS(:1, :2);
		END;`
	PostgresSetVariableCommand = "SELECT set_config($1,$2,true);"
)

func SetVariable(ctx context.Context, tx *sql.Tx, command, key, value string) error {
	_, err := tx.ExecContext(ctx, command, key, value)
	if err != nil {
		return libError.Join(err, SetCommandError, command, key, value)
	}
	return nil
}

func (m QueryRunnerModel) SetModifVariables(ctx context.Context, moduleName, methodName string, tx *sql.Tx) error {
	command := m.SetVariable
	err := SetVariable(ctx, tx, command, APP, fmt.Sprintf("%s.%s", m.ProgramName, m.ModuleName))
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
	err = SetVariable(ctx, tx, command, MODULE, moduleName)
	if err != nil {
		return err
	}
	err = SetVariable(ctx, tx, command, METHOD, methodName)
	if err != nil {
		return err
	}
	return nil
}
