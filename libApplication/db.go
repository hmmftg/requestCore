package initiator

import (
	"database/sql"
	"log"

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
