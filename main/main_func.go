package main

import (
	"database/sql"
	"log"

	pkgdsn "github.com/iotames/detl/pkg/dsn"
	"github.com/iotames/easydb"
)

func dbTest() error {
	var sqldb *sql.DB
	var err error
	dsnconf := pkgdsn.GetDsnConf(nil)
	dgp := pkgdsn.DsnGroup{}
	dsnconf.GetDsnGroup(&dgp)
	for _, ds := range dgp.DsnList {
		sqldb, err = sql.Open(ds.DriverName, ds.Dsn)
		if err != nil {
			return err
		}

		d := easydb.NewEasyDbBySqlDB(sqldb)
		// 这个很少用。是关闭整个连接池。
		defer d.CloseDb()

		var datalist []map[string]interface{}

		err = d.GetMany("SELECT id, name, age, wallet_balance FROM users", &datalist)
		if err != nil {
			return err
		}
		for i := range datalist {
			log.Printf("---GetMany--dsncode(%s)--row(%d)---result(%+v)----\n", ds.Code, i, d.DecodeInterface(datalist[i]))
		}
	}
	return err
}
