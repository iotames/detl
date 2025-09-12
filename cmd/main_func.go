package main

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/iotames/easydb"
	_ "github.com/lib/pq"
)

func DbPing(key, dsn string) error {
	var sqldb *sql.DB
	var err error
	sqldb, err = sql.Open(key, dsn)
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
		log.Printf("---GetMany--row(%d)---result(%+v)----\n", i, d.DecodeInterface(datalist[i]))
	}
	return err
}
