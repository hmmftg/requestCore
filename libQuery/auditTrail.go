package libQuery

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hmmftg/requestCore/libError"
)

// ContextKey is the type for context keys used in audit trail variable propagation.
type ContextKey string

const (
	// APP is the context key for the application name in audit trail variables.
	APP = "request.APP"
	// USER is the context key for the user name in audit trail variables.
	USER = "request.USER"
	// MODULE is the context key for the module name in audit trail variables.
	MODULE = "request.MODULE"
	// METHOD is the context key for the method name in audit trail variables.
	METHOD = "request.METHOD"
	// SetCommandError is the error format for set variable command failures.
	SetCommandError = "error in Dml->SetTrxVariable(%s,%s,%s)"
	// ErrorExecuteDML is the error description for DML execution failures.
	ErrorExecuteDML = "ERROR_EXECUTE_DML"
	// OracleSetVariableCommand is the Oracle SQL command for setting audit trail variables.
	OracleSetVariableCommand = `--sql
		BEGIN 
			CARD_ISSUE.AUDIT_TRAIL.SET_MODIF_ARGS(:1, :2);
		END;`
	// PostgresSetVariableCommand is the PostgreSQL SQL command for setting audit trail variables.
	PostgresSetVariableCommand = "SELECT set_config($1,$2,true);"
)

// SetVariable executes a set variable command within a transaction.
func SetVariable(ctx context.Context, tx *sql.Tx, command, key, value string) error {
	_, err := tx.ExecContext(ctx, command, key, value)
	if err != nil {
		return libError.Join(err, SetCommandError, command, key, value)
	}
	return nil
}

// SetModifVariables sets audit trail modification variables on the transaction.
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
