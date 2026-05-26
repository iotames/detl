# detl 使用指南

## 概述

detl 是一个命令行 ETL 工具，通过 **任务定义文件** 或 **脚本文件** 来描述数据流程：从数据源抽取 → 转换 → 载入到目标端。

---

## 安装与运行

```bash
# 构建
go build -o detl.exe ./cmd/detl

# 运行任务
detl -task task.yml

# 运行脚本目录中的骨架文件
detl -dir script/main_users.py
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-task` | — | 任务定义文件路径（YAML/JSON），与 `-dir` 二选一 |
| `-dir` | `script` | ETL 业务脚本目录，执行其中的 `main_*` 骨架文件 |
| `-conf` | `conf` | 配置文件目录（存放 `dsn.json` 等） |
| `-driver` | `postgres` | 默认数据库驱动 |
| `-dsn` | `user=postgres ...` | 默认数据源连接字符串 |

### 环境变量

所有命令行参数都支持通过环境变量覆盖：

| 环境变量 | 对应参数 |
|---|---|
| `CONF_DIR` | `-conf` |
| `SCRIPT_DIR` | `-dir` |
| `DB_DRIVER` | `-driver` |
| `ACTIVE_DSN` | `-dsn` |

---

## ETL 业务脚本系统

所有脚本文件存放在 `script/` 目录中，通过文件名前缀约定职责。

### 命名规则

| 前缀 | 用途 | 示例 |
|---|---|---|
| `e_` | 抽取独立脚本片段 | `e_getusers.sql`, `e_fetch_api.py` |
| `main_` | ETL 骨架入口文件 | `main_users.py`, `main_users.sql` |
| 其他 | 辅助/工具脚本 | `utils.py`, `config.json` |

### 骨架文件（main_*）

骨架文件是一个完整的 ETL 任务描述，可以用多种格式编写：

#### main_*.sql — 纯 SQL 模式

适合简单的数据库到数据库迁移，全部用 SQL 完成。

```sql
-- script/main_users.sql
-- 直接从源库抽取并写入目标库（由 Load 配置决定写入方式）
-- 抽取：SELECT * FROM users WHERE updated_at > :last_run
```

执行方式：

```bash
detl -dir script/main_users.sql
```

#### main_*.py — Python 脚本模式

适合需要复杂逻辑的 ETL 流程。detl 会注入内置变量和函数。

```python
# script/main_users.py
# 使用内置函数抽取数据
rows = detl.source.sql("postgres", "SELECT * FROM users WHERE created_at > '$last_run'")

# 转换：Python 任意逻辑
def transform(row):
    row["full_name"] = f"{row['first_name']} {row['last_name']}"
    row["role"] = "user"
    return row

transformed = [transform(r) for r in rows]

# 载入到目标
detl.load.sql("mysql", "users", transformed, mode="upsert")
```

执行方式：

```bash
detl -dir script/main_users.py
```

#### main_*.yml / main_*.json — 声明式模式

通过 YAML/JSON 声明 ETL 流程，无需写代码。

```yaml
# script/main_users.yml
name: 用户数据同步
source:
  type: sql
  driver: postgres
  sql: "SELECT * FROM users WHERE updated_at > '{{.LastRun}}'"
transform:
  - type: python
    script: |
      row["full_name"] = f"{row['first_name']} {row['last_name']}"
load:
  type: sql
  driver: mysql
  table: users
  mode: upsert
```

执行方式：

```bash
detl -dir script/main_users.yml
```

---

## 任务定义文件（task.yml）

通过 `-task` 参数执行独立的 YAML/JSON 任务文件，与脚本系统解耦。

### 结构

```yaml
# task.yml
name: "用户数据同步"
source:
  type: sql                     # source 类型
  driver: postgres              # 数据库驱动
  dsn: "user=... dbname=..."    # 数据源连接字符串
  sql: "SELECT * FROM users"    # 查询语句
transform:                      # 可选，省略则跳过转换
  - type: python
    script: |
      row["full_name"] = f"{row['first_name']} {row['last_name']}"
load:                           # 目标端
  type: sql
  driver: mysql
  dsn: "user=... dbname=..."
  table: "users"
  mode: upsert                  # insert | upsert | replace
```

---

## Source（数据抽取）

### SQL

从 PostgreSQL 或 MySQL 按查询语句抽取数据。

```yaml
source:
  type: sql
  driver: postgres           # postgres | mysql
  dsn: "host=127.0.0.1 ..."  # 连接字符串
  sql: "SELECT * FROM users" # SQL 查询
  # 或引用外部脚本文件
  script_file: "e_getusers.sql"
```

### 文件（CSV / JSON）

从本地文件读取数据。

```yaml
source:
  type: file
  format: csv                  # csv | json
  path: "./data/users.csv"     # 文件路径
  # CSV 特有选项
  delimiter: ","
  has_header: true
  encoding: "utf-8"
```

```yaml
source:
  type: file
  format: json
  path: "./data/users.json"
  # JSON 指针路径，提取嵌套数据
  pointer: "/data/items"
```

### Python 脚本

