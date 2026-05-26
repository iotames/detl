# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

## 简介

- **抽取**：MySQL, PostgreSQL, 文件, API 接口等
- **转换**：通过脚本表达式或脚本语言转换
- **载入**：输出结果到另一个数据库, 文件, API 接口等

---

## 当前状态

项目处于**开发阶段**，已完成：

### ✅ 已实现
| 模块 | 内容 | 测试 |
|---|---|---|
| **Source** | SQL 数据源：PostgreSQL + MySQL | ✅ 集成测试通过 |
| **Transform** | Func 适配器（Go 函数转换） | ✅ 集成测试通过 |
| **Load** | CSV 文件写入 | ✅ 集成测试通过 |
| **Engine** | Pipeline 编排（Source → Transform → Load） | ✅ 集成测试通过 |
| **配置** | 环境变量 + 命令行 + DSN 文件管理 | 基础可用 |

### 📋 待实现
- 文件 Source（CSV/JSON）
- Load：SQL 写入（UPSERT）
- Load：控制台输出（Stdout）
- Transform：Python 脚本支持
- CLI 命令行工具（`-task`, `-dir`）
- YAML/JSON 任务定义
- HTTP API 数据源

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

### 包结构

```
detl/
├── internal/                           # ✅ 核心库
│   ├── engine/
│   │   └── pipeline.go                 # ✅ Pipeline 编排
│   ├── source/
│   │   ├── source.go                   # ✅ Source 接口
│   │   ├── sql.go                      # ✅ SQL 数据源（PG + MySQL）
│   │   └── file.go                     # 📋 待实现：文件读取
│   ├── transform/
│   │   └── transform.go                # ✅ Transformer 接口 + Func 适配器
│   ├── load/
│   │   ├── load.go                     # ✅ Load 接口
│   │   ├── csv.go                      # ✅ CSV 文件写入
│   │   ├── sql.go                      # 📋 待实现：数据库写入
│   │   └── stdout.go                   # 📋 待实现：控制台输出
│   └── config/                         # 📋 待实现：任务配置
├── cmd/detl/main.go                    # 📋 待实现：CLI 入口
├── conf/                               # 运行时配置目录（dsn.json 等）
├── script/                             # ETL业务脚本存放目录
├── detl_test.go                        # ✅ 集成测试
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

| 步骤 | 内容 | 说明 | 状态 |
|---|---|---|---|
| 1 | 定义核心接口 + Pipeline Engine | Source/Transform/Load 骨架 | ✅ |
| 2 | 实现 SQL Source（MySQL + Postgres）| 数据库数据抽取 | ✅ |
| 3 | 实现 Load：CSV 文件写入 | 文件输出 | ✅ |
| 4 | 实现 Transform Func 适配器 | Go 函数转换 | ✅ |
| 5 | 修复配置系统 Bug + 重构 | 修复 `GetConf("")` 重复调用 | 📋 |
| 6 | 实现 Load：SQL（UPSERT）、Stdout | 数据库写入 + 控制台 | 📋 |
| 7 | 实现文件 Source/Load（CSV、JSON）| 文件数据源 | 📋 |
| 8 | 实现 YAML 任务定义 + CLI 编排 | 可用性提升 | 📋 |
| 9 | 实现 Transform Python 脚本引擎 | ETL 核心价值 | 📋 |
| 10 | 完整的测试覆盖 | 保证质量 | 📋 |

---

### 待决策的技术选项

1. **配置文件格式**：YAML（推荐，可读性好）
2. **Transform数据转换处理逻辑**：Python脚本（足够灵活）
3. **数据源**：SQL（PostgreSQL + MySQL）， 文件（CSV, JSON）, Python脚本（获取HTTP API接口等灵活场景处理）
4. **数据目标**：SQL（PostgreSQL + MySQL）， 文件（CSV, JSON）, yml/json文件定义
