# detl 使用指南

## 概述

detl 是一个命令行 ETL 工具，支持两种运行模式：

1. **传统模式** — 通过环境变量驱动（无需 YAML 配置）
2. **任务模式** — 通过 YAML 任务文件驱动（推荐）

系统配置（数据库连接、目录路径）通过环境变量管理，ETL 业务逻辑通过 YAML 任务文件定义，两者分离。

---

## 安装

```bash
go build -o main.exe ./main
```

---

## 模式一：任务模式（推荐）

将 ETL 流程定义在 YAML 文件中，通过 `-task` 参数运行。

### 运行

```bash
# 指定任务文件
go run ./main -task user_etl.yaml

# 指定 TASK_DIR（默认 task），-task 使用相对路径
TASK_DIR=main/task go run ./main -task user_etl.yaml
```

### 转换任务（单个 ETL 流程）

```yaml
# main/task/user_etl.yaml
kind: 转换
name: 用户数据清洗

source:
  connection: dev_pg                  # 引用 dsn.json 中的连接名
  query_file: e_detl_users.sql        # SQL 脚本（相对于 SCRIPT_DIR）
  # query: "SELECT ..."               # 或内联 SQL（二选一）

transform:
  mode: builtin                       # builtin | python | none
  # script: t_users.py                # mode=python 时生效

load:
  type: csv                           # csv | stdout
  file: etl_output.csv                # 输出文件（相对于 OUTPUT_DIR）
  columns:
    - id
    - full_name
    - email
    - age
```

### 作业（多个转换的集合）

执行引擎尚未实现，预留设计中：

```yaml
# main/task/daily_job.yaml
kind: 作业
name: 每日用户数据同步

tasks:
  - task: user_etl.yaml               # 第一步
  - task: user_enrich.yaml            # 第二步（预留）
  - task: user_load_db.yaml           # 第三步（预留）
```

### dsn.json — 按连接名引用

数据源配置新增 `Name` 字段作为连接标识：

```json
{
  "DsnList": [
    {"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres..."},
    {"Name": "dev_mysql", "DriverName": "mysql", "Dsn": "root:root@..."}
  ]
}
```

任务文件中的 `source.connection` 通过 `Name` 匹配。旧版 `dsn.json`（无 Name）自动按驱动名回退。

### system.yaml — 系统配置覆盖（可选）

```yaml
# main/conf/system.yaml
script_dir: main/script
output_dir: output
```

环境变量优先级高于 `system.yaml`。

---

## 模式二：传统模式（环境变量驱动）

### 环境变量

| 模块 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| **Source** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json） |
| | `SCRIPT_DIR` | `script` | ETL 业务脚本目录 |
| | `DB_DRIVER` | `postgres` | 数据库驱动：`postgres` / `mysql` |
| | `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| | `ACTIVE_DSN` | （PG 默认） | 默认 DSN 连接字符串 |
| **Transform** | `TRANSFORM_MODE` | `builtin` | 转换模式：`builtin` / `python` / `none` |
| | `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本（`python` 模式） |
| **Load** | `LOAD_TYPE` | `csv` | 输出类型：`csv` / `stdout` |
| | `OUTPUT_DIR` | `output` | 输出目录 |
| | `OUTPUT_FILE` | `etl_output.csv` | 输出文件名 |
| | `OUTPUT_COLUMNS` | `id,full_name,...` | CSV 列名（逗号分隔） |
| **Task** | `TASK_DIR` | `task` | ETL 任务 YAML 目录 |
| | `-task` flag | — | 指定任务文件（启用任务模式） |

### Source（数据抽取）

```bash
DB_DRIVER=postgres SCRIPT_FILE=e_getusers.sql ./main.exe
DB_DRIVER=mysql    SCRIPT_FILE=e_detl_users.sql ./main.exe
```

程序从 `CONF_DIR/dsn.json` 中查找与 `DB_DRIVER` 匹配的 DSN。

### Transform（数据转换）

#### 内置转换（builtin）

默认模式。自动执行：姓名拼接、邮箱小写、NULL age → 0、日期格式化。

```bash
TRANSFORM_MODE=builtin ./main.exe
```

