# CLAUDE.md

本文档为 Claude Code（claude.ai/code）在此仓库中工作时提供指导。

## 常用命令

```bash
# 构建
go build ./...

# 构建主入口
go build -o main.exe ./main

# 运行测试（需要 PG 在 localhost:5432，user=postgres, password=postgres）
go test -v -run TestPipeline_PG_to_CSV

# MySQL 测试（需要 MySQL 在 localhost:3306，root:root）
go test -v -run TestPipeline_MySQL_to_CSV

# 运行所有测试
go test -v ./...

# 代码检查
go vet ./...

# 运行 ETL 任务（YAML 任务模式）
TASK_DIR=main/task go run ./main -task user_etl.yaml

# 任务模式 + stdout
TASK_DIR=main/task go run ./main -task user_etl_stdout.yaml

# 运行（环境变量驱动）
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql go run ./main

# Python 脚本转换
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py go run ./main

# 控制台输出
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=postgres LOAD_TYPE=stdout go run ./main

# 透传原始数据（不转换）
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=postgres TRANSFORM_MODE=none go run ./main
```

## 架构

Pipeline 管道模式：**Source → Transform → Load**

系统配置（环境变量 + system.yaml）和 ETL 业务配置（YAML 任务文件）分离。

```
                     ┌─────────────────────────────────┐
                     │      main/main.go + conf         │
                     │   (环境变量/flag 解析, DSN 初始化) │
                     │   system.yaml（可选系统配置覆盖）  │
                     └──────────┬──────────────────────┘
                                │
              ┌─────────────────┼──────────────────┐
              ▼                 ▼                    ▼
     ┌────────────────┐ ┌──────────────┐ ┌──────────────────┐
     │  main/task/*.yaml││   dsn.json   │ │  system.yaml     │
     │  ETL 业务定义    ││  连接名映射   │ │  系统默认配置     │
     │  kind: 转换/作业 ││  name→DSN    │ │  env 可覆盖       │
     └────────┬───────┘ └──────┬───────┘ └──────────────────┘
              │                │
              ▼                ▼
     ┌──────────▼──────────────────────┐
     │   engine.Pipeline (Run)          │
     │   打开 Source → 迭代 Read() →    │
     │   对每行 Transform → Load.Write   │
     └──────────┬──────────────────────┘
                │
              ┌─┼─┐
              ▼ ▼ ▼
     ┌────────────────┐ ┌──────────────┐ ┌────────────────┐
     │  source.Source │ │transform.   │ │   load.Load    │
     │                │ │ Transformer  │ │                │
     │ • source.sql   │ │ • transform. │ │ • load.csv     │
     │   (PG+MySQL)   │ │   Func 适配器│ │ • load.stdout  │
     │                │ │ • transform. │ │                │
     │                │ │   pyscript   │ │                │
     └────────────────┘ └──────────────┘ └────────────────┘
```

### 核心接口（`internal/`）

```go
source.Source          → Open(), Read() (map[string]any, bool), Close()
transform.Transformer  → Transform(map[string]any) ([]map[string]any, error)
load.Load              → Open(), Write(map[string]any) error, Close()
```

### 实现

| 接口 | 包 | 说明 |
|------|-----|------|
| Source | `internal/source/sql.go` | `NewSQL(SQLConfig)` — 支持 PG（`github.com/lib/pq`）和 MySQL（`github.com/go-sql-driver/mysql`）。打开数据库 → 执行查询 → 迭代 `Rows.Scan()` 到 `map[string]any` |
| Transformer | `internal/transform/transform.go` | `Func` 适配器：将 `func(map[string]any)([]map[string]any, error)` 转为 `Transformer` |
| Transformer | `internal/transform/pyscript.go` | `NewPython(PythonConfig)` — 启动常驻 Python 子进程，通过 stdin/stdout 传递 JSON 行 |
| Load | `internal/load/csv.go` | `NewCSV(CSVConfig)` — 写入 CSV，可配置列顺序 |
| Load | `internal/load/stdout.go` | `NewStdout(cols)` — 以 CSV 格式打印到控制台 |

### Pipeline 引擎（`internal/engine/pipeline.go`）

`Pipeline.Run()`：调用 `source.Open()` → 循环 `source.Read()` → 对每行调用 `transformer.Transform()` → 对每个输出行调用 `load.Write()`。转换失败则跳过该行，写入失败则中止。

### 程序入口（`main/main.go` + `main/main_func.go`）

`init()` 从环境变量读取所有配置（通过 `github.com/iotames/easyconf`）和 flag。`runETL()` 根据配置构建 Source/Transformer/Load，组装成 Pipeline 并运行。

## ETL 任务 YAML（`internal/task/task.go`）

