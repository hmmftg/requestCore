package libParams

import (
	"database/sql"

	"gorm.io/gorm"
)

type DbParams struct {
	DataBaseType    string        `yaml:"dbType"`
	DataBaseAddress SecurityParam `yaml:"dbAddress"`
	Db              *sql.DB       `yaml:"-"`
	Orm             *gorm.DB      `yaml:"-"`
}

func (m ApplicationParams[SpecialParams]) GetDB(name string) *DbParams {
	return GetValueFromMap(name, m.DB)
}

func (m ApplicationParams[SpecialParams]) SetDB(name string, db *DbParams) {
	m.DB[name] = *db
}
