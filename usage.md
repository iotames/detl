# detl 使用指南

## 概述

detl 是一个命令行 ETL 工具，通过 **Go 代码** 或 **任务定义文件** 来描述数据流程：从数据源抽取 → 转换 → 载入到目标端。

> **当前实现状态**：核心 Pipeline 引擎已可用，支持 PG/MySQL SQL 抽取、Go 函数转换、CSV 文件写入。其余功能为规划中。

---

## 安装与运行

```bash
# 构建
go build -o detl.exe ./cmd/detl    # 📋 CLI 待实现
```

当前通过 Go 测试运行 ETL 流程：

```bash
# 运行 PG → Transform → CSV 集成测试
go test -v -run TestPipeline_PG_to_CSV

# 运行 MySQL → Transform → CSV 集成测试
go test -v -run TestPipeline_MySQL_to_CSV
```

### 环境变量（规划中）

| 环境变量 | 对应参数 |
|---|---|
| `CONF_DIR` | 配置目录（默认 `conf`） |
| `SCRIPT_DIR` | 脚本目录（默认 `script`） |
| `DB_DRIVER` | 默认数据库驱动（默认 `postgres`） |
| `ACTIVE_DSN` | 默认数据源连接字符串 |

---

## Pipeline 编程接口

当前通过 Go 代码直接调用 `internal/` 包的 API 来编排 ETL 流程。

### Source（数据抽取）✅

#### SQL

从 PostgreSQL 或 MySQL 按查询语句抽取数据。

```go
import "github.com/iotames/detl/internal/source"

src := source.NewSQL(source.SQLConfig{
    Driver: "postgres",               // "postgres" | "mysql"
    DSN:    "host=127.0.0.1 ...",     // 连接字符串
    Query:  "SELECT * FROM users",    // SQL 查询
})
```

#### 文件（CSV / JSON）📋 待实现

从本地文件读取数据。

### Transform（数据转换）✅

通过 Go 函数实现数据转换。

```go
import "github.com/iotames/detl/internal/transform"

tf := transform.Func(func(row map[string]any) ([]map[string]any, error) {
    // 字段映射
    row["user_id"] = row["id"]
    // 类型转换
    row["age"] = int(row["age"].(int64))
    // 新增计算字段
    row["full_name"] = fmt.Sprintf("%s %s", row["first_name"], row["last_name"])
    // 删除字段
    delete(row, "first_name")
    delete(row, "last_name")
    return []map[string]any{row}, nil
})
```

返回约定：
- 返回包含单行的 `[]map[string]any`：替换当前行
- 返回空 `nil`：跳过该行（不输出到 Load）
- 返回多行：展开为多行

### Load（数据载入）✅ CSV / 📋 其他待实现

#### CSV 文件写入 ✅

```go
import "github.com/iotames/detl/internal/load"

ld := load.NewCSV(load.CSVConfig{
    Path:    "./output/users.csv",       // 输出路径
    Columns: []string{"id", "name", "email"},  // 列名顺序
})
```

#### SQL 写入（UPSERT）📋 待实现

#### 控制台输出 📋 待实现

#### 多目标 📋 待实现

### Pipeline 编排 ✅

```go
import "github.com/iotames/detl/internal/engine"

p := engine.New(src, tf, ld)
if err := p.Run(); err != nil {
    // 处理错误
}
```

### 完整示例 ✅

```go
package main

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
    "github.com/iotames/detl/internal/engine"
    "github.com/iotames/detl/internal/load"
    "github.com/iotames/detl/internal/source"
    "github.com/iotames/detl/internal/transform"
)

func main() {
    // 1. Source：从 Postgres 抽取
    src := source.NewSQL(source.SQLConfig{
        Driver: "postgres",
        DSN:    "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable",
        Query:  "SELECT id, first_name, last_name, email, age, created_at FROM users",
    })

    // 2. Transform：清洗转换
    tf := transform.Func(func(row map[string]any) ([]map[string]any, error) {
        fn := fmt.Sprintf("%v", row["first_name"])
        ln := fmt.Sprintf("%v", row["last_name"])
        fullName := fn
        if ln != "" && ln != "<nil>" {
            fullName += " " + ln
        }
        return []map[string]any{{
            "id":        row["id"],
            "full_name": fullName,
            "email":     row["email"],
        }}, nil
    })

    // 3. Load：写入 CSV
    ld := load.NewCSV(load.CSVConfig{
        Path:    "./users.csv",
        Columns: []string{"id", "full_name", "email"},
    })

    // 4. 运行 Pipeline
    p := engine.New(src, tf, ld)
    if err := p.Run(); err != nil {
        panic(err)
    }
    fmt.Println("完成")
}
```

---

## 集成测试

### PostgreSQL

```bash
go test -v -run TestPipeline_PG_to_CSV
```

测试流程：
1. 连接 PG（`user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432`）
2. 创建 `detl_test_users` 表并插入 5 行测试数据
3. 执行 Pipeline：SQL 抽取 → 姓名拼接/邮箱小写/NULL 处理/日期格式化 → CSV 输出
4. 验证 CSV 内容

### MySQL

```bash
go test -v -run TestPipeline_MySQL_to_CSV
```

测试流程同上，数据库为 `detl_test`，连接 `root:root@127.0.0.1:3306`。测试表保留不删除。

---

## DSN 配置管理

连接字符串存储在 `conf/dsn.json` 中，支持多数据源管理。

```json
{
  "list": [
    {
      "code": "a1b2c3d4e5f6g7h8",
      "driver": "postgres",
      "dsn": "host=... dbname=prod",
      "active": true
    },
    {
      "code": "i9j0k1l2m3n4o5p6",
      "driver": "mysql",
      "dsn": "user=... dbname=staging",
      "active": false
    }
  ]
}
```

---

## 目录结构约定

```
detl/
├── conf/                   # 配置文件目录
│   └── dsn.json           # 数据源连接配置
├── script/                 # ETL 业务脚本
├── internal/               # 核心库
│   ├── engine/             # ✅ Pipeline 编排
│   ├── source/             # ✅ 数据抽取
│   ├── transform/          # ✅ 数据转换
│   └── load/               # ✅ 数据载入
└── output/                 # 文件输出目录
```

---

> **注**：带 ✅ 标记的为已实现功能，📋 标记的为规划中。本文档将随开发迭代持续更新。
