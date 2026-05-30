package main

import (
	"flag"
	"fmt"

	"github.com/iotames/detl/conf"
	"github.com/iotames/easyconf"
)

var (
	ActiveDsn, Version string
	cf                 *conf.Conf

	// Source 配置
	DbDriver   string
	ScriptFile string

	// Transform 配置
	TransformMode   string
	TransformScript string

	// Load 配置
	LoadType      string
	OutputDir     string
	OutputFile    string
	OutputColumns string

	// Task 模式
	TaskFile string
	TaskDir  string
)

func main() {
	flag.Parse()
	// 兼容旧版：第二次 GetConf 不改变单例，仅为了与旧版顺序一致
	cf = conf.GetConf("")
	fmt.Println("GetScriptDir:", cf.GetScriptDir())
	cf.InitDSN(DbDriver, ActiveDsn)

	if TaskFile != "" {
		err := runETLFromTask(TaskFile)
		if err != nil {
			panic(fmt.Errorf("runETLFromTask:%s", err))
		}
		return
	}
	err := runETL()
	if err != nil {
		panic(fmt.Errorf("runETL:%s", err))
	}
}

func init() {
	var ConfDir, ScriptDir string
	cf = conf.GetConf(ConfDir)
	cf.SetScriptDir(ScriptDir)
	env := easyconf.NewConf()
	env.StringVar(&ConfDir, "CONF_DIR", "conf", "配置目录")
	env.StringVar(&ScriptDir, "SCRIPT_DIR", "script", "ETL业务脚本目录")

	// Source
	env.StringVar(&DbDriver, "DB_DRIVER", "postgres", "数据库驱动(postgres/mysql)")
	env.StringVar(&ActiveDsn, "ACTIVE_DSN", "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable search_path=public", "默认DSN")
	env.StringVar(&ScriptFile, "SCRIPT_FILE", "e_detl_users.sql", "抽取脚本文件名")

	// Transform
	env.StringVar(&TransformMode, "TRANSFORM_MODE", "builtin", "转换模式(builtin/python/none)")
	env.StringVar(&TransformScript, "TRANSFORM_SCRIPT", "t_users.py", "转换脚本文件名(TRANSFORM_MODE=python时生效)")

	// Load
	env.StringVar(&LoadType, "LOAD_TYPE", "csv", "输出类型(csv/stdout)")
	env.StringVar(&OutputDir, "OUTPUT_DIR", "output", "输出目录")
	env.StringVar(&OutputFile, "OUTPUT_FILE", "etl_output.csv", "输出文件名")
	env.StringVar(&OutputColumns, "OUTPUT_COLUMNS", "id,full_name,email,age,created_at,source,etl_time", "CSV列名(逗号分隔)")
	env.StringVar(&TaskDir, "TASK_DIR", "task", "ETL任务目录")
	env.StringVar(&TaskFile, "task", "", "ETL任务文件（YAML），启用任务模式")
	env.StringVar(&Version, "version", "unstable", "显示版本信息")
	env.Parse(true)
}
