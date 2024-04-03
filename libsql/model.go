package libsql

import (
	"database/sql"
)

type DataBaseModel struct {
	DB          *sql.DB
	ProgramName string
	ModuleName  string
	SetVariable string
}