任务文件放在 `TASK_DIR`（默认 `task`）目录下，通过 `-task` 参数指定。

### 转换（单个 ETL 流程）

```yaml
kind: 转换
name: 用户数据清洗
source:
  connection: dev_pg              # 引用 dsn.json 中的连接名
  query_file: e_detl_users.sql    # SQL 脚本文件
  # query: "SELECT ..."           # 或内联 SQL（二选一）
transform:
  mode: builtin                   # builtin | python | none
  # script: t_users.py            # mode=python 时生效
load:
  type: csv                       # csv | stdout
  file: etl_output.csv
  columns: [id, full_name, email, age]
```

### 作业（多个转换的集合，预留）

```yaml
kind: 作业
name: 每日同步
tasks:
  - task: user_etl.yaml
  - task: user_enrich.yaml
```

DSN 连接按 `Name` 字段引用。旧版 dsn.json（无 Name 字段）会自动回退为按驱动名匹配。

### 配置系统（`conf/conf.go`）

- 通过 `sync.Once` 实现单例 — `GetConf(confdir)` 只创建一次
- DSN 管理：`InitDSN(driverName, dsn)` 在 `dsn.json` 不存在时创建；`SetActiveDSN()` 激活分组中的 DSN
- DSN 文件格式：JSON，结构为 `DsnList[]` 包含 `{DriverName, Dsn}` 条目

### 模块依赖（`go.mod`）

`github.com/iotames/detl` — Go 1.24.1。依赖由 `iotames` 在 GitHub 上维护：

- `github.com/iotames/easyconf` — 环境变量绑定
- `github.com/iotames/easydb/easydb` — DB 封装（已内联到 `easydb/` 目录）
- `github.com/iotames/easydb/dsn` — DSN 分组管理
- `github.com/iotames/miniutils` — 文件/MD5 工具
- `github.com/lib/pq` — PostgreSQL 驱动
- `github.com/go-sql-driver/mysql` — MySQL 驱动

## 环境变量大全

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CONF_DIR` | `conf` | 配置目录（存放 dsn.json） |
| `SCRIPT_DIR` | `script` | 脚本目录 |
| `DB_DRIVER` | `postgres` | 数据库驱动（`postgres` / `mysql`） |
| `ACTIVE_DSN` | PG 默认 | 默认 DSN 连接字符串 |
| `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| `TRANSFORM_MODE` | `builtin` | 转换模式（`builtin` / `python` / `none`） |
| `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本（`python` 模式时生效） |
| `LOAD_TYPE` | `csv` | 输出类型（`csv` / `stdout`） |
| `OUTPUT_DIR` | `output` | 输出目录 |
| `OUTPUT_FILE` | `etl_output.csv` | 输出文件名 |
| `OUTPUT_COLUMNS` | 默认列名 | CSV 列名（逗号分隔） |
| `TASK_DIR` | `task` | ETL 任务 YAML 目录 |
| `-task` flag | — | 指定任务文件，启用任务模式 |

## Python 脚本转换

常驻 Python 子进程。协议：stdin/stdout，每行一个 JSON 对象。

- 输出时必须 `flush=True`
- 返回 `null` 或空行可跳过该行
- 示例见 `main/script/t_users.py`

## 注意事项

- `main.go:init()` 中 `GetConf("")` 被调用了两次 — 第一次使用 `ConfDir`，第二次使用空字符串。但 `sync.Once` 确保只有第一次生效，因此 `conf.dirPath` 实际由首次 `GetConf(ConfDir)` 设置。后续的 `SetScriptDir` 和 `InitDSN` 会正确完成配置初始化。
- `builtin` 转换（`main_func.go:78-132`）硬编码为 `detl_test_users` 表结构（id, first_name, last_name, email, age, created_at），不具有通用性。
- Python 转换优先使用 `"python"` 命令，找不到时回退 `"python3"` — 在 Windows 上可能解析到 Microsoft Store 存根。
- 数据库不可用时测试会跳过（helper 先 ping，失败则 Fatal）。
- 模块路径为 `github.com/iotames/detl`，根包为 `package detl`（不是 `package main`）。
- `easydb/` 目录是 `github.com/iotames/easydb` 的内联副本，通过 `go.mod` 的 `replace` 指令使用本地版本。
- DSN 配置使用 MD5 哈希（`miniutils.Md5`）做 DSN 去重，非加密用途。
- `easydb/dsn.DataSource` 新增 `Name` 字段用于连接名引用，旧版 dsn.json（无 Name 字段）自动兼容。
- 任务模式（`-task`）和环境变量模式完全独立。任务模式从 YAML 读取 ETL 业务配置，系统配置仍来自环境变量。
- 作业（`kind: 作业`）的执行引擎尚未实现，运行时仅列出子任务后返回错误。
