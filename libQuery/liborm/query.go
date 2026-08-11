package liborm

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/status"
)

// OrmInterface defines the interface for ORM-based database access with mode and DB handle.
type OrmInterface interface {
	GetDbMode() libQuery.DBMode
	GetDB() *gorm.DB
}

// GetQuery executes a raw SQL query and returns the result rows, returning NoDataFound error if empty.
func GetQuery[R any](query string, core OrmInterface, args ...any) ([]R, error) {
	// Query
	rows, err := QueryToStruct[R](core.GetDB(), query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, libQuery.DBReadError, "unable to execute query"),
		)
	}
	if len(rows) == 0 {
		return nil, libError.NewWithDescription(
			http.StatusBadRequest,
			libQuery.NoDataFound,
			"no data found: %s,%v", query, args,
		)
	}
	return rows, nil
}

// QueryToStruct executes a raw SQL query and scans the result rows into a slice of Target.
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

// Query executes a command interface query and returns result rows, handling single vs all result modes.
func Query[R any](command libQuery.CommandInterface, core OrmInterface, args ...any) ([]R, error) {
	if command.GetType() == int(libQuery.QueryMap) {
		return nil, libError.NewWithDescription(status.BadRequest, libQuery.DBReadError, "unsupported command type")
	}
	query := command.GetCommand(core.GetDbMode())
	// Query
	rows, err := QueryToStruct[R](core.GetDB(), query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, libQuery.DBReadError, "unable to execute query"),
		)
	}
	switch command.GetType() {
	case int(libQuery.QuerySingle):
		if len(rows) == 0 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				libQuery.NoDataFound,
				"no data found: %s,%v", query, args,
			)
		}
		if len(rows) > 1 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				libQuery.DuplicateFound,
				"duplicate data found: %s,%v,%v", query, args, rows,
			)
		}
		return rows, nil
	case int(libQuery.QueryAll):
		return rows, nil
	}
	return nil, nil
}
