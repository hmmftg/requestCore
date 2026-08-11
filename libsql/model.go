package libsql

import (
	"database/sql"
)

// DataBaseModel holds the database connection and program/module metadata for SQL operations.
type DataBaseModel struct {
	DB          *sql.DB
	ProgramName string
	ModuleName  string
	SetVariable string
}
