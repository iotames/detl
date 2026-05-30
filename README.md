# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

- **抽取**：MySQL, PostgreSQL
- **转换**：内置 Go 函数 或 **Python 脚本**
- **载入**：CSV 文件、控制台输出、SQL 写入（insert/upsert）

两种运行模式：**传统模式**（环境变量驱动）和**任务模式**（YAML 任务文件驱动）。

---

## 安装

### 方式一：go install（推荐）

```bash
go install ./cmd/detl
```

安装到 `$GOPATH/bin/detl.exe`（若 `$GOPATH/bin` 在 PATH 中，可直接运行 `detl.exe`）。

### 方式二：本地编译

```bash
go build -o ./bin/detl.exe ./cmd/detl
./bin/detl.exe -task ...
```

### 运行测试

```bash
# PG 集成测试（需要 PG localhost:5432, user=postgres, password=postgres）
go test -v -run TestPipeline_PG_to_CSV

# MySQL 集成测试（需要 MySQL localhost:3306, root:root）
go test -v -run TestPipeline_MySQL_to_CSV

# 全部测试
go test -v ./...

# 代码检查
go vet ./...
```

---

## Hello ETL：MySQL → 清洗转换 → CSV

完整的 ETL 流程，无需改一行源代码。

### 流程概览

```
MySQL(detl_test.detl_test_users)  ← conf/dsn.json
    │ 抽取：e_detl_users.sql       ← script/
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

`conf/dsn.json` 支持多数据源，每个连接有唯一的 `Name` 供 ETL 任务引用：

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

`script/e_detl_users.sql`：

```sql
SELECT id, first_name, last_name, email, age, created_at
FROM detl_test_users
ORDER BY id
```

### 3. 安装并运行

```bash
# 安装到 $GOPATH/bin
go install ./cmd/detl

# Windows（如果 $GOPATH/bin 在 PATH 中）
CONF_DIR=conf SCRIPT_DIR=script DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql detl.exe

# Linux/Mac（如果 $GOPATH/bin 在 PATH 中）
CONF_DIR=conf SCRIPT_DIR=script DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql detl

# 或本地编译后直接运行
go build -o ./bin/detl.exe ./cmd/detl
CONF_DIR=conf SCRIPT_DIR=script DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql ./bin/detl.exe
```

### 4. 切换输出方式

```bash
# 控制台输出
CONF_DIR=conf SCRIPT_DIR=script DB_DRIVER=mysql LOAD_TYPE=stdout detl.exe

# 透传原始数据（不转换）
CONF_DIR=conf SCRIPT_DIR=script DB_DRIVER=mysql TRANSFORM_MODE=none detl.exe
```

### 5. 任务模式运行

```bash
detl.exe -task task/user_etl.yaml
```

任务文件 `task/user_etl.yaml`：

```yaml
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

### 示例：`script/t_users.py`

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
TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py detl.exe
```

---

## 架构

Pipeline 管道模式：**Source → Transform → Load**

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
     │                │ │ • none(透传) │ │ • SQL insert   │
     └────────────────┘ └──────────────┘ │   / upsert     │
                                         └────────────────┘
```

### 包结构

```
detl/
├── cmd/detl/                          # 程序入口（package main）
│   ├── main.go                        # CLI 入口：flag/env 解析
│   ├── main_func.go                   # ETL 编排（传统 + 任务模式）
│   ├── main_func_test.go              # 工具函数测试
│   ├── conf/                          # 示例配置（用户可按需修改）
│   │   ├── dsn.json                   # 数据源连接配置
│   │   └── system.yaml                # 系统默认配置
│   ├── script/                        # 示例 ETL 脚本（SQL + Python）
│   └── task/                          # 示例 ETL 任务 YAML
├── conf/conf.go                       # 配置单例 + DSN 管理 + system.yaml
├── internal/
│   ├── engine/pipeline.go             # Pipeline 编排
│   ├── source/                        # 数据抽取（SQL: PG + MySQL）
│   ├── transform/                     # 数据转换（Func + Python）
│   ├── load/                          # 数据载入（CSV + Stdout + SQL）
│   └── task/task.go                   # 任务/作业 YAML 定义 + 解析
├── output/                            # 输出文件
├── SKILL.md                           # AI Agent 技能文档
├── CLAUDE.md                          # AI 辅助开发指南
└── README.md
```

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

### 环境变量大全

| 模块 | 变量 | 默认值 | 说明 |
|------|------|--------|------|
| **Source** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json + system.yaml） |
| | `SCRIPT_DIR` | `script` | ETL 脚本目录 |
| | `DB_DRIVER` | `postgres` | 数据库驱动（`postgres` / `mysql`） |
| | `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| | `ACTIVE_DSN` | （PG 默认值） | 默认 DSN 连接字符串 |
| **Transform** | `TRANSFORM_MODE` | `builtin` | 转换模式（`builtin` / `python` / `none`） |
| | `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本（`python` 模式时生效） |
| **Load** | `LOAD_TYPE` | `csv` | 输出类型（`csv` / `stdout` / `sql`） |
| | `OUTPUT_DIR` | `output` | 输出目录 |
| | `OUTPUT_FILE` | `etl_output.csv` | 输出文件名（或 SQL 目标表名） |
| | `OUTPUT_COLUMNS` | 默认列 | CSV 列名（逗号分隔） |
| **Task** | `TASK_DIR` | `task` | ETL 任务 YAML 目录 |
| | `-task` flag | — | 指定任务文件，启用任务模式 |

---

## 实现状态

### 已实现

| 模块 | 内容 | 测试 |
|------|------|------|
| **Source** | SQL 数据源：PostgreSQL + MySQL | 集成测试通过 |
| **Transform** | 内置 Go 函数转换 / Python 脚本转换 | 集成测试通过 |
| **Load** | CSV 写入 / Stdout 控制台输出 / SQL 写入（insert/upsert） | 集成测试通过 |
| **Engine** | Pipeline 编排（Source → Transform → Load） | 集成测试通过 |
| **配置** | 环境变量 + DSN 文件管理 + system.yaml | 基础可用 |
| **YAML 任务** | `kind: 转换` + 按连接名引用 DSN | 编译通过 |
| **作业设计** | `kind: 作业` 结构预留 | 设计完成 |

### 待实现

- 文件 Source（CSV/JSON）
- 作业执行引擎
- HTTP API 数据源

---

## 注意事项

- `cmd/detl/conf/` 和 `cmd/detl/script/` 是示例配置，用户可复制到工作目录后直接修改
- 任务模式与传统模式互斥
- `builtin` 转换为硬编码示例（针对 `detl_test_users` 表），非通用实现
- 作业（`kind: 作业`）执行引擎尚未实现
- SQL Load 自动建表时所有列使用 TEXT 类型
