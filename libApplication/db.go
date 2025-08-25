package initiator

import (
	"database/sql"
	"log"

	"github.com/glebarez/sqlite"
	oracle "github.com/godoes/gorm-oracle"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/hmmftg/requestCore/libParams"
)

func InitDB(dbParams *libParams.DbParams) {
	db, err := sql.Open(dbParams.DataBaseType, dbParams.DataBaseAddress.Value)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln("db ping", err)
	}

	dbParams.Db = db

	var orm *gorm.DB

	switch dbParams.DataBaseType {
	case "sqlite":
		orm, err = gorm.Open(sqlite.Open(dbParams.DataBaseAddress.Value), &gorm.Config{})
	case "mysql":
		orm, err = gorm.Open(mysql.Open(dbParams.DataBaseAddress.Value), &gorm.Config{})
	case "postgres":
		orm, err = gorm.Open(postgres.Open(dbParams.DataBaseAddress.Value), &gorm.Config{})
	case "oracle":
		orm, err = gorm.Open(oracle.Open(dbParams.DataBaseAddress.Value), &gorm.Config{})
	default:
		log.Fatal("Unsupported database type")
	}

	dbParams.Orm = orm
}

func InitDataBases(params libParams.ParamInterface, dbNames []string) {
	for id := range dbNames {
		db := params.GetDB(dbNames[id])
		if db == nil {
			log.Println("invalid db name provided", dbNames[id])
			continue
		}
		InitDB(db)
		params.SetDB(dbNames[id], db)
	}
}