通过 Python 脚本获取数据，适用于 HTTP API、爬虫等灵活场景。

```yaml
source:
  type: python
  script: |
    import requests
    resp = requests.get("https://api.example.com/users")
    return resp.json()
  # 或引用外部文件
  script_file: "e_fetch_api.py"
```

Python 脚本的返回值要求：
- 返回 `list[dict]` 类型，每个 dict 代表一行数据
- 返回空列表表示无数据

---

## Transform（数据转换）

### Python 脚本转换

```yaml
transform:
  - type: python
    script: |
      # 字段映射
      row["user_id"] = row.pop("id")
      # 类型转换
      row["age"] = int(row["age"])
      # 新增计算字段
      row["full_name"] = f"{row['first_name']} {row['last_name']}"
      return row
```

多个转换按顺序依次执行：

```yaml
transform:
  - type: python
    script_file: "t_clean.py"
  - type: python
    script_file: "t_enrich.py"
```

返回约定：
- 返回 dict：替换当前行
- 返回 None / 空：跳过该行（不输出到 Load）
- 返回 list[dict]：展开为多行

---

## Load（数据载入）

### SQL 写入

```yaml
load:
  type: sql
  driver: mysql              # postgres | mysql
  dsn: "user=... dbname=..."
  table: "users"             # 目标表
  mode: upsert               # 写入模式
  # 写入模式说明：
  #   insert  - 追加写入（主键冲突报错）
  #   upsert  - 主键冲突则更新
  #   replace - 先删后插
  batch_size: 500            # 批量提交行数（默认 500）
  # 字段映射：源字段名 → 目标表列名
  mapping:
    user_id: id
    full_name: name
    email: email
```

### 文件写入

```yaml
load:
  type: file
  format: csv                 # csv | json
  path: "./output/users.csv"  # 输出路径
  # CSV 选项
  delimiter: ","
  include_header: true
  # JSON 选项
  json_indent: 2              # JSON 缩进，设为 0 紧凑输出
```

### 控制台输出

```yaml
load:
  type: stdout
  format: table               # table | json | csv
  # table：以表格形式打印到终端
  # json：打印 JSON Lines
  # csv：打印 CSV 格式
```

### 多目标

同一份数据可以同时写入多个目标：

```yaml
load:
  - type: sql
    driver: mysql
    table: users
    mode: upsert
  - type: file
    format: csv
    path: "./backup/users.csv"
  - type: stdout
```

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

首次运行时会自动创建 `conf/dsn.json`，你也可以手动编辑。

通过 `-dsn` 参数可以临时切换数据源，不会修改 `dsn.json`。

---

## 完整示例

### 示例 1：Postgres → 转换 → MySQL

```yaml
# sync_users.yml
name: "用户数据同步"
source:
  type: sql
  driver: postgres
  sql: "SELECT id, first_name, last_name, email FROM users WHERE updated_at > '2024-01-01'"
transform:
  - type: python
    script: |
      row["full_name"] = f"{row['first_name']} {row['last_name']}".strip()
      row["email"] = row["email"].lower()
      del row["first_name"]
      del row["last_name"]
      return row
load:
  type: sql
  driver: mysql
  table: users
  mode: upsert
  mapping:
    id: user_id
    full_name: name
```

### 示例 2：HTTP API → 转换 → CSV 文件

```yaml
# fetch_orders.yml
name: "拉取订单"
source:
  type: python
  script: |
    import requests, os
    api_key = os.environ["API_KEY"]
    resp = requests.get("https://api.example.com/orders", headers={"Authorization": f"Bearer {api_key}"})
    return resp.json()["data"]
transform:
  - type: python
    script: |
      row["amount"] = float(row["amount"])
      row["created_at"] = row["created_at"][:10]
      return row
load:
  type: file
  format: csv
  path: "./output/orders.csv"
```

### 示例 3：CSV 文件 → 控制台预览

```yaml
# preview.yml
name: "预览用户数据"
source:
  type: file
  format: csv
  path: "./data/users.csv"
transform:
  - type: python
    script: |
      # 只保留关键字段
      return {k: row[k] for k in ["id", "name", "email", "created_at"]}
load:
  type: stdout
  format: table
```

---

## 错误处理与重试

- Source 连接失败：重试 3 次，间隔 5 秒
- SQL 执行超时：默认 30 秒，可通过 `timeout` 配置
- Transform 脚本异常：跳过当前行，记录错误，继续下一行
- Load 写入失败：批量数据单条重试，跳过持久性错误行

---

## 目录结构约定

```
detl/
├── conf/                   # 配置文件目录
│   └── dsn.json           # 数据源连接配置
├── script/                 # ETL 业务脚本
│   ├── e_getusers.sql     # 抽取脚本
│   ├── e_fetch_api.py     # 抽取脚本
│   ├── t_clean.py         # 转换脚本
│   ├── main_users.py      # 骨架入口
│   └── main_users.yml     # 骨架入口
└── output/                 # 文件输出目录（自动创建）
```

---

> **注**：本文档描述的功能为规划中的设计，具体实现可能随开发迭代调整。
