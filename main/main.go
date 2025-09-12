package main

import (
	"flag"
	"fmt"

	"github.com/iotames/detl/conf"
	"github.com/iotames/easyconf"
)

var ActiveDsn, Version string

func main() {
	cf := conf.GetConf("")
	fmt.Println("GetScriptDir:", cf.GetScriptDir())
	// "postgres" 必须填写正确
	err := cf.SetActiveDSN(ActiveDsn, "postgres")
	// && !strings.Contains(err.Error(), "has actived")
	if err != nil {
		panic(fmt.Errorf("SetActiveDSN:%s", err))
	}
	name, dsn := cf.GetDefaultDSN()
	fmt.Println("GetDefaultDSN:", name, dsn)
	err = DbPing(name, dsn)
	if err != nil {
		panic(fmt.Errorf("DbPing:%s", err))
	}
	fmt.Println("DbPing:", name, dsn)
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
	flag.StringVar(&Version, "version", "unstable", "显示版本信息")
	flag.Parse()
}
