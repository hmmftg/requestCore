package libQuery

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

// CommandInterface defines the interface for query commands that provide SQL and arguments.
type CommandInterface interface {
	GetCommand(DBMode) string
	GetArgs() []any
	GetType() int
}

// GetQuery executes a SQL query and returns the result as a slice of the target type.
func GetQuery[R any](query string, core QueryRunnerInterface, args ...any) ([]R, error) {
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DBReadError, "unable to execute query"),
		)
	}
	if len(rows) == 0 {
		return nil, libError.NewWithDescription(
			http.StatusBadRequest,
			NoDataFound,
			"no data found: %s,%v", query, args,
		)
	}
	return rows, nil
}

// QueryToStruct executes a SQL query and scans the results into a slice of the target type.
func QueryToStruct[Target any](q QueryRunnerInterface, querySql string, args ...any) ([]Target, error) {
	stmt, err := q.NewStatement(querySql)
	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_INITIALIZE_STATEMENT",
				"queryRunner[prepare](%s,%v)", querySql, args,
			))
	}
	defer func() { _ = stmt.Close() }()
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_QUERY_STATEMENT",
				"queryRunner[query](%s,%v)", querySql, args,
			))
	}
	defer func() { _ = rows.Close() }()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_GET_COLUMN_TYPES",
				"queryRunner[ColumnTypes](%s,%v)", querySql, args,
			))
	}

	count := len(columnTypes)

	baseArgs := make([]any, count)

	for i := range columnTypes {
		baseArgs[i] = new(sql.Null[any])
	}

	finalRows := make([]Target, 0)

	for rows.Next() {
		scanArgs := slices.Clone(baseArgs)
		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, errors.Join(err,
				libError.NewWithDescription(
					status.InternalServerError,
					"UNABLE_TO_GET_SCAN_ROW",
					"queryRunner[Scan](%s,%v)", querySql, scanArgs,
				))
		}

		masterData := map[string]any{}

		for i, v := range columnTypes {
			masterData[v.Name()] = scanArgs[i].(*sql.Null[any]).V
		}

		parsed, err := ParseMap[Target](masterData)
		if parsed == nil {
			return nil, errors.Join(err,
				libError.NewWithDescription(
					status.InternalServerError,
					"UNABLE_TO_GET_SCAN_ROW",
					"queryRunner[parse](%s,%v)", querySql, masterData,
				))
		}
		finalRows = append(finalRows, *parsed)
	}
	//resp, _ := json.Marshal(finalRows)
	return finalRows, nil
}

// Query executes a command interface query and returns results based on the command type.
func Query[R any](command CommandInterface, core QueryRunnerInterface, args ...any) ([]R, error) {
	if command.GetType() == int(QueryMap) {
		return nil, libError.NewWithDescription(status.BadRequest, DBReadError, "unsupported command type")
	}
	query := command.GetCommand(core.GetDbMode())
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DBReadError, "unable to execute query"),
		)
	}
	switch command.GetType() {
	case int(QuerySingle):
		if len(rows) == 0 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				NoDataFound,
				"no data found: %s,%v", query, args,
			)
		}
		if len(rows) > 1 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				DuplicateFound,
				"duplicate data found: %s,%v,%v", query, args, rows,
			)
		}
		return rows, nil
	case int(QueryAll):
		return rows, nil
	}
	return nil, nil
}
