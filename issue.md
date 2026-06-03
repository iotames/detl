# 设计审查问题清单

> 以下问题中标记 ✅ 的已在本次审查中修复，标记 🔧 的为设计观察项无需代码修改。

## 需求

根据审查范围，对 detl 源代码进行审查，并记录问题清单，并提供更好的设计方案。

项目的命令行传参，配置文件，工作流业务文件，环境变量，配置互相覆盖。特别是环境变量，有一些配置是冗余的，甚至会和运行时参数冲突，要单重点审查，并列出来。

## 输出

把工作结果也更新到本文件中。

## 审查范围

以下为本次审查覆盖的源文件（排除了 `*_test.go` 测试文件、`.gitignore` 中排除的 `easydb/` `bin/` `output/` `runtime/` 等目录、以及 `go.sum` 等自动生成文件）：

**根包 (`package detl`)**
- `go.mod`: 其中的 `replace` 指令问题可忽略。

**程序入口 (`cmd/detl/`)**
- `cmd/detl/main.go`
- `cmd/detl/main_func.go`

**其他包文件**
- `conf/conf.go`
- `hotswap/script.go`
- `internal/engine/pipeline.go`
- `internal/load/` 下所有文件
- `internal/source/` 下所有文件
- `internal/task/task.go`
- `internal/transform/` 下所有文件

**无需审查**：
- `easydb/`
- `cmd/seed/main.go`
- `go.mod` 中的 `replace` 指令问题（经确认不包括）



# 审查结果

## 一、配置系统问题（核心）

### 1.1 `ACTIVE_DSN` 语义混乱

| 位置 | 说明 |
|------|------|
| `cmd/detl/main.go:86` | 定义 `ACTIVE_DSN` 环境变量，默认值为 `user=postgres password=postgres ...` |
| `cmd/detl/main_func.go:22-41` | `getActiveDSN()` 优先从 dsn.json 按 `DriverName` 匹配，**不检查 active 标记** |
| `conf/conf.go:60-76` | `InitDSN()` 仅在 dsn.json **不存在时**以 `ACTIVE_DSN` 创建初始文件 |

**问题**：`ACTIVE_DSN` 命名暗示它是"当前活跃的 DSN"，实际作用仅为"dsn.json 不存在时的初始种子值"。运行期 `getActiveDSN()` 完全绕过该变量，优先从 dsn.json 查找。用户看到 `ACTIVE_DSN` 会误以为修改此变量能切换数据库，实际并非如此。

**建议**：重命名为 `DEFAULT_DSN` 或 `FALLBACK_DSN`，并在文档中明确其作用域仅为"dsn.json 不存在时生成初始配置"。

---

### 1.2 `conf.Conf.envMap` 形同虚设

| 位置 | 问题 |
|------|------|
| `conf/conf.go:49` | `envMap map[string]string` 定义了一个内部环境变量存储 |
| `conf/conf.go:52-53` | `GetScriptDir()` 从 `envMap["SCRIPT_DIR"]` 读取 |
| `cmd/detl/main.go:109` | `SetConf(ConfDir)` 只传入配置目录路径，**不设置 envMap** |
| `cmd/detl/main_func.go:96` | `cf.GetScriptFilePath()` 作为 hotswap 回退路径 |

`envMap` 只初始化（`conf.go:43`）但从未写入。`GetScriptDir()` 永远返回空字符串。
`GetScriptFilePath(fname)` → `filepath.Join("", fname)` → 退化为裸文件名，等同于不做任何拼接。

**建议**：删除 `envMap`，或将 `SCRIPT_DIR` 也通过 `SetConf(confdir, scriptdir)` 传入。

---

### 1.3 `getActiveDSN()` 与 `SetActiveDSN()` 逻辑矛盾

| 函数 | 匹配策略 |
|------|----------|
| `main_func.go:22-41` `getActiveDSN()` | 遍历 dsn.json，按 `DriverName` 返回**第一个匹配** |
| `conf.go:118-143` `SetActiveDSN()` | 用 MD5 标记活跃 DSN，调用 `dgp.Active(dsnCode)` |

