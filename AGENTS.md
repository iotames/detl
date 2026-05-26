# AGENTS.md — detl（ETL 工具）

## 这是什么？

一个用 Go 写的简易 ETL（抽取、转换、载入）工具。从脚本文件中读取 SQL，通过可配置的 DSN 连接到 PostgreSQL（或其他数据库）执行查询，并处理结果集。

模块：`github.com/iotames/detl` · Go 1.24.1

## 项目结构

```
detl/
├── conf/conf.go       # 单例配置：DSN 管理、脚本目录
├── main/
│   ├── main.go        # CLI 入口：flag/env 解析、初始化配置
│   └── main_func.go   # 使用 easydb 的数据库查询逻辑
├── dsql.go            # 从脚本文件读取 SQL 文本
├── detl_test.go       # 占位测试
├── build.sh           # 未完成——只输出版本号（构建步骤被注释掉了）
├── go.mod / go.sum
└── .gitignore
```

## 依赖

| 导入路径 | 用途 |
|---|---|
| `github.com/iotames/easyconf` | 通过 `easyconf.NewConf()` 绑定环境变量 |
| `github.com/iotames/easydb` | 数据库抽象层（`EasyDb`、`DecodeInterface`、DSN 分组） |
| `github.com/iotames/miniutils` | `Md5`、`Mkdir`、`IsPathExists` |
| `github.com/lib/pq` | PostgreSQL 驱动 |

所有依赖都由 `iotames` 在 GitHub 上维护——没有标准第三方库。

## 核心架构

### 配置单例 (`conf.GetConf(confdir)`)
- 通过 `sync.Once` 创建一次。用 `miniutils.Mkdir` 确保目录存在（如果创建失败则 panic）。
- 持有 `dirPath` 和 `envMap[SCRIPT_DIR]`。
- **DSN 生命周期**（`conf.go:54-97`）：
  1. `InitDSN(driverName, dsn)` 如果 `confdir` 下没有 `dsn.json` 则创建一个（追加初始 DSN 并保存）。
  2. `SetActiveDSN(driverName, dsn)` 从 `dsn.json` 加载已有的 DSN 分组，激活经过 MD5 哈希的 DSN，并保存。
- **注意**：`main.go:33` 在设置 `ScriptDir` 后又调用了 `GetConf("")`——因为单例已存在，这是个空操作。

### CLI 初始化顺序（`main/main.go:22-36`）
1. `easyconf.NewConf()` 绑定环境变量：`CONF_DIR`、`SCRIPT_DIR`、`DB_DRIVER`、`ACTIVE_DSN`（都有默认值）
2. `conf.GetConf(ConfDir)` 创建单例
3. `cf.SetScriptDir(ScriptDir)` 设置脚本目录
4. `flag.Parse()` 解析 `-version` 参数
5. `cf.GetConf("")` —— 冗余调用，无效果（单例已存在）
6. `cf.InitDSN(DbDriver, ActiveDsn)` —— 如果 `dsn.json` 不存在则初始化 DSN

### 数据库查询流程（`main/main_func.go`）
1. 通过 `pkgdsn.GetDsnConf(nil)` 和 `dsnconf.GetDsnGroup(&dgp)` 从 `dsn.json` 加载 DSN 分组
2. 遍历 `dgp.DsnList` 中的每个 DSN：`sql.Open(driver, dsn)` → `easydb.NewEasyDbBySqlDB(db)` → `d.GetMany(sql, &datalist)`
3. 结果通过 `d.DecodeInterface(row)` 解码后 `log.Printf` 输出

### SQL 加载（`dsql.go`）
- `GetSqlText(cf, filename)` 通过 `os.ReadFile` 从 `{SCRIPT_DIR}/{filename}` 读取文件内容。

## 命令

| 操作 | 命令 |
|---|---|
| 构建 | `go build ./...` |
| 运行 | `go run ./main`（使用环境变量默认值） |
| 运行（自定义） | `go run ./main -CONF_DIR custom_conf -SCRIPT_DIR custom_script -DB_DRIVER postgres -ACTIVE_DSN "user=... dbname=..."` |
| 测试 | `go test ./...` |
| 测试（详细） | `go test -v ./...` |
| 代码检查 | `go vet ./...` |

没有 Makefile，没有任务运行器。`build.sh` 是个空壳——没什么用。

## 不易发现的坑

- **`main.go` 中 `GetConf("")` 调了两次**——第二次调用（`init` 第33行）传了空字符串，覆盖了第一次用 `ConfDir` 设置的 `dirPath`。这意味着 DSN 文件的 `confdir` 永远是 `""`（即当前工作目录）。如果想让配置目录用实际的 `CONF_DIR` 值，需要删掉第二个 `GetConf("")` 调用。
- **脚本目录必须存在**——`SetScriptDir` 会调用 `miniutils.Mkdir` 来创建。如果传一个不存在的目录，会自动创建（不会报错）。
- **空 confdir 会 panic**——`newConf` 会调用 `miniutils.Mkdir("")`，这会导致 panic。`GetConf("")` 之所以能工作是因为单例已经创建好了。
- **Exe 构建被忽略**——`main/.gitignore` 屏蔽了 `*.exe`。
- **`DecodeInterface`** 用于解码原始 map 结果——具体行为请查阅 `easydb` 文档。
- **DSN JSON 格式** 由 `easydb/dsn` 管理——文件在 `{confdir}/dsn.json`，存的是一个 `DsnGroup`，包含 `DsnList`（每项有 `Code`、`DriverName`、`Dsn`）。

## 编码风格与约定

- 配置结构体使用值接收器（安全）和指针接收器（`SetScriptDir`——修改状态）。
- 用 `miniutils.Mkdir` 而不是 `os.MkdirAll`。
- 用 `miniutils.Md5` 做 DSN 哈希（不是 `crypto/sha`）。
- 环境变量用 `easyconf`，CLI 参数用 `flag`（混合方式）。
- 错误处理：`init` 中用 `panic`，启动错误用 `log.Fatal`，运行时错误通过返回值传递。
- `defer d.CloseDb()` 的注释写着"很少用——会关闭整个连接池"。

## 测试

- `detl_test.go` 只是一个占位符（只是 `log` 输出 `---`）。没有任何真正的测试。
- 包名是 `detl`（不是 `detl_test`），所以测试内部可以访问所有函数和类型。
- 使用 `_ "github.com/lib/pq"` 这种空白导入风格来注册数据库驱动。
