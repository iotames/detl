package main

import (
	"fmt"

	"github.com/iotames/detl/cmd/detl/script"
	"github.com/iotames/detl/conf"
	"github.com/iotames/detl/hotswap"
	"github.com/iotames/easyconf"
)

var ConfDir, ScriptDir string

const appVersion = "unstable"

var (
	ActiveDsn string
	cf        *conf.Conf

	// Source 配置
	DbDriver   string
	ScriptFile string

	// Transform 配置
	TransformMode   string
	TransformScript string

	// Load 配置（文件名和列名由 YAML 任务定义，环境变量模式使用固定默认值）
	LoadType  string
	OutputDir string

	// Task 模式
	TaskFile string

	// DB 测试
	DBTest  bool
	DsnName string

	showVersion bool
)

func main() {
	if showVersion {
		fmt.Println(appVersion)
		return
	}
	cf = conf.GetConf()
	fmt.Println("ScriptDir:", ScriptDir)
	cf.InitDSN(DbDriver, ActiveDsn)

	if DBTest {
		err := runDBTest(DsnName)
		if err != nil {
			panic(fmt.Errorf("runDBTest:%s", err))
		}
		return
	}

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

func initScript() {
	sqldir := hotswap.NewScriptDir(script.GetScriptFs(), ScriptDir)
	hotswap.SetScriptDir(sqldir)
}

func init() {
	parseEnv()
	initConf()
	initScript()
}

func parseEnv() {
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

	// Load（文件名和列名由 YAML 任务定义，环境变量模式使用固定默认值）
	env.StringVar(&LoadType, "LOAD_TYPE", "csv", "输出类型(csv/stdout)")
	env.StringVar(&OutputDir, "OUTPUT_DIR", "output", "输出目录")
	env.StringVar(&TaskFile, "task", "", "ETL任务文件（YAML），启用任务模式")
	env.BoolVar(&showVersion, "version", false, "显示版本信息并退出")

	// DB 测试
	env.BoolVar(&DBTest, "dbtest", false, "数据库连通性测试模式")
	env.StringVar(&DsnName, "dsnName", "", "要测试的连接名称（dsn.json 中的 Name），为空则测试全部")

	env.Parse(true)
}

func initConf() {
	if err := conf.SetConf(ConfDir); err != nil {
		panic(fmt.Errorf("SetConf error:%s", err))
	}
	cf = conf.GetConf()
	cf.SetScriptDir(ScriptDir)
}