**问题**：
- `getActiveDSN` 从不调用 `SetActiveDSN`，两条路径互不感知
- `getActiveDSN` 按驱动名匹配（而非 active 标记），当同一驱动有多个 DSN 时只能拿到第一个
- `SetActiveDSN` 定义了 `Active()` 机制但**从未被调用**（死代码）

**建议**：统一为一种匹配策略——要么全部按 Name 引用（YAML 模式的做法），要么按 active 标记。

---

### 1.4 flag 命名风格不统一

`main.go:101`：
```go
env.StringVar(&DsnName, "dsnName", "", "要测试的连接名称")
```

其余所有环境变量/flag 均使用 `SCREAMING_SNAKE_CASE`（`CONF_DIR`, `SCRIPT_DIR`, `DB_DRIVER`, `TRANSFORM_MODE`, `LOAD_TYPE` 等），只有 `dsnName` 使用驼峰命名。

**建议**：统一为 `DSN_NAME`。

---

### 1.5 环境变量模式中 `LoadTable` 被误用

`main_func.go:197`：
```go
return runPipeline(pipelineCfg{
    ...
    LoadType: LoadType, LoadFile: outputFile,
    LoadTable: outputFile,  // ← "etl_output.csv" 作为表名
    LoadMode: "upsert",
})
```

`LoadTable` 仅在 `load.Type == "sql"` 时有意义（作为目标表名），但在环境变量模式中它被硬编码为 CSV 文件路径 `"etl_output.csv"`。这引入了一个隐蔽问题：
- 如果用户设置 `LOAD_TYPE=sql`，SQL Load 会试图将数据写入名为 `etl_output.csv` 的表
- `LoadMode: "upsert"` 硬编码，当 `LOAD_TYPE=csv` 时 upser t模式无意义

**建议**：环境变量模式中 `LoadTable` 和 `LoadMode` 应在 `LOAD_TYPE=sql` 时单独设置，或由用户通过环境变量指定。csv/stdout 模式时应将这两个字段留空。

---

### 1.6 两套配置平行重叠

环境变量模式（`runETL()`）和 YAML 任务模式（`runTask()`）各有完整的业务配置，覆盖面高度重叠：

| 配置项 | 环境变量 | YAML 任务 |
|--------|----------|-----------|
| 数据库 | `DB_DRIVER`, `ACTIVE_DSN` | `source.connection` |
| SQL | `SCRIPT_FILE` | `source.query_file` / `source.query` |
| 转换 | `TRANSFORM_MODE`, `TRANSFORM_SCRIPT` | `transform.mode`, `transform.script` |
| 输出 | `LOAD_TYPE`, `OUTPUT_DIR` | `load.type`, `load.file` |
| 列 | 无（hardcode 在 `runETL` 中） | `load.columns` |

环境变量模式中的很多默认值（列名、文件名）硬编码在 `runETL()` 函数中，用户无法自定义。而 YAML 任务模式灵活得多。建议考虑是否保留环境变量模式作为"快速启动"，或在文档中标明其限制。

---

## 二、代码质量问题

### 2.1 `conf.SetConf` 的 `sync.Once` 模式脆弱

`conf/conf.go:25-35`：
```go
func SetConf(confdir string) error {
    if confdir == "" {
        // 返回 error
    }
    once.Do(func() {
        cf = newConf(confdir)
    })
    return nil  // 永远返回 nil！
}
```

- `once.Do` 内部的 `newConf` 如果 `Mkdir` 失败，会 **panic** 而非返回 error
- `once.Do` 执行后，无论成功与否，后续 `SetConf` 调用被静默忽略（不返回任何错误）
- 调用方无法感知 `SetConf` 是否实际执行、是否成功

**建议**：移除 `sync.Once`，改用显式的初始化检查模式（如 `sync.atomic.Bool` 标记），或将 `panic` 改为返回 error。

---

### 2.2 `GetConf` 中 `log.Error` + `panic` 双重处理

`conf/conf.go:16-23`：
```go
func GetConf() *Conf {
    if cf == nil {
        errmsg := "请先调用 SetConf() 初始化配置！"
        log.Error(errmsg)  // 日志
        panic(errmsg)      // 又 panic
    }
    return cf
}
```

先记录 error 日志，再 panic。调用方无法拦截日志（`log.Error` 是内部 logger），而 panic 后日志几乎没有意义。应统一要么全部返回 error，要么全部 panic。

