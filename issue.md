# 设计审查问题清单

## 需求

根据审查范围，对 detl 源代码进行审查，并记录问题清单，并提供更好的设计方案。

项目的命令行传参，配置文件，工作流业务文件，环境变量，这几个运行配置互相覆盖，特别是环境变量，有一些配置是冗余的，甚至会和运行时参数冲突。这个环境变量问题，要单重点审查，并列出来。

## 输出

把工作结果也更新到本文件中。

## 审查范围

以下为本次审查覆盖的源文件（排除了 `*_test.go` 测试文件、`.gitignore` 中排除的 `easydb/` `bin/` `output/` `runtime/` 等目录、以及 `go.sum` 等自动生成文件）：

**根包 (`package detl`)**
- `dsql.go`
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

---

## 问题清单

> 状态标记说明：✅ 已解决／🔨 部分解决／❌ 未解决／🔄 策略变更（已按其他方向处理）

### 1. `init()` / `main()` 中 `GetConf` 调用混乱 🔨

- **已处理**：`GetConf()` 不再接受 `confdir` 参数，`main()` 中不再传入 `""`
- **已处理**：新增 `SetConf(ConfDir)` 显式初始化，`GetConf()` 只返回单例
- **遗留问题**：`SetConf` 内有 `if cf != nil` 的守卫检查，与 `sync.Once` 功能冗余；`GetConf()` 若未先调 `SetConf` 会 panic（虽暴露了编程错误，但比之前更脆弱）

### 2. `builtinTransform()` 硬编码特定业务表结构 ❌

- 未处理

### 3. `runETL` / `runETLFromTask` 重度重复 ❌

- 未处理

### 4. `log.Fatalf` 杀进程 ❌

- 未处理

### 5. `runJob` 递归无环检测 ❌

- 未处理

### 6. DSN 回退语义混淆 ❌

- 未处理

### 7. 手写 `toLower` 破坏 UTF-8 ❌

- 未处理

### 8. 任务模式下 `DbDriver` 污染 `source` 字段 ❌

- 未处理

### 9. `buildTransform` 返回 nil 的边缘情况 ❌

- 未处理

### 10. `resolveTaskPath` 的 fallback 与文档不一致 🔨

- **已处理**：`runETLFromTask` 中去掉了 `resolveTaskPath` 调用
- **遗留问题**：`resolveTaskPath` 函数本身仍存在（仅加了 `TODO 删除此函数` 注释），未实际删除

### 11. 包级变量隐式耦合 ❌

- 未处理

### 12. `detl.GetSqlText` 是无意义的包装 ❌

- 未处理

### 13. 根包 `package detl` 职责模糊 ❌

- 未处理

### 14. `sortStrings` 手写冒泡排序代替 `sort.Strings` ❌

- 未处理

### 15. `stdoutLoad` 的 `csv.Writer` 未使用，数据行手写 CSV 拼接无转义 ❌

- 未处理

### 16. `hotswap/script.go` 是死代码 🔄

- **策略变更**：`hotswap` 已被重新启用——`main.go` 中调用了 `initScript()` → `hotswap.NewScriptDir()` / `hotswap.SetScriptDir()`，`task.go` 中 `LoadTask()` 通过 `hotswap.GetScriptDir().GetScriptBytes(path)` 读取任务文件
- 新增 `cmd/detl/script/script.go` 提供 `embed.FS`（内嵌了 `cmd/detl/script/` 下的 SQL 文件）
- 不再归类为死代码，但引入新的注意点：`LoadTask` 依赖 `hotswap.GetScriptDir()` 必须先被初始化（隐式初始化顺序依赖）

### 17. `Conf` 包级单例 + 可变状态存在数据竞争风险 ❌

- 未处理

### 18. `LoadTask` 缺少字段约束前置校验 ❌

- 未处理（`LoadTask` 仅增加了 `hotswap` 读取逻辑，未增加 `Source/Load/Tasks` 校验）

### 19. 环境变量与任务模式的配置冲突（重点） ❌

- 未处理。`TASK_DIR` vs `SCRIPT_DIR` 的路径问题因 `resolveTaskPath` 调用移除而有所缓解，但其他冲突点（DSN 污染、OutputDir 冲突、builtinTransform 的 source 字段等）全部未动

### 20. 数据输出目录未被创建 ❌

- 未处理

---

## 本次变更总结

根据 git diff，以下文件被修改：

