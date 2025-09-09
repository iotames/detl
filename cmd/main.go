package main

import (
	"fmt"

	"github.com/iotames/detl/conf"
	"github.com/iotames/easyconf"
	// "github.com/iotames/easydb"
)

var ActiveDsn string

func main() {
	cf := conf.GetConf("")
	fmt.Println("GetScriptDir:", cf.GetScriptDir())
	cf.SetActiveDSN(ActiveDsn, "postgres123")
	dsn := cf.GetDSN("postgres123")
	fmt.Println(dsn)
}

func init() {
	var ConfDir, ScriptDir string
	env := easyconf.NewConf()
	env.StringVar(&ConfDir, "CONF_DIR", "conf", "用户配置目录")
	env.StringVar(&ScriptDir, "SCRIPT_DIR", "script", "低代码脚本目录")
	env.StringVar(&ActiveDsn, "ACTIVE_DSN", "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable search_path=public", "默认的DSN数据源")
	env.Parse()
	cf := conf.GetConf(ConfDir)
	cf.SetScriptDir(ScriptDir)
}