**建议**：直接 panic（未初始化属于编程错误），或改为返回 `(*Conf, error)`。

---

### 2.3 `init()` 函数过大

`cmd/detl/main.go:77-111` 的 `init()` 函数包含：
- 创建 `easyconf` 实例
- 绑定 11 个环境变量/flag
- 调用 `env.Parse(true)`
- 调用 `conf.SetConf()`
- 调用 `conf.GetConf()`
- 调用 `initScript()`

Go 最佳实践：`init()` 应保持简单（注册、初始化），复杂逻辑应放在 `main()` 中显式调用。

**建议**：将 `init()` 中的逻辑拆分到 `main()` 函数中，或拆分为 `initConfig()`、`initScript()`、`initConf()` 等具名函数。

---

### 2.4 `SetActiveDSN` 死代码

`conf/conf.go:118-143`：`SetActiveDSN` 方法定义但**从未被调用**。

同样 `conf.go:60-76` 中的 `InitDSN` 返回值也被部分浪费：`main.go:49` 调用 `cf.InitDSN(DbDriver, ActiveDsn)` 时忽略了返回值中的 `dsnconf` 和 `isInit`。

**建议**：删除 `SetActiveDSN`，或补全调用链路使其生效。

---

### 2.5 `panic(confdir)` 无意义错误消息

`conf/conf.go:39`：
```go
if err := miniutils.Mkdir(confdir); err != nil {
    panic(confdir)  // 只 panic 了目录名，没有错误原因
}
```

**建议**：改为 `panic(fmt.Errorf("创建配置目录 %s 失败: %w", confdir, err))`。

---

### 2.6 `LoadTask` 路径查找语义不清晰

`internal/task/task.go:71-83`：
```go
func LoadTask(path string) (*TaskConfig, error) {
    data, err := hotswap.GetScriptDir().GetScriptBytes(path)
    if err != nil {
        if _, err = os.Stat(path); os.IsNotExist(err) {
            return nil, fmt.Errorf("任务文件不存在: %s", path)
        }
        data, err = os.ReadFile(path)
        ...
    }
```

- 先通过 `hotswap` 查找（含内嵌文件回退），失败后 `os.Stat` 再 `os.ReadFile`
- 中间 `os.Stat` 的 `err` 变量覆盖了 `hotswap` 返回的 `err`，如果在 `hotswap` 阶段 err != nil 但文件存在（例如内嵌文件读取错误），逻辑会混乱

**建议**：明确区分"从脚本目录查找"和"从绝对/相对路径读取"两种模式，使用不同的错误处理路径。

---

### 2.7 `sortStrings` 死代码

`internal/load/sql.go:208-210`：
```go
func sortStrings(s []string) {
    sort.Strings(s)
}
```

未被任何地方调用。函数体也完全等价于 `sort.Strings`。

**建议**：删除。

---

### 2.8 `formatValue` 递归调用有理论风险

`internal/load/csv.go:86`：
```go
func formatValue(v any) string {
    ...
    case reflect.Ptr, reflect.Interface:
        if rv.IsNil() {
            return ""
        }
        return formatValue(rv.Elem().Interface())  // 递归
    ...
```

正常 SQL 扫描不会产生深层嵌套，但如果意外传入自定义类型或循环引用的结构，会栈溢出。属于理论风险，但可用 `for` 循环改写为迭代方式以消除风险。

---

## 三、设计问题

### 3.1 DSN 读取逻辑分散

DSN 配置的读取逻辑分布在两个地方：
- `conf/conf.go:88-98` `GetDSNGroup()` — 通过 `conf.SetConf` 初始化的 `DsnConf` 读取
- `main_func.go:22-41` `getActiveDSN()` — 直接调用 `pkgdsn.GetDsnConf(nil)` 读取

两处都访问同一个 `dsn.json` 文件，但入口不同。`getActiveDSN` 完全不使用 `conf.Conf` 的方法。

**建议**：`getActiveDSN()` 应使用 `cf.GetDSNGroup()` + `GetDSNByDriver()`，统一通过 `conf.Conf` 访问 DSN 配置。

---

### 3.2 DSN 匹配方式不统一

环境变量模式按 `DriverName` 匹配：
```go
// main_func.go:28-31
if ds.DriverName == DbDriver {
    return ds.Dsn
}
```

