package libQuery

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/hmmftg/requestCore/libError"
)

func (m QueryRunnerModel) InsertRow(insert string, args ...any) (sql.Result, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		slog.Error("error in ping", slog.Any("error", errPing))
	}
	result, err := m.DB.Exec(
		insert,
		args...)
	if err != nil {
		return nil, libError.Join(err, "InsertRow(%s,%s)=>%v", insert, args, result)
	}
	return result, nil
}

func (m QueryRunnerModel) Dml(ctx context.Context, moduleName, methodName, command string, args ...any) (sql.Result, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		slog.Error("error in ping", slog.Any("error", errPing))
	}
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, libError.Join(err, "error in Dml->BeginTrx()")
	}

	err = m.SetModifVariables(ctx, moduleName, methodName, tx)
	if err != nil {
		return nil, err
	}

	preparedArgs := PrepareArgs(args)
	result, err := tx.ExecContext(ctx, command, preparedArgs...)
	if err != nil {
		errRB := tx.Rollback()
		if errRB != nil {
			err = errors.Join(errRB, err)
		}
		return nil, libError.Join(err, "error in Dml->Exec(%s,%s)=>%v", command, args, result)
	}
	err = tx.Commit()
	if err != nil {
		return nil, libError.Join(err, "error in Dml->Commit(%s,%s)", command, args)
	}
	return result, nil
}
