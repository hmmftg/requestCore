package liborm

import (
	"context"
	"errors"
	"fmt"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"gorm.io/gorm"
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
			AUDIT_TRAIL.SET_MODIF_ARGS(:1, :2);
		END;`
	PostgresSetVariableCommand = "SELECT set_config($1,$2,true);"
	MySQLSetVariableCommand    = "SET @%s = '%s';"
	SQLiteSetVariableCommand   = "PRAGMA %s = '%s';"
)

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

type OrmModel struct {
	DB          *gorm.DB
	ProgramName string
	ModuleName  string
	SetVariable string
	Mode        libQuery.DBMode
}

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

func (m OrmModel) GetDB() *gorm.DB {
	return m.DB
}
func (m OrmModel) GetDbMode() libQuery.DBMode {
	return m.Mode
}

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
