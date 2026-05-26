# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

## 简介

- **抽取**：MySQL, PostgreSQL, 文件, API 接口等
- **转换**：内置 Go 函数 或 **Python 脚本**
- **载入**：CSV 文件、控制台输出等

---

## Hello ETL：MySQL → 清洗转换 → CSV

完整的 ETL 流程，通过**环境变量和配置文件**驱动，无需改一行源代码。

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

`main/conf/dsn.json` 支持多数据源，程序根据 `DB_DRIVER` 自动选择：

```json
{
  "DsnList": [
    {"DriverName": "postgres", "Dsn": "user=postgres ..."},
    {"DriverName": "mysql",    "Dsn": "root:root@tcp(127.0.0.1:3306)/detl_test?charset=utf8mb4"}
  ]
}
```

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

### 5. 输出结果

`output/etl_output.csv`：

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
| **Source** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json） |
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
| **配置** | 环境变量 + 命令行 + DSN 文件管理 | 基础可用 |

### 📋 待实现
- 文件 Source（CSV/JSON）
- Load：SQL 写入（UPSERT）
- CLI 命令行工具（`-task`, `-dir`）
- YAML/JSON 任务定义
- HTTP API 数据源

---

## ETL 业务脚本系统规范

- 存放目录：`script/`
- 命名规范：抽取脚本加前缀 `e_`（如 `e_*.sql`, `e_*.py`），转换脚本加前缀 `t_`（如 `t_*.py`），骨架文件以 `main_` 开头

---

## 架构规划

整体采用 **Pipeline 管道架构**：`Source → Transform → Load`。

```
                     ┌─────────────────────────────────┐
                     │          CLI / 配置层            │
                     │   (环境变量, JSON/YAML 任务)      │
                     └──────────┬──────────────────────┘
                                │
                     ┌──────────▼──────────────────────┐
                     │          Pipeline Engine         │
                     └──────────┬──────────────────────┘
                                │
              ┌─────────────────┼──────────────────┐
              ▼                 ▼                   ▼
     ┌────────────────┐ ┌──────────────┐ ┌────────────────┐
     │     Source     │ │  Transform   │ │      Load      │
     ├────────────────┤ ├──────────────┤ ├────────────────┤
     │ • SQL (PG/MySQL)│ │ • 内置 Func  │ │ • CSV 文件     │
     │ • CSV/JSON 文件 │ │ • Py 脚本   │ │ • SQL (UPSERT) │
     │ • HTTP API     │ │              │ │ • 控制台输出    │
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
│   │   ├── sql.go                      # ✅ SQL 数据源（PG + MySQL）
│   │   └── file.go                     # 📋 待实现
│   ├── transform/
│   │   ├── transform.go                # ✅ Transformer 接口 + Func 适配器
│   │   └── pyscript.go                 # ✅ Python 脚本转换
│   ├── load/
│   │   ├── load.go                     # ✅ Load 接口
│   │   ├── csv.go                      # ✅ CSV 写入
│   │   ├── stdout.go                   # ✅ 控制台输出
│   │   └── sql.go                      # 📋 待实现
│   └── config/                         # 📋 待实现
├── cmd/detl/main.go                    # 📋 待实现
├── conf/                               # 运行时配置
├── script/                             # ETL 业务脚本
├── main/                               # 当前入口
│   ├── main.go                         # ✅ 程序入口
│   ├── main_func.go                    # ✅ ETL 编排
│   ├── conf/dsn.json                   # 🔒 本地 DSN 配置
│   └── script/                         # 🔒 本地脚本
├── detl_test.go                        # ✅ 集成测试
├── output/                             # 输出文件
├── go.mod
├── AGENTS.md
└── README.md
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
| 6 | 修复配置系统 Bug | 修复 `GetConf("")` 重复调用 | 📋 |
| 7 | 实现 Load：SQL（UPSERT）| 数据库写入 | 📋 |
| 8 | 实现文件 Source/Load（CSV、JSON）| 文件数据源 | 📋 |
| 9 | 实现 YAML 任务定义 + CLI 编排 | 可用性提升 | 📋 |
| 10 | 完整的测试覆盖 | 保证质量 | 📋 |
