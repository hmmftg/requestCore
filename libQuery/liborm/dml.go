// Package liborm provides ORM-based query execution using Gorm.
package liborm

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
)

// ContextKey is the type used for context keys in ORM transaction variables.
type ContextKey string

const (
	// APP is the context key for the application/program identifier in audit trail variables.
	APP = "request.APP"
	// USER is the context key for the user identifier in audit trail variables.
	USER = "request.USER"
	// MODULE is the context key for the module name in audit trail variables.
	MODULE = "request.MODULE"
	// METHOD is the context key for the method name in audit trail variables.
	METHOD = "request.METHOD"
	// SetCommandError is the error description format for SetTrxVariable failures.
	SetCommandError = "error in Dml->SetTrxVariable(%s,%s,%s)"
	// ErrorExecuteDML is the error code for DML execution failures.
	ErrorExecuteDML = "ERROR_EXECUTE_DML"
	// OracleSetVariableCommand is the SQL command for setting audit trail variables in Oracle.
	OracleSetVariableCommand = `--sql
		BEGIN 
			AUDIT_TRAIL.SET_MODIF_ARGS(:1, :2);
		END;`
	// PostgresSetVariableCommand is the SQL command for setting audit trail variables in PostgreSQL.
	PostgresSetVariableCommand = "SELECT set_config($1,$2,true);"
	// MySQLSetVariableCommand is the SQL command format for setting audit trail variables in MySQL.
	MySQLSetVariableCommand = "SET @%s = '%s';"
	// SQLiteSetVariableCommand is the SQL command format for setting audit trail variables in SQLite.
	SQLiteSetVariableCommand = "PRAGMA %s = '%s';"
)

// SetVariable executes the given command to set a transaction variable (key/value) in the database.
func SetVariable(ctx context.Context, tx *gorm.DB, command, key, value string) error {
	if command == "none" {
		return nil
	}
	res := tx.Exec(command, key, value)
	err := res.Error
	if err != nil {
		return libError.Join(err, SetCommandError, command, key, value)
	}
	return nil
}

// OrmModel holds the Gorm DB connection, program/module names, and DML configuration.
type OrmModel struct {
	DB          *gorm.DB
	ProgramName string
	ModuleName  string
	SetVariable string
	Mode        libQuery.DBMode
}

// Init creates and returns an OrmModel configured for the given database mode.
func Init(
	DB *gorm.DB,
	ProgramName string,
	ModuleName string,
	mode libQuery.DBMode) (*OrmModel, error) {
	model := OrmModel{
		DB:          DB,
		ProgramName: ProgramName,
		ModuleName:  ModuleName,
	}
	switch mode {
	case libQuery.Oracle:
		model.SetVariable = OracleSetVariableCommand
	case libQuery.Postgres:
		model.SetVariable = PostgresSetVariableCommand
	case libQuery.MySql:
		model.SetVariable = MySQLSetVariableCommand
	case libQuery.Sqlite:
		model.SetVariable = SQLiteSetVariableCommand
	default:
		return nil, errors.New("db not supported") // or return an error
	}

	return &model, nil
}

// GetDB returns the underlying Gorm DB connection.
func (m OrmModel) GetDB() *gorm.DB {
	return m.DB
}

// GetDbMode returns the database mode (Oracle, Postgres, MySQL, SQLite).
func (m OrmModel) GetDbMode() libQuery.DBMode {
	return m.Mode
}

// SetModifVariables sets audit trail modification variables (app, user, module, method) on the transaction.
func (m OrmModel) SetModifVariables(ctx context.Context, moduleName, methodName string, tx *gorm.DB) error {
	if tx == nil {
		return errors.New("tx is nil")
	}

	variables := map[string]string{
		APP:    fmt.Sprintf("%s.%s", m.ProgramName, m.ModuleName),
		USER:   "",
		MODULE: moduleName,
		METHOD: methodName,
	}

	user := ctx.Value(ContextKey(USER))
	switch userCasted := user.(type) {
	case string:
		variables[USER] = userCasted
	default:
		variables[USER] = ""
	}

	for key, value := range variables {
		err := SetVariable(ctx, tx, m.SetVariable, key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// Dml executes a DML command within a transaction with audit trail variables set, returning rows affected.
func (m OrmModel) Dml(ctx context.Context, moduleName, methodName, command string, args ...any) (int64, error) {
	tx := m.DB.Begin()
	if tx == nil {
		return 0, errors.New("tx is nil")
	}

	err := m.SetModifVariables(ctx, moduleName, methodName, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	preparedArgs := libQuery.PrepareArgs(args)
	result := tx.Exec(command, preparedArgs...)
	if result.Error != nil {
		tx.Rollback()
		return 0, libError.Join(result.Error, "error in Dml->Exec(%s,%s)=>%v", command, args, result)
	}
	err = tx.Commit().Error
	if err != nil {
		return 0, libError.Join(err, "error in Dml->Commit(%s,%s)", command, args)
	}
	return result.RowsAffected, nil
}
