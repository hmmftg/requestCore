package liborm

import (
	"errors"
	"net/http"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/status"
	"gorm.io/gorm"
)

type OrmInterface interface {
	GetDbMode() libQuery.DBMode
	GetDB() *gorm.DB
}

func GetQuery[R any](query string, core OrmInterface, args ...any) ([]R, error) {
	// Query
	rows, err := QueryToStruct[R](core.GetDB(), query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, libQuery.DB_READ_ERROR, "unable to execute query"),
		)
	}
	if len(rows) == 0 {
		return nil, libError.NewWithDescription(
			http.StatusBadRequest,
			libQuery.NO_DATA_FOUND,
			"no data found: %s,%v", query, args,
		)
	}
	return rows, nil
}

func QueryToStruct[Target any](db *gorm.DB, querySql string, args ...any) ([]Target, error) {
	var rows []Target
	result := db.Raw(querySql, args...).Find(&rows)
	if result.Error != nil {
		return nil, errors.Join(result.Error,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_QUERY_STATEMENT",
				"queryRunner[query](%s,%v)", querySql, args,
			))
	}
	return rows, nil
}

func Query[R any](command libQuery.CommandInterface, core OrmInterface, args ...any) ([]R, error) {
	if command.GetType() == int(libQuery.QueryMap) {
		return nil, libError.NewWithDescription(status.BadRequest, libQuery.DB_READ_ERROR, "unsupported command type")
	}
	query := command.GetCommand(core.GetDbMode())
	// Query
	rows, err := QueryToStruct[R](core.GetDB(), query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, libQuery.DB_READ_ERROR, "unable to execute query"),
		)
	}
	switch command.GetType() {
	case int(libQuery.QuerySingle):
		if len(rows) == 0 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				libQuery.NO_DATA_FOUND,
				"no data found: %s,%v", query, args,
			)
		}
		if len(rows) > 1 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				libQuery.DUPLICATE_FOUND,
				"duplicate data found: %s,%v,%v", query, args, rows,
			)
		}
		return rows, nil
	case int(libQuery.QueryAll):
		return rows, nil
	}
	return nil, nil
}
