# detl ETL 命令行工具 — AI Agent 技能文档

## 简介

detl 是一个 Go 编写的 ETL 命令行工具，支持从 MySQL/PostgreSQL 抽取数据，经过内置函数或 Python 脚本转换，最终写入 CSV 文件、控制台或目标数据库。

两种运行模式：
- **任务模式**（推荐）：ETL 流程定义在 YAML 文件中，系统配置与业务配置分离
- **传统模式**：通过环境变量驱动

---

## 依赖项

| 依赖 | 说明 |
|------|------|
| Go 1.24+ | 编译运行环境 |
| MySQL 8.0+（可选） | 数据源之一 |
| PostgreSQL 15+（可选） | 数据源或目标库 |
| Python 3（可选） | Python 脚本转换模式 |
| `main/conf/dsn.json` | 数据库连接配置 |
| `main/script/` | ETL 业务脚本（SQL + Python） |

---

## 构建

```bash
go build -o main.exe ./main
```

---

## 配置

### dsn.json — 数据库连接注册

文件路径：`main/conf/dsn.json`

每个数据源必须有唯一的 `Name`，ETL 任务通过 `Name` 引用。

```json
{
  "DsnList": [
    {"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable"},
    {"Name": "dev_mysql", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/detl_test?charset=utf8mb4"}
  ]
}
```

### system.yaml — 系统默认配置（可选）

文件路径：`main/conf/system.yaml`

环境变量优先级高于此文件。

```yaml
script_dir: main/script
output_dir: output
```

---

## 运行方式

### 任务模式（推荐）

```bash
go run ./main -task <任务文件路径>
```

任务文件相对于当前目录或 `SCRIPT_DIR` 查找。路径解析优先级：
1. 绝对路径 → 直接读取
2. 相对路径且文件存在 → 直接读取
3. 不存在 → 回退到 `SCRIPT_DIR` 下查找

### 传统模式（环境变量驱动）

```bash
CONF_DIR=main/conf DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql go run ./main
```

两种模式互斥。指定 `-task` 时忽略所有 ETL 业务相关的环境变量。

---

## YAML 任务定义

### 转换（单个 ETL 流程）

```yaml
kind: 转换
name: 用户数据清洗

source:
  connection: dev_pg                  # 引用 dsn.json 中的连接名
  query_file: e_detl_users.sql        # SCRIPT_DIR 下的 SQL 文件
  # query: "SELECT ..."               # 或内联 SQL（二选一）

transform:
  mode: builtin                       # builtin | python | none
  # script: t_users.py                # mode=python 时生效

load:
  type: csv                           # csv | stdout | sql
  file: etl_output.csv                # 输出文件（相对于 OUTPUT_DIR）
  columns: [id, full_name, email, age]
```

### SQL 写入模式

```yaml
load:
  type: sql
  connection: dev_pg                  # 目标连接名
  table: staging.etl_users            # 目标表（支持 schema.table）
  mode: upsert                        # insert | upsert
  key_columns: [id]                   # upsert 唯一键
  create_table: true                  # 自动建表（全部 TEXT 类型）
  batch_size: 100                     # 每批行数
```

### 作业（多个转换的集合，预留）

执行引擎尚未实现，仅作为设计参考。

```yaml
kind: 作业
name: 每日同步
tasks:
  - task: user_etl.yaml
  - task: user_enrich.yaml
```

---

## 环境变量大全

| 模块 | 变量 | 默认值 | 说明 |
|------|------|--------|------|
| **Meta** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json + system.yaml） |
| | `SCRIPT_DIR` | `script` | ETL 业务脚本目录 |
| **Source** | `DB_DRIVER` | `postgres` | 数据库驱动（postgres / mysql） |
| | `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| | `ACTIVE_DSN` | PG 默认 | 默认 DSN 连接字符串 |
| **Transform** | `TRANSFORM_MODE` | `builtin` | 转换模式（builtin / python / none） |
| | `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本路径 |
| **Load** | `LOAD_TYPE` | `csv` | 输出类型（csv / stdout / sql） |
| | `OUTPUT_DIR` | `output` | CSV 输出目录 |
| | `OUTPUT_FILE` | `etl_output.csv` | CSV 文件名（或 SQL 目标表名） |
| | `OUTPUT_COLUMNS` | 默认列 | CSV 列名（逗号分隔） |
| **Task** | `TASK_DIR` | `task` | ETL 任务 YAML 目录 |
| | `-task` flag | — | 指定 YAML 任务文件 |