| 文件 | 变更 |
|------|------|
| `cmd/detl/main.go` | `GetConf` 调用分离为 `SetConf`+`GetConf`；去掉 `flag` 导入；`ConfDir`/`ScriptDir` 提升为包级变量；新增 `initScript()` 初始化 `hotswap` |
| `cmd/detl/main_func.go` | 移除 `resolveTaskPath` 调用，函数本体加 TODO 注释 |
| `conf/conf.go` | `GetConf()` 不再接受参数；新增 `SetConf(confdir)` 显式初始化；`GetConf` 加入守卫检查 |
| `conf/conf_test.go` | 已删除 |
| `hotswap/script.go` | 重命名 `GetScriptDir(sd)` → `SetScriptDir(sd)`；新增无参 `GetScriptDir()` |
| `internal/task/task.go` | `LoadTask` 通过 `hotswap.GetScriptDir().GetScriptBytes(path)` 读取，回退 `os.ReadFile` |

### 已解决的问题

- **问题 1**（GetConf 混乱）— 部分解决。`GetConf` 不再接受 confdir 参数，`SetConf` 分离初始化
- **问题 10**（resolveTaskPath fallback）— 部分解决。调用点已移除，函数本体尚未删除
- **问题 16**（hotswap 死代码）— 策略变更。hotswap 已被重新启用，不再是死代码

### 仍存在的问题

其余 17 个问题（2-9、11-15、17-20）均未处理。

### 新引入的问题

- **`conf/conf_test.go` 删除**：损失了 DSN 分组查询、旧版格式兼容、system.yaml 加载等测试覆盖
- **隐式初始化顺序依赖**：`LoadTask()` 现在依赖 `hotswap.GetScriptDir()` 返回非 nil，必须在调用前由 `main.initScript()` 初始化。若在测试或非标准入口中直接调 `LoadTask()` 会 panic
- **`SetConf` 双重守卫**：`SetConf` 内的 `if cf != nil` 检查与 `sync.Once` 功能重叠，逻辑冗余

## 核心结论

`main_func.go` 是一个 400+ 行的上帝文件，承担了配置解析、转换实现、DSN 查找、任务调度等多重职责，且通过包级变量与 `main.go` 隐式耦合。根包 `dsql.go` 和 `hotswap/script.go` 的存在进一步模糊了模块边界。最紧迫的改进方向：

1. **环境变量 vs YAML 任务配置优先级规范化**——当前两者混合作用，规则隐式且不可预测。应明确定义：任务模式下 YAML 优先，环境变量仅提供系统级默认值。
2. **抽离 `buildPipeline(config)` 公共路径**，消除 `runETL` 与 `runETLFromTask` 的重复
3. **消除包级变量耦合**，让配置显式传递
4. **`builtinTransform` 改为可配置或删除**，不再硬编码业务 schema
5. **统一错误处理策略**，移除 `log.Fatalf`
6. **`runJob` 增加环检测**
7. **移除 `dsql.go` 和 `hotswap/script.go` 的冗余代码**
8. **`stdoutLoad` 使用 `csv.Writer` 而非手写拼接**，避免 CSV 格式错误
9. **`sortStrings` 替换为标准库 `sort.Strings`**

### 环境变量冗余/冲突完整清单

| 环境变量 | 环境变量模式 | 任务模式 | 问题 |
|---------|------------|---------|------|
| `DB_DRIVER` | 必须 | 不参与（YAML 指定） | 任务模式下仍通过 `cf.InitDSN` 写入 dsn.json，污染配置 |
| `ACTIVE_DSN` | 必须 | 不参与（YAML 指定） | 同上，且默认值包含密码，写死在代码中不安全 |
| `SCRIPT_FILE` | 必须 | 不参与（YAML 的 `query_file`） | 任务模式下无意义但仍在 init 中注册 |
| `TRANSFORM_MODE` | 必须 | YAML 的 `transform.mode` 覆盖 | 两套逻辑重复，优先级隐式 |
| `TRANSFORM_SCRIPT` | 条件必须 | YAML 的 `transform.script` 覆盖 | 同上 |
| `LOAD_TYPE` | 必须 | 不参与（YAML 的 `load.type`） | 即使使用 YAML 也会被 init 解析，无意义 |
| `OUTPUT_DIR` | 必须 | 用于 CSV 相对路径拼接到 YAML 的文件名 | 混合使用造成路径混乱 |
| `OUTPUT_FILE` | 必须 | 仅用于 CSV 拼接 | 语义混乱：环境变量模式下是文件名，任务模式下被 YAML 的 `load.file` 覆盖 |
| `OUTPUT_COLUMNS` | 必须 | 被 YAML 的 `load.columns` 覆盖 | 两套逻辑重复 |
| `TASK_DIR` | 无用 | 没有被 `resolveTaskPath` 使用 | 定义为环境变量但实际未被消费 |