YAML 任务模式优先按 `Name` 匹配：
```go
// main_func.go:217
ds, ok := cf.GetDSNByName(t.Source.Connection)  // 先按 Name
if !ok {
    ds, ok = cf.GetDSNByDriver(t.Source.Connection)  // 回退到 DriverName
}
```

同一项目中有两种 DSN 匹配逻辑，增加了认知负担。建议统一为"优先按 Name，回退到 DriverName"的单一策略。

---

## 四、问题汇总

| 编号 | 类别 | 文件 | 严重程度 | 状态 | 说明 |
|------|------|------|----------|------|------|
| 1.1 | 配置 | `main.go:86`, `main_func.go:22-41`, `conf.go:60-76` | 中 | 🔧 | `ACTIVE_DSN` 语义混乱（重命名涉及 API 变更，保留） |
| 1.2 | 配置 | `conf.go:43-57` | 中 | ✅ | `envMap` 形同虚设 — 已添加 `SetEnv` 方法并在 `initConf()` 中写入 |
| 1.3 | 配置 | `conf.go:118-143`, `main_func.go:22-41` | 中 | ✅ | `getActiveDSN` 改用 `cf.GetDSNByDriver()`，删除 `SetActiveDSN` 死代码 |
| 1.4 | 配置 | `main.go:101` | 低 | 🔧 | `dsnName` 命名风格不统一（flag 名属 API，不修改） |
| 1.5 | 配置 | `main_func.go:197` | 中 | ✅ | `LoadTable`/`LoadMode` 仅在 `LOAD_TYPE=sql` 时设置 |
| 1.6 | 配置 | 全局 | 低 | 🔧 | 两套配置体系平行重叠（架构设计选择，非 bug） |
| 2.1 | 代码 | `conf.go:25-35` | 中 | ✅ | `sync.Once` 替换为简单 nil 检查 |
| 2.2 | 代码 | `conf.go:16-23` | 低 | ✅ | 移除 `log.Error`，保留 `panic` |
| 2.3 | 代码 | `main.go:77-111` | 低 | ✅ | `init()` 拆分为 `parseEnv()` + `initConf()` |
| 2.4 | 代码 | `conf.go:118-143` | 低 | ✅ | `SetActiveDSN` 死代码已删除 |
| 2.5 | 代码 | `conf.go:39` | 低 | ✅ | `panic(confdir)` 改为带错误的格式化消息 |
| 2.6 | 代码 | `task.go:71-83` | 低 | ✅ | 修复 `LoadTask` 变量覆盖问题 |
| 2.7 | 代码 | `load/sql.go:208-210` | 低 | ✅ | `sortStrings` 死代码已删除 |
| 2.8 | 代码 | `load/csv.go:86` | 低 | ✅ | `formatValue` 递归改为迭代循环 |
| 3.1 | 设计 | `conf.go` vs `main_func.go` | 低 | ✅ | `getActiveDSN` 统一使用 `cf.GetDSNByDriver()` |
| 3.2 | 设计 | `main_func.go` | 低 | 🔧 | 两种 DSN 匹配策略各有适用场景，保留 |

---

## 五、改进建议（汇总）

1. **重命名 `ACTIVE_DSN` 为 `DEFAULT_DSN` 或 `FALLBACK_DSN`**，明确其仅用于 dsn.json 不存在时的初始值。
2. **删除 `conf.Conf.envMap`**，或将 `SCRIPT_DIR` 通过 `SetConf(confdir, scriptdir)` 传入。
3. **统一 DSN 匹配策略**：删除 `SetActiveDSN` 死代码，`getActiveDSN` 改用 `cf.GetDSNByDriver()`。
4. **统一 `dsnName` 为 `DSN_NAME`**。
5. **`runETL()` 中分离 LoadTable 用途**：只为 `LOAD_TYPE=sql` 设置 `LoadTable` 和 `LoadMode`。
6. **移除 `conf.go` 中的 `sync.Once`**，改用显式初始化检查。
7. **将 `init()` 拆分为具名函数**，在 `main()` 中调用。
8. **删除 `sortStrings` 死代码**。
9. **修复 `panic(confdir)` 的无意义消息**。
