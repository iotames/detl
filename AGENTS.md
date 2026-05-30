# AGENTS.md — detl（ETL 工具）

## 这是什么？

一个用 Go 写的简易 ETL（抽取、转换、载入）工具。采用 Pipeline 管道模式：**Source → Transform → Load**。

模块：`github.com/iotames/detl` · Go 1.24.1

## 项目结构

```
detl/
├── cmd/detl/                   # 程序入口（package main）
│   ├── main.go                 # CLI 入口：flag/env 解析、初始化配置
│   ├── main_func.go            # ETL 编排：构建 Source/Transform/Load
│   └── main_func_test.go       # 工具函数测试
├── conf/
│   └── conf.go                 # 配置单例：DSN 管理、脚本目录、system.yaml
├── internal/
│   ├── engine/
│   │   └── pipeline.go         # Pipeline.Run(): Source → Transform → Load
│   ├── source/
│   │   ├── source.go           # Source 接口
│   │   └── sql.go              # SQL 数据源（PG + MySQL）
│   ├── transform/
│   │   ├── transform.go        # Transformer 接口 + Func 适配器
│   │   └── pyscript.go         # Python 脚本转换（常驻子进程 + JSON 行）
│   ├── load/
│   │   ├── load.go             # Load 接口
│   │   ├── csv.go              # CSV 写入
│   │   ├── sql.go              # SQL 写入（insert / upsert）
│   │   └── stdout.go           # 控制台输出
│   └── task/
│       └── task.go             # 任务/作业 YAML 定义 + 解析
├── dsql.go                     # GetSqlText: 从 SCRIPT_DIR 读取 SQL 文件
├── detl_test.go                # 集成测试
├── SKILL.md                    # AI Agent 技能文档（面向终端用户）
├── README.md                   # 项目首页（面向开发者 + 用户）
└── CLAUDE.md                   # AI 辅助开发指南
```

## 依赖

| 导入路径 | 用途 |
|---|---|
| `github.com/iotames/easyconf` | 环境变量绑定 |
| `github.com/iotames/easydb/easydb` | DB 封装（内联到 `easydb/` 目录） |
| `github.com/iotames/easydb/dsn` | DSN 分组管理 |
| `github.com/iotames/miniutils` | 文件/MD5 工具（Md5、Mkdir、IsPathExists） |
| `github.com/lib/pq` | PostgreSQL 驱动 |
| `github.com/go-sql-driver/mysql` | MySQL 驱动 |

所有非标准依赖都由 `iotames` 在 GitHub 上维护。

## 核心架构

### 配置单例 (`conf.GetConf(confdir)`)

- 通过 `sync.Once` 创建一次。用 `miniutils.Mkdir` 确保目录存在。
- 持有 `dirPath` 和 `envMap[SCRIPT_DIR]`。
- 支持 `system.yaml` 可选覆盖，环境变量优先级高于文件。
- **DSN 生命周期**：`InitDSN(driverName, dsn)` 在 `dsn.json` 不存在时创建；`SetActiveDSN()` 激活分组中的 DSN。

### CLI 入口（`cmd/detl/main.go:init()`）

1. `easyconf.NewConf()` 绑定环境变量：`CONF_DIR`、`SCRIPT_DIR`、`DB_DRIVER`、`ACTIVE_DSN` 等
2. `conf.GetConf(ConfDir)` 创建单例
3. `cf.SetScriptDir(ScriptDir)` 设置脚本目录
4. `flag.Parse()` 解析 `-version` / `-task` 参数
5. `main()` 中根据是否指定 `-task` 分流：`runETLFromTask()` 或 `runETL()`

**注意**：`init()` 中 `GetConf("")` 被调用了两次——第二次传空字符串但不影响单例。

### Pipeline 流程（`internal/engine/pipeline.go`）

```go
p := engine.New(src, tf, ld)
p.Run()
```

`Run()` 调用链：`source.Open()` → 循环 `source.Read()` → `transformer.Transform()` → `load.Write()`。转换失败跳过该行，写入失败中止。

### ETL 任务 YAML（`internal/task/task.go`）

任务文件放在 `TASK_DIR`（默认 `task`）目录下。支持两种 `kind`：
- `转换`：单个 ETL 流程（Source → Transform → Load）
- `作业`：多个转换的集合（预留，尚未实现执行引擎）

DSN 连接按 `Name` 字段引用，旧版 dsn.json（无 Name）自动回退为按驱动名匹配。

### Python 脚本转换（`internal/transform/pyscript.go`）

启动常驻 Python 子进程，通过 stdin/stdout 传递 JSON 行：
- 每行输入一个 JSON 对象（原始数据行）
- 每行输出一个 JSON 对象（转换后数据行）
- 返回 `null` 或空行跳过该行
- 输出后必须 `flush=True`
- 优先使用 `"python"` 命令，回退 `"python3"`

### SQL Load（`internal/load/sql.go`）

支持 MySQL 和 PostgreSQL：
- MySQL 使用 `?` 占位符和反引号标识符
- PostgreSQL 使用 `$N` 占位符和双引号标识符
- Mode: `insert` / `upsert`（MySQL: `ON DUPLICATE KEY UPDATE`, PG: `ON CONFLICT DO UPDATE`）
- 自动建表（全部列 TEXT 类型）
- 批量写入（默认 50 行一批）

### 编译与安装

```bash
# 安装到 $GOPATH/bin（推荐，不污染项目根目录）
go install ./cmd/detl

# 本地编译
go build -o ./bin/detl.exe ./cmd/detl
```

## 命令

| 操作 | 命令 |
|---|---|
| 安装 | `go install ./cmd/detl` |
| 本地编译 | `go build -o ./bin/detl.exe ./cmd/detl` |
| 运行（开发） | `go run ./cmd/detl` |
| 运行（DSN/任务模式） | `go run ./cmd/detl -task task/user_etl.yaml` |
| 测试 | `go test -v -run TestPipeline_MySQL_to_CSV` |
| 全部测试 | `go test -v ./...` |
| 代码检查 | `go vet ./...` |

## 不易发现的坑

- **`GetConf("") 调了两次**——`init()` 第 67 行第二次传空字符串，但 `sync.Once` 确保只有第一次生效。
- **脚本目录必须存在**——`SetScriptDir` 会调用 `Mkdir` 自动创建。
- **builtin 转换非通用**——硬编码为 `cmd/detl/main_func.go:80-135` 的 `detl_test_users` 表结构。
- **Python 命令查找**——Windows 上 `"python"` 可能解析到 Microsoft Store 存根。
- **模块路径**——`github.com/iotames/detl`，根包为 `package detl`（不是 `package main`）。
- **`cmd/detl/conf/` 和 `cmd/detl/script/`**——受 `.gitignore` 保护，是示例配置。用户可自由复制修改。
- **`easydb/` 目录**——是 `github.com/iotames/easydb` 的内联副本，通过 `go.mod` 的 `replace` 指令使用本地版本。
- **DSN 去重**——使用 `miniutils.Md5` 做 MD5 哈希，非加密用途。
- **作业未实现**——`kind: 作业` 运行时仅列出子任务后返回错误。
