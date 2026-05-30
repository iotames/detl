# detl — 简易 ETL 工具

数据的抽取（Extract）、转换（Transform）、载入（Load）。

- **抽取**：MySQL, PostgreSQL
- **转换**：内置 Go 函数 或 **Python 脚本**
- **载入**：CSV 文件、控制台输出、SQL 写入（insert/upsert）

---

## 编译 / 测试 / 安装

```bash
# 本地编译
cd cmd/detl && go build .

# 运行测试（需要 PG localhost:5432, user=postgres, password=postgres）
go test -v -run TestPipeline_PG_to_CSV

# MySQL 测试（需要 MySQL localhost:3306, root:root）
go test -v -run TestPipeline_MySQL_to_CSV

# 全部测试
go test -v ./...

# 代码检查
go vet ./...

# 安装到 $GOPATH/bin
go install ./cmd/detl
```

安装后若 `$GOPATH/bin` 在 PATH 中，可直接执行 `detl.exe`。

```bash
# 安装演示数据工具
go install ./cmd/seed
```

---

## 目录结构

```
├── conf/           # 数据源配置（dsn.json）
├── script/         # ETL 脚本（SQL + Python）
└── task/           # ETL 任务 YAML 定义
```

`conf/`、`script/`、`task/` 中的文件为示例，可复制到工作目录后按需修改。

---

## 快速开始：用户表 ETL

完整 ETL 流程：MySQL → 清洗转换 → CSV。

### 1. 准备数据源

`conf/dsn.json` 定义数据源连接：

```json
{
  "DsnList": [
    {"Name": "dev_mysql", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/detl_test?charset=utf8mb4"},
    {"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres password=postgres dbname=detl_test sslmode=disable"}
  ]
}
```

### 2. 创建测试数据

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS detl_test CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
go test -v -run TestPipeline_MySQL_to_CSV
```

### 3. 编写抽取脚本

`script/e_detl_users.sql`：

```sql
SELECT id, first_name, last_name, email, age, created_at
FROM detl_test_users
ORDER BY id
```

### 4. 定义 ETL 任务

`task/user_etl.yaml`：

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

### 5. 运行

```bash
detl.exe -task user_etl.yaml
```

输出文件在 `output/etl_output.csv`：

```csv
id,full_name,email,age,created_at,source,etl_time
1,John Doe,john@example.com,30,2024-01-15,mysql,2026-05-26 19:10:30
2,Jane Smith,jane@example.com,25,2024-02-20,mysql,2026-05-26 19:10:30
```

### 切换输出方式

```yaml
# 控制台输出
load:
  type: stdout
  columns: [id, full_name, email]
```

```yaml
# SQL 写入（upsert）
load:
  type: sql
  file: etl_output       # 目标表名
  mode: upsert
  keys: [id]
```

---

## 实战：商品表 ETL（MySQL → Python → PostgreSQL）

完整链路：MySQL 抽取商品数据 → Python 脚本转换 → PostgreSQL upsert 写入。

### 1. 准备演示数据

`seed` 命令自动读取 `conf/dsn.json`，创建 PG 目标表并填充 MySQL 演示数据：

```bash
# PG 目标建表
seed -pg

# MySQL 填充 300 条商品演示数据
seed

# 自定义数量
seed -count 500
```

### 2. 抽取脚本

`script/get_products.sql`：

```sql
SELECT id, title, category_id, price, stock, description, status, is_deleted, created_at, updated_at
FROM etl_test_product
WHERE is_deleted = 0
ORDER BY id
```

### 3. Python 转换脚本

`script/t_product.py` 负责字段映射（MySQL 单 title → PG 三标题）、价格翻倍等。核心逻辑：

```python
import sys, json

PRICE_MULTIPLIER = 2.0

def transform_row(row):
    title_cn = str(row.get("title") or row.get("title_cn") or "")
    price = round(float(row.get("price", 0) or 0) * PRICE_MULTIPLIER, 2)
    return {
        "id": row.get("id"),
        "title_cn": title_cn,
        "title_en": "",           # 可扩展英文映射
        "title": title_cn,
        "category_id": int(row.get("category_id", 0) or 0),
        "price": price,
        "stock": int(row.get("stock", 0) or 0),
        "description": str(row.get("description", "") or ""),
        "status": int(row.get("status", 1) or 1),
        "is_deleted": int(row.get("is_deleted", 0) or 0),
        "created_at": str(row.get("created_at", "")),
        "updated_at": str(row.get("updated_at", "")),
    }

for line in sys.stdin:
    row = json.loads(line.strip())
    print(json.dumps(transform_row(row), ensure_ascii=False), flush=True)
```

### 4. 定义 ETL 任务

`task/product_sync_to_pg.yaml`：

```yaml
kind: 转换
name: 商品数据同步到 PG
source:
  connection: mysql8_local
  query_file: get_products.sql
transform:
  mode: python
  script: t_product.py
load:
  type: sql
  connection: pg2              # 目标数据源 Name
  table: debug.etl_test_product # schema.table
  mode: upsert                 # 重复则更新
  key_columns: [id]
  create_table: false
  batch_size: 50
```

### 5. 运行

```bash
# 先控制台输出快速验证 Python 脚本
detl -task task/product_mysql_stdout.yaml

# 确认无误后写入 PG
detl -task task/product_sync_to_pg.yaml
```

SQL Load 每批写入 50 行，`upsert` 模式保证幂等性——重复运行也不会产生重复数据。

### 调试用 YAML 变体

```yaml
# MySQL → stdout（快速验证 Python 脚本）
source:
  connection: mysql8_local
  query_file: get_products.sql
transform:
  mode: python
  script: t_product.py
load:
  type: stdout
  columns: [id, title_cn, title_en, title, price, stock, category_id, status]
```

```yaml
# PG → stdout（独立测试，不依赖 MySQL）
source:
  connection: pg2
  query_file: get_pg_products.sql
transform:
  mode: python
  script: t_product.py
load:
  type: stdout
  columns: [id, title_cn, title_en, title, price, stock, category_id, status]
```

---

## Python 脚本转换

`TRANSFORM_MODE=python` 会启动一个常驻 Python 子进程，通过 stdin/stdout 传递 JSON 行，逐行读写。

### 脚本规范

- 输入：stdin，每行一个 JSON 对象
- 输出：stdout，每行一个 JSON 对象
- 返回 `null` 或空行可跳过该行
- **必须 flush stdout**（`print(..., flush=True)`）

```yaml
transform:
  mode: python
  script: t_product.py
```

---

## 环境变量大全（暂未充分测试）

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

## 注意事项

- `cmd/detl/conf/`、`cmd/detl/script/`、`cmd/detl/task/` 是示例目录，可复制到工作目录后修改
- `builtin` 转换为硬编码示例（针对 `detl_test_users` 表），非通用实现
- 作业（`kind: 作业`）执行引擎尚未实现
- SQL Load 自动建表时所有列使用 TEXT 类型
