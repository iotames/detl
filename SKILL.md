# detl ETL 命令行工具 — AI Agent 技能文档

## 简介

detl 是一个 ETL 命令行工具，从 MySQL/PostgreSQL 抽取数据，经过内置函数或 Python 脚本转换，写入 CSV、控制台或目标数据库。

- **运行方式**：Windows 执行 `detl.exe`，Linux/Mac 执行 `./detl`
- **两种模式**：**任务模式**（推荐，YAML 驱动）和**传统模式**（环境变量驱动）

---

## 快速开始

### 1. 配置数据库连接

编辑 `conf/dsn.json`，注册数据源：

```json
{
  "DsnList": [
    {"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable"},
    {"Name": "dev_mysql", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/detl_test?charset=utf8mb4"}
  ]
}
```

### 2. 运行任务

```bash
# Windows
detl.exe -task task/user_etl.yaml

# Linux / Mac
./detl -task task/user_etl.yaml
```

---

## 运行方式

### 任务模式（推荐）

ETL 流程定义在 YAML 中，通过 `-task` 参数指定：

```bash
detl.exe -task task/user_etl.yaml
```

任务文件查找优先级：
1. 绝对路径 → 直接读取
2. 相对路径且文件存在 → 直接读取
3. 不存在 → 回退到 `SCRIPT_DIR` 下查找

### 传统模式（环境变量驱动）

```bash
# Windows
set CONF_DIR=conf && set DB_DRIVER=postgres && set SCRIPT_FILE=e_detl_users.sql && detl.exe

# Linux / Mac
CONF_DIR=conf DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql ./detl
```

两种模式互斥。指定 `-task` 时忽略所有 ETL 业务环境变量。

---

## 配置

### dsn.json — 数据库连接注册

路径：`conf/dsn.json`

每个数据源必须有唯一的 `Name`，ETL 任务通过 `Name` 引用。

### system.yaml — 系统默认配置（可选）

路径：`conf/system.yaml`

```yaml
script_dir: script
output_dir: output
```

环境变量优先级高于此文件。

---

## YAML 任务定义

### 转换（单个 ETL 流程）

```yaml
kind: 转换
name: 用户数据清洗

source:
  connection: dev_pg                  # 引用 dsn.json 中的连接名
  query_file: e_detl_users.sql        # SQL 脚本文件
  # query: "SELECT ..."               # 或内联 SQL（二选一）

transform:
  mode: builtin                       # builtin | python | none
  # script: t_users.py                # mode=python 时生效

load:
  type: csv                           # csv | stdout | sql
  file: etl_output.csv                # 输出文件
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

```yaml
kind: 作业
name: 每日同步
tasks:
  - task: user_etl.yaml
  - task: user_enrich.yaml
```

执行引擎尚未实现。

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

**示例脚本** `script/t_users.py`：

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

## 场景示例

### MySQL → 内置转换 → CSV

```yaml
# task/user_etl.yaml
kind: 转换
name: 用户数据清洗
source:
  connection: dev_mysql
  query_file: e_detl_users.sql
transform:
  mode: builtin
load:
  type: csv
  file: etl_output.csv
  columns: [id, full_name, email, age, created_at, source, etl_time]
```

```bash
detl.exe -task task/user_etl.yaml
```

### MySQL → Python 清洗 → PostgreSQL 写入

```yaml
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
detl.exe -task task/user_etl_to_pg.yaml
```

### 传统模式一行命令

```bash
# PG → 内置转换 → CSV
CONF_DIR=conf DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql detl.exe

# MySQL → 透传 → 控制台
CONF_DIR=conf DB_DRIVER=mysql TRANSFORM_MODE=none LOAD_TYPE=stdout detl.exe
```

---

## 用户目录结构

```
detl/
├── detl.exe                  # 可执行文件（Windows）
├── conf/
│   ├── dsn.json              # 数据源配置
│   └── system.yaml           # 系统默认配置（可选）
├── script/                   # ETL 业务脚本（SQL + Python）
├── task/                     # ETL 任务 YAML 文件
└── output/                   # CSV 输出目录
```

---

## 注意事项

- 任务模式与传统模式互斥，`-task` 指定后忽略 ETL 业务 env 变量
- `builtin` 转换为硬编码示例（针对 `detl_test_users` 表），非通用
- 作业（`kind: 作业`）执行引擎尚未实现
- 数据库连接按 `Name` 引用，旧版 dsn.json（无 Name）自动回退到按驱动名匹配
- SQL Load 自动建表时所有列使用 TEXT 类型
