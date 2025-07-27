package libQuery

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

type CommandInterface interface {
	GetCommand(DBMode) string
	GetArgs() []any
	GetType() int
}

func GetQuery[R any](query string, core QueryRunnerInterface, args ...any) ([]R, error) {
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DB_READ_ERROR, "unable to execute query"),
		)
	}
	if len(rows) == 0 {
		return nil, libError.NewWithDescription(
			http.StatusBadRequest,
			NO_DATA_FOUND,
			"no data found: %s,%v", query, args,
		)
	}
	return rows, nil
}

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
	defer stmt.Close()
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_QUERY_STATEMENT",
				"queryRunner[query](%s,%v)", querySql, args,
			))
	}
	defer rows.Close()

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
func Query[R any](command CommandInterface, core QueryRunnerInterface, args ...any) ([]R, error) {
	if command.GetType() == int(QueryMap) {
		return nil, libError.NewWithDescription(status.BadRequest, DB_READ_ERROR, "unsupported command type")
	}
	query := command.GetCommand(core.GetDbMode())
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DB_READ_ERROR, "unable to execute query"),
		)
	}
	switch command.GetType() {
	case int(QuerySingle):
		if len(rows) == 0 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				NO_DATA_FOUND,
				"no data found: %s,%v", query, args,
			)
		}
		if len(rows) > 1 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				DUPLICATE_FOUND,
				"duplicate data found: %s,%v,%v", query, args, rows,
			)
		}
		return rows, nil
	case int(QueryAll):
		return rows, nil
	}
	return nil, nil
}
