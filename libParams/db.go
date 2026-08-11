package libParams

import (
	"database/sql"

	"gorm.io/gorm"
)

// DbParams holds database connection configuration and runtime DB/ORM handles.
type DbParams struct {
	DataBaseType    string        `yaml:"dbType"`
	DataBaseAddress SecurityParam `yaml:"dbAddress"`
	Db              *sql.DB       `yaml:"-"`
	Orm             *gorm.DB      `yaml:"-"`
}

// GetDB returns the database parameters for the given name.
func (m ApplicationParams[SpecialParams]) GetDB(name string) *DbParams {
	return GetValueFromMap(name, m.DB)
}

// SetDB sets the database parameters for the given name.
func (m ApplicationParams[SpecialParams]) SetDB(name string, db *DbParams) {
	m.DB[name] = *db
}
