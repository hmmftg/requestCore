// Package libsql provides SQL query execution utilities with result scanning.
package libsql

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/hmmftg/requestCore/response"

	"github.com/blockloop/scan/v2"
)

// Query executes a SQL query and scans the result rows into a slice of Result.
func Query[Result any](db *sql.DB, querySQL string, args ...any) ([]Result, error) {
	errPing := db.Ping()
	if errPing != nil {
		slog.Error("error in ping", slog.Any("error", errPing))
	}
	stmt, err := db.Prepare(querySQL)
	if err != nil {
		return nil, response.ToError("QUERY_PREPARE_ERROR", fmt.Sprintf("Query[prepare](%s,%v)", querySQL, args), err)
	}
	defer func() { _ = stmt.Close() }()
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, response.ToError("QUERY_RUN_ERROR", fmt.Sprintf("Query[run](%s,%v)", querySQL, args), err)
	}
	defer func() { _ = rows.Close() }()

	results := make([]Result, 0)
	err = scan.Rows(&results, rows)

	if err != nil {
		return nil, response.ToError("ERROR_IN_SCAN", fmt.Sprintf("could not scan query into type(%T, %s)", results, querySQL), err)
	}
	return results, nil
}