---

## Python 脚本转换规范

启动一个常驻 Python 子进程，逐行通过 stdin/stdout 以 JSON 通信。

| 方向 | 格式 | 说明 |
|------|------|------|
| stdin | 每行一个 JSON 对象 | 原始数据行输入 |
| stdout | 每行一个 JSON 对象 | 转换后数据行输出 |
| stdout | `null` | 跳过该行（不输出） |
| stderr | 任意文本 | 错误日志透传 |

**要求：**
- Python 3
- 每行输出后必须 `flush=True`
- 返回 `null` 或空行跳过该行

**示例脚本** `main/script/t_users.py`：

```python
import sys, json

for line in sys.stdin:
    row = json.loads(line.strip())
    row["full_name"] = f"{row.get('first_name','')} {row.get('last_name','')}".strip()
    row["email"] = (row.get("email") or "").lower()
    if row.get("age") is None:
        row["age"] = 0
    print(json.dumps(row), flush=True)
```

---

## 常用命令速查

```bash
# ── 构建 ──
go build -o main.exe ./main

# ── 任务模式（推荐）──
go run ./main -task main/task/user_etl.yaml
go run ./main -task main/task/user_etl_stdout.yaml
go run ./main -task main/task/user_etl_to_pg.yaml

# ── 传统模式 ──
# PG → 内置转换 → CSV
CONF_DIR=main/conf DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql go run ./main

# MySQL → Python 转换 → CSV
CONF_DIR=main/conf DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py go run ./main

# 控制台输出
CONF_DIR=main/conf DB_DRIVER=postgres LOAD_TYPE=stdout go run ./main

# 透传原始数据
CONF_DIR=main/conf DB_DRIVER=postgres TRANSFORM_MODE=none go run ./main

# SQL 写入
LOAD_TYPE=sql OUTPUT_FILE=etl_target go run ./main

# ── 测试 ──
go test -v -run TestPipeline_PG_to_CSV
go test -v -run TestPipeline_MySQL_to_CSV
go test -v -run TestSQLLoad_MySQL_to_PG    # 全链路：MySQL→Python→PG
```

---

## 完整场景示例

### 场景 1：MySQL 清洗后写入 CSV

```bash
# 1. 确保 MySQL 有测试数据
go test -v -run TestPipeline_MySQL_to_CSV

# 2. 运行 ETL 任务
go run ./main -task main/task/user_etl.yaml
```

### 场景 2：MySQL → Python 清洗 → PG 写入

```yaml
# main/task/user_etl_to_pg.yaml
kind: 转换
name: 用户数据同步到 PG
source:
  connection: dev_mysql
  query_file: e_detl_users.sql
transform:
  mode: python
  script: t_users.py
load:
  type: sql
  connection: dev_pg
  table: staging.etl_users
  mode: upsert
  key_columns: [id]
  create_table: true
  batch_size: 100
```

```bash
go run ./main -task main/task/user_etl_to_pg.yaml
```

---

## 目录结构

```
detl/
├── main/                       # 程序入口
│   ├── main.go                # 入口（支持 -task flag）
│   ├── main_func.go           # ETL 编排
│   ├── conf/
│   │   ├── dsn.json           # 数据源配置（连接名 → DSN）
│   │   └── system.yaml        # 系统默认配置（可选）
│   ├── script/                 # ETL 业务脚本（SQL + Python）
│   └── task/                   # ETL 任务 YAML 文件
├── conf/
│   └── conf.go                 # 配置管理（DSN + system.yaml）
├── internal/
│   ├── engine/                 # Pipeline 编排
│   ├── source/                 # 数据抽取（SQL）
│   ├── transform/              # 数据转换（内置 + Python）
│   ├── load/                   # 数据载入（CSV + Stdout + SQL）
│   └── task/                   # YAML 任务解析
├── output/                     # 输出文件
├── SKILL.md                    # 本文件
├── README.md
└── CLAUDE.md
```

---

## 注意事项

- 任务模式与传统模式互斥，`-task` 指定后忽略 ETL 业务 env 变量
- `builtin` 转换为硬编码示例（针对 `detl_test_users` 表），非通用
- 作业（`kind: 作业`）执行引擎尚未实现
- 数据库连接按 `Name` 引用，旧版 dsn.json（无 Name）自动回退到按驱动名匹配
- SQL Load 自动建表时所有列使用 TEXT 类型