#### Python 脚本（python）

启动常驻 Python 子进程，逐行通过 stdin/stdout 以 JSON 通信。

```bash
TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py ./main.exe
```

**Python 脚本接口：**

| 方向 | 格式 | 说明 |
|---|---|---|
| stdin（输入） | 每行一个 JSON | 原始数据行 |
| stdout（输出） | 每行一个 JSON | 转换后的数据行 |
| stdout | `null` | 跳过该行（不输出） |
| stderr | 任意 | 错误日志透传 |

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

#### 透传（none）

不对数据做任何转换，原始数据直接输出：

```bash
TRANSFORM_MODE=none ./main.exe
```

### Load（数据载入）

#### CSV 文件

```bash
LOAD_TYPE=csv OUTPUT_DIR=output OUTPUT_FILE=result.csv ./main.exe
```

#### 控制台输出

```bash
LOAD_TYPE=stdout ./main.exe
```

### SQL 写入

任务模式：在 YAML 中配置 `load.type: sql`。

```yaml
load:
  type: sql
  connection: dev_pg                  # dsn.json 中的目标连接名
  table: staging.etl_users            # 目标表（支持 schema.table）
  mode: upsert                        # insert | upsert
  key_columns: [id]                   # upsert 唯一键
  create_table: true                  # 自动建表（所有列 TEXT）
  batch_size: 100                     # 每批行数
```

传统模式：

```bash
LOAD_TYPE=sql OUTPUT_FILE=etl_users ./main.exe
```

（传统模式使用 `OUTPUT_FILE` 作为表名，使用源库同一连接写入）

---

## 完整示例

## 完整示例

### 示例 1：Postgres → 内置转换 → CSV（传统模式）

```bash
CONF_DIR=main/conf SCRIPT_DIR=main/script \
  DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=builtin \
  LOAD_TYPE=csv OUTPUT_FILE=pg_users.csv \
  ./main.exe
```

### 示例 2：MySQL → Python 转换 → Stdout（传统模式）

```bash
CONF_DIR=main/conf SCRIPT_DIR=main/script \
  DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py \
  LOAD_TYPE=stdout \
  ./main.exe
```

### 示例 3：任务模式

```bash
go run ./main -task main/task/user_etl.yaml
```

### 示例 4：MySQL → Python 清洗 → PostgreSQL 写入

```bash
go run ./main -task main/task/user_etl_to_pg.yaml
```

任务文件 `main/task/user_etl_to_pg.yaml`：

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

---

## 目录结构

```
detl/
├── conf/                   # 配置文件目录
│   └── conf.go            # 配置单例 + DSN 管理 + system.yaml 加载
├── script/                 # ETL 业务脚本
├── main/                   # 程序入口
│   ├── main.go
│   ├── main_func.go
│   ├── conf/
│   │   ├── dsn.json       # 数据源配置（带 Name 连接名）
│   │   └── system.yaml    # 系统默认配置（可选）
│   ├── script/             # SQL + Python 脚本
│   └── task/               # ETL 任务 YAML 文件
│       ├── user_etl.yaml
│       ├── user_etl_stdout.yaml
│       └── daily_job.yaml
├── internal/
│   ├── engine/             # Pipeline 编排
│   ├── source/             # 数据抽取（SQL: PG + MySQL）
│   ├── transform/          # 数据转换（Func + Python）
│   ├── load/               # 数据载入（CSV + Stdout）
│   └── task/               # 任务定义 + YAML 解析
├── output/                 # 输出文件
├── CLAUDE.md               # AI 辅助开发指南
├── README.md
└── usage.md
```

## 注意事项

- 任务模式与传统模式互斥：指定 `-task` 时忽略所有 ETL 业务相关的环境变量（`SCRIPT_FILE`、`TRANSFORM_MODE` 等）
- 作业（`kind: 作业`）的执行引擎尚未实现，目前仅列出子任务后提示未完成
- `builtin` 转换为硬编码示例（针对 `detl_test_users` 表结构），非通用实现
- 数据库连接通过 `dsn.json` 中的 `Name` 引用，旧版无 Name 的配置自动回退为按驱动名匹配
