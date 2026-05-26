# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

## 简介

- **抽取**：MySQL, PostgreSQL, 文件, API 接口等
- **转换**：通过脚本表达式或脚本语言转换
- **载入**：输出结果到另一个数据库, 文件, API 接口等

---

## 当前状态

项目处于**早期开发阶段**，目前完成了：
- 基础配置系统（环境变量 + 命令行参数 + DSN 文件管理）: `conf` 目录存放基础配置文件，如：`dsn.json` 存放数据库连接配置。
- ETL业务脚本系统：`script/` 目录存放ETL业务脚本文件，包括：.sql, .json, .yml, .py 等。

**待实现**：MySQL 支持、文件/API 数据源、数据转换逻辑、数据写入目标端、完整的测试覆盖。支持ETL骨架文件和各种脚本的拆分组合。

## ETL业务脚本系统规范

- 存放目录：`script/`
- 命名规范：如果抽取数据源的脚本，要拆分为独立文件，请加前缀 `e_`。如： `e_*.sql`, `e_*.py`。ETL骨架文件为 `main_` 开头，如 `main_*.py`, `main_*.sql`, `main_*.json`, `main_*.yml`。

---

## 架构规划（草稿）

整体采用 **Pipeline 管道架构**：`Source → Transform → Load`，每条 ETL 任务作为一个 Job 编排执行。

```
                     ┌─────────────────────────────────┐
                     │           CLI / 配置层           │
                     │   (环境变量, 命令行, YAML/JSON 任务)   │
                     └──────────┬──────────────────────┘
                                │
                     ┌──────────▼──────────────────────┐
                     │          Pipeline Engine         │
                     │      (编排 Source → Transform → Load)│
                     └──────────┬──────────────────────┘
                                │
              ┌─────────────────┼──────────────────┐
              ▼                 ▼                   ▼
     ┌────────────────┐ ┌──────────────┐ ┌────────────────┐
     │     Source     │ │  Transform   │ │      Load      │
     │   (数据抽取)    │ │  (数据转换)   │ │   (数据载入)    │
     ├────────────────┤ ├──────────────┤ ├────────────────┤
     │ • SQL (PG/MySQL)│ │ • SQL 表达式 │ │ • SQL (PG/MySQL)│
     │ • CSV/JSON 文件 │ │ • Py 脚本   │ │ • CSV/JSON 文件 │
     │ • HTTP API     │ │ • 管道过滤    │ │ • HTTP API     │
     │ • 自定义脚本    │ │              │ │ • 控制台输出    │
     └────────────────┘ └──────────────┘ └────────────────┘
```

---

### 包结构规划

```
detl/
├── cmd/detl/main.go        # CLI 入口（轻量，只引 engine）
├── internal/
│   ├── engine/
│   │   ├── pipeline.go      # Pipeline 编排（串行/并行 Job）
│   │   └── job.go           # Job 定义（Source → Transform → Load）
│   ├── source/
│   │   ├── source.go        # Source 接口定义
│   │   ├── sql.go           # 从数据库抽取
│   │   └── file.go          # 从文件读取（CSV, JSON）
│   ├── transform/
│   │   ├── transform.go     # Transformer 接口定义
│   │   └── expr.go          # 脚本表达式转换
│   ├── load/
│   │   ├── load.go          # Load 接口定义
│   │   ├── sql.go           # 写入数据库（UPSERT）
│   │   ├── file.go          # 写入文件
│   │   └── stdout.go        # 控制台输出
│   └── config/
│       ├── config.go        # 全局配置
│       └── task.go          # 任务定义（YAML 描述 ETL 流程）
├── conf/                    # 运行时配置目录（dsn.json 等）
├── script/                  # ETL业务脚本存放目录。包括：.sql, .json, .yml, .py 等。
├── go.mod
├── AGENTS.md
└── README.md
```

---

### 核心接口设计（草案）

```go
// Source 数据源：返回一个迭代器，每次 yield 一行数据
type Source interface {
    Open() error
    Read() (map[string]any, bool) // (数据行, 是否还有更多)
    Close() error
}

// Transformer 数据转换：输入一行，输出零行或多行
type Transformer interface {
    Transform(map[string]any) ([]map[string]any, error)
}

// Load 数据目标：写入一行
type Load interface {
    Open() error
    Write(map[string]any) error
    Close() error
}
```

---

### 任务定义示例（YAML）

```yaml
# etl-task.yaml
name: "用户数据同步"
source:
  type: sql
  driver: postgres
  dsn: "user=... dbname=..."
  sql: "SELECT * FROM users WHERE updated_at > '{{.LastRun}}'"
transform:
  - type: expr
    script: |
      row.role = "user"
      row.created_at = time.now()
load:
  type: sql
  driver: mysql
  dsn: "user=... dbname=..."
  table: "users"
  mode: upsert
```

---

### 阶段实施计划

| 步骤 | 内容 | 说明 |
|---|---|---|
| 1 | 修复配置系统 Bug + 重构 | 修复 `GetConf("")` 重复调用问题，整理配置层 |
| 2 | 定义核心接口 + Pipeline Engine | 搭建 Source/Transform/Load 骨架 |
| 3 | 实现 SQL Source（MySQL + Postgres）| 增加 MySQL 驱动支持 |
| 4 | 实现 Load：SQL（UPSERT）、Stdout | 打通端到端数据流 |
| 5 | 实现文件 Source/Load（CSV、JSON）| 文件数据源支持 |
| 6 | 实现 Transform 表达式引擎 | ETL 核心价值 |
| 7 | 实现 YAML 任务定义 + CLI 编排 | 提升可用性 |
| 8 | 完整的测试覆盖 | 保证质量 |

---

### 待决策的技术选项

1. **配置文件格式**：YAML（推荐，可读性好）
2. **Transform数据转换处理逻辑**：Python脚本（足够灵活）
3. **数据源**：SQL（PostgreSQL + MySQL）， 文件（CSV, JSON）, Python脚本（获取HTTP API接口等灵活场景处理）
4. **数据目标**：SQL（PostgreSQL + MySQL）， 文件（CSV, JSON）, yml/json文件定义