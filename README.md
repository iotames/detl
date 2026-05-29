# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

## 简介

- **抽取**：MySQL, PostgreSQL, 文件, API 接口等
- **转换**：内置 Go 函数 或 **Python 脚本**
- **载入**：CSV 文件、控制台输出等

两种运行模式：**传统模式**（环境变量驱动）和**任务模式**（YAML 任务文件驱动）。系统配置与 ETL 业务配置分离。

---

## Hello ETL：MySQL → 清洗转换 → CSV

完整的 ETL 流程，支持**环境变量**和 **YAML 任务文件**两种驱动方式，无需改一行源代码。

### 流程概览

```
MySQL(detl_test.detl_test_users)  ← main/conf/dsn.json
    │ 抽取：e_detl_users.sql       ← main/script/
    ▼
Transform（内置 或 Python 脚本）   ← t_users.py
    │
    ▼
CSV 文件（output/etl_output.csv）
```

### 前置准备

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS detl_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
go test -v -run TestPipeline_MySQL_to_CSV   # 建表并插入数据
```

### 1. 配置数据源

`main/conf/dsn.json` 支持多数据源，每个连接有唯一的 `Name` 供 ETL 任务引用：

```json
{
  "DsnList": [
    {"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres ..."},
    {"Name": "dev_mysql", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/detl_test?charset=utf8mb4"}
  ]
}
```

传统模式下程序根据 `DB_DRIVER` 自动选择；任务模式下通过 `source.connection` 按 `Name` 引用。

### 2. 编写抽取脚本

`main/script/e_detl_users.sql`：

```sql
SELECT id, first_name, last_name, email, age, created_at
FROM detl_test_users
ORDER BY id
```

### 3. 编译并运行

```bash
go build -o main.exe ./main

# 内置转换
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql ./main.exe

# Python 脚本转换
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py ./main.exe
```

### 4. 切换输出方式

```bash
# 控制台输出
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=mysql LOAD_TYPE=stdout ./main.exe

# 透传原始数据（不转换）
CONF_DIR=main/conf SCRIPT_DIR=main/script DB_DRIVER=mysql TRANSFORM_MODE=none ./main.exe
```

### 5. 任务模式运行

上述流程也可通过 YAML 任务文件驱动，系统配置与 ETL 业务配置分离：

```bash
go run ./main -task main/task/user_etl.yaml
```

任务文件 `main/task/user_etl.yaml`：

```yaml
kind: 转换
name: 用户数据清洗
source:
  connection: dev_mysql           # 按连接名引用 dsn.json
  query_file: e_detl_users.sql
transform:
  mode: builtin
load:
  type: csv
  file: etl_output.csv
  columns: [id, full_name, email, age, created_at, source, etl_time]
```

### 6. 输出结果

```csv
id,full_name,email,age,created_at,source,etl_time
1,John Doe,john@example.com,30,2024-01-15,mysql,2026-05-26 19:10:30
2,Jane Smith,jane@example.com,25,2024-02-20,mysql,2026-05-26 19:10:30
3,Bob,bob@test.com,0,2024-03-10,mysql,2026-05-26 19:10:30
4,Alice Wang,alice.wang@test.com,28,2024-04-05,mysql,2026-05-26 19:10:30
5,Lee,null_first@example.com,35,2024-05-01,mysql,2026-05-26 19:10:30
```

---

## Python 脚本转换

`TRANSFORM_MODE=python` 会启动一个常驻 Python 子进程，逐行读取数据、逐行输回转换结果。

### 脚本规范

- 语言：Python 3
- 输入：stdin，每行一个 JSON 对象
- 输出：stdout，每行一个 JSON 对象
- 返回 `null` 或空行可跳过该行
- **必须 flush stdout**（`print(..., flush=True)`）

### 示例：`main/script/t_users.py`

```python
import sys, json

for line in sys.stdin:
    row = json.loads(line.strip())

    first = row.get("first_name") or ""
    last = row.get("last_name") or ""
    row["full_name"] = f"{first} {last}".strip()
    row["email"] = (row.get("email") or "").lower()
    if row.get("age") is None:
        row["age"] = 0

    print(json.dumps(row), flush=True)
```

### 运行

```bash
TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py ./main.exe
```

---

### 环境变量大全

| 模块 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| **Source** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json + system.yaml） |
| | `SCRIPT_DIR` | `script` | ETL 脚本目录 |
| | `DB_DRIVER` | `postgres` | 数据库驱动（`postgres` / `mysql`） |
| | `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| | `ACTIVE_DSN` | （PG 默认值） | 默认 DSN 连接字符串 |
| **Transform** | `TRANSFORM_MODE` | `builtin` | 转换模式（`builtin` / `python` / `none`） |
| | `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本（`python` 模式时生效） |
| **Load** | `LOAD_TYPE` | `csv` | 输出类型（`csv` / `stdout`） |
| | `OUTPUT_DIR` | `output` | 输出目录 |
| | `OUTPUT_FILE` | `etl_output.csv` | 输出文件名 |
| | `OUTPUT_COLUMNS` | `id,full_name,...` | CSV 列名（逗号分隔） |
| **Task** | `TASK_DIR` | `task` | ETL 任务 YAML 目录 |
| | `-task` flag | — | 指定任务文件，启用任务模式 |

---

## 当前状态

项目处于**开发阶段**，已完成：

### ✅ 已实现
| 模块 | 内容 | 测试 |
|---|---|---|
| **Source** | SQL 数据源：PostgreSQL + MySQL | ✅ 集成测试通过 |
| **Transform** | 内置 Go 函数转换 / Python 脚本转换 | ✅ 集成测试通过 |
| **Load** | CSV 写入 / Stdout 控制台输出 | ✅ 集成测试通过 |
| **Engine** | Pipeline 编排（Source → Transform → Load） | ✅ 集成测试通过 |
| **配置** | 环境变量 + DSN 文件管理 + system.yaml | 基础可用 |
| **YAML 任务** | `kind: 转换` + 按连接名引用 DSN | ✅ 编译通过 |
| **作业设计** | `kind: 作业` 结构预留 | ✅ 设计完成 |

### 📋 待实现
- 文件 Source（CSV/JSON）
- Load：SQL 写入（UPSERT）
- 作业执行引擎
- HTTP API 数据源

---

## ETL 业务脚本系统规范

- 存放目录：`script/`
- 命名规范：抽取脚本加前缀 `e_`（如 `e_*.sql`, `e_*.py`），转换脚本加前缀 `t_`（如 `t_*.py`），骨架文件以 `main_` 开头

---

## 架构规划

整体采用 **Pipeline 管道架构**：`Source → Transform → Load`。

系统配置（环境变量 + system.yaml）与 ETL 业务配置（YAML 任务文件）分离。

```
                     ┌─────────────────────────────────┐
                     │      CLI / 配置层                │
                     │  环境变量 | system.yaml | dsn.json│
                     └──────────┬──────────────────────┘
                                │
              ┌─────────────────┼──────────────────┐
              ▼                 ▼                    ▼
     ┌────────────────┐ ┌──────────────┐ ┌──────────────────┐
     │  任务 YAML     │ │  dsn.json    │ │  system.yaml     │
     │  kind: 转换/作业│ │  连接名→DSN   │ │  系统默认配置     │
     └────────┬───────┘ └──────┬───────┘ └──────────────────┘
              │                │
              ▼                ▼
     ┌──────────▼──────────────────────┐
     │          Pipeline Engine         │
     └──────────┬──────────────────────┘
                │
              ┌─┼─┐
              ▼ ▼ ▼
     ┌────────────────┐ ┌──────────────┐ ┌────────────────┐
     │     Source     │ │  Transform   │ │      Load      │
     ├────────────────┤ ├──────────────┤ ├────────────────┤
     │ • SQL (PG/MySQL)│ │ • 内置 Func  │ │ • CSV 文件     │
     │                │ │ • Py 脚本   │ │ • 控制台输出    │
     └────────────────┘ └──────────────┘ └────────────────┘
```

### 包结构

```
detl/
├── internal/                           # ✅ 核心库
│   ├── engine/
│   │   └── pipeline.go                 # ✅ Pipeline 编排
│   ├── source/
│   │   ├── source.go                   # ✅ Source 接口
│   │   └── sql.go                      # ✅ SQL 数据源（PG + MySQL）
│   ├── transform/
│   │   ├── transform.go                # ✅ Transformer 接口 + Func 适配器
│   │   └── pyscript.go                 # ✅ Python 脚本转换
│   ├── load/
│   │   ├── load.go                     # ✅ Load 接口
│   │   ├── csv.go                      # ✅ CSV 写入
│   │   └── stdout.go                   # ✅ 控制台输出
│   └── task/
│       └── task.go                     # ✅ 任务/作业 YAML 定义 + 解析
├── conf/
│   └── conf.go                         # ✅ 配置单例 + DSN 管理 + system.yaml
├── main/                               # 程序入口
│   ├── main.go                         # ✅ 程序入口（支持 -task flag）
│   ├── main_func.go                    # ✅ ETL 编排（传统 + 任务模式）
│   ├── conf/
│   │   ├── dsn.json                    # 🔒 数据源配置（按 Name 引用）
│   │   └── system.yaml                 # 🔒 系统默认配置（可选）
│   ├── script/                         # 🔒 SQL + Python 脚本
│   └── task/                           # 🔒 ETL 任务 YAML 文件
│       ├── user_etl.yaml               # 转换示例
│       ├── user_etl_stdout.yaml        # stdout 输出示例
│       └── daily_job.yaml              # 作业示例（预留）
├── detl_test.go                        # ✅ 集成测试
├── output/                             # 输出文件
├── go.mod
├── CLAUDE.md
├── README.md
└── usage.md
```

---

### 核心接口

```go
type Source interface {
    Open() error
    Read() (map[string]any, bool)
    Close() error
}

type Transformer interface {
    Transform(map[string]any) ([]map[string]any, error)
}

type Load interface {
    Open() error
    Write(map[string]any) error
    Close() error
}

// YAML 任务定义（internal/task/task.go）
type TaskConfig struct {
    Kind      string           // "转换" 或 "作业"
    Name      string
    Source    *SourceConfig
    Transform *TransformConfig
    Load      *LoadConfig
    Tasks     []JobEntry       // 作业子任务
}
```

---

### 阶段实施计划

| 步骤 | 内容 | 说明 | 状态 |
|---|---|---|---|
| 1 | 定义核心接口 + Pipeline Engine | Source/Transform/Load 骨架 | ✅ |
| 2 | 实现 SQL Source（MySQL + Postgres）| 数据库抽取 | ✅ |
| 3 | 实现 Load：CSV / Stdout | 文件输出 + 控制台 | ✅ |
| 4 | 实现 Transform Func 适配器 | 内置 Go 函数转换 | ✅ |
| 5 | 实现 Transform Python 脚本引擎 | Python 脚本转换 | ✅ |
| 6 | YAML 任务定义 + 按名引用 DSN | ETL 业务与系统配置分离 | ✅ |
| 7 | system.yaml 混合配置 | 系统配置支持文件+环境变量覆盖 | ✅ |
| 8 | 作业结构预留 | 多转换集合接口设计 | ✅ |
| 9 | 实现 Load：SQL（UPSERT）| 数据库写入 | 📋 |
| 10 | 实现文件 Source/Load（CSV、JSON）| 文件数据源 | 📋 |
| 11 | 作业执行引擎 | 多转换编排执行 | 📋 |
| 12 | 完整的测试覆盖 | 保证质量 | 📋 |
