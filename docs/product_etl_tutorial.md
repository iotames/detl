# DETL 实战案例：商品数据 ETL 全流程

> 适用工具：`detl`（ETL 执行引擎）、`seed`（演示数据填充）
> 数据流：MySQL 抽取 → Python 转换 → PostgreSQL 加载
> 难度：★★★☆☆ | 预计时间：30 分钟

---

## 目录

1. [环境准备](#1-环境准备)
2. [DSN 数据源配置](#2-dsn-数据源配置)
3. [数据库空表创建](#3-数据库空表创建)
4. [演示数据填充](#4-演示数据填充)
5. [SQL 抽取脚本](#5-sql-抽取脚本)
6. [Python 转换脚本](#6-python-转换脚本)
7. [YAML 任务定义](#7-yaml-任务定义)
8. [运行 ETL 任务](#8-运行-etl-任务)
9. [转换层灵活控制](#9-转换层灵活控制)
10. [完整文件结构](#10-完整文件结构)

---

## 1. 环境准备

### 1.1 安装工具

```bash
# 安装 detl ETL 引擎
go install github.com/iotames/detl/cmd/detl@latest

# 安装 seed 演示数据工具
go install github.com/iotames/detl/cmd/seed@latest

# 验证安装
detl -version
seed -h
```

### 1.2 数据库要求

| 数据库 | 用途 | 默认连接 |
|--------|------|---------|
| MySQL 8.0+ | 源数据（OLTP） | `root:root@tcp(127.0.0.1:3306)` |
| PostgreSQL 15+ | 目标数据（OLAP） | `postgres:postgres@tcp(127.0.0.1:5432)` |

### 1.3 创建 ETL 工作目录

一个 ETL 项目就是一个独立的文件夹，包含配置、脚本和任务定义。

```bash
mkdir my_etl_project && cd my_etl_project
mkdir conf script task output
```

目录说明：

| 目录 | 用途 |
|------|------|
| `conf/` | DSN 连接配置（dsn.json） |
| `script/` | SQL 抽取脚本 + Python 转换脚本 |
| `task/` | YAML 任务定义文件 |
| `output/` | CSV 输出文件（自动生成） |

---

## 2. DSN 数据源配置

### 2.1 什么是 DSN

DSN（Data Source Name）是数据库连接字符串，告诉 ETL 引擎"连哪个数据库、用什么账号"。

### 2.2 配置 dsn.json

在 `conf/dsn.json` 中定义所有数据源：

```json
{
    "ActiveCode": "pg2",
    "DsnList": [
        {
            "Code": "pg1",
            "Name": "pg1",
            "DriverName": "postgres",
            "Dsn": "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable search_path=public"
        },
        {
            "Code": "pg2",
            "Name": "pg2",
            "DriverName": "postgres",
            "Dsn": "user=postgres password=postgres dbname=postgres host=127.0.0.1 port=5432 sslmode=disable search_path=debug"
        },
        {
            "Code": "mysql8_local",
            "Name": "mysql8_local",
            "DriverName": "mysql",
            "Dsn": "root:root@tcp(127.0.0.1:3306)/debugdb?charset=utf8mb4&parseTime=True&loc=Local"
        }
    ]
}
```

字段说明：

| 字段 | 说明 |
|------|------|
| `Code` | 内部编码（自动生成时可省略） |
| `Name` | **连接名** —— YAML 任务中通过此名称引用数据源 |
| `DriverName` | 驱动类型：`postgres` 或 `mysql` |
| `Dsn` | 连接字符串 |

> **要点**：`Name` 字段是 ETL 任务引用数据源的依据，务必为每个数据源设置一个有意义的名称。

### 2.3 验证配置

```bash
# detl 启动时会自动加载 conf/dsn.json
# 配置无误即可跳过此步
```

---

## 3. 数据库空表创建

### 3.1 PostgreSQL 建表（seed -pg 模式）

`seed -pg` 命令自动读取 `dsn.json` 中 `Name` 为 `pg2` 的数据源，执行建表 DDL。

```bash
# 在 ETL 项目根目录执行
seed -pg
```

执行成功后控制台输出：

```
读取 DSN 配置: /path/to/conf/dsn.json
postgres(pg2) 连接成功
PG 建表完成
```

### 3.2 创建的表结构

`seed -pg` 会自动创建以下 4 张表（均在 `debug` schema 下）：

**product_category** —— 商品分类主表

| 列名 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键，自增 |
| name | VARCHAR(100) | 分类名称 |
| parent_id | BIGINT | 父分类 ID（支持多级分类） |
| sort_order | INT | 排序号 |
| status | SMALLINT | 1=启用，0=禁用 |
| created_at | TIMESTAMP | 创建时间 |

**product_tag** —— 商品标签表

| 列名 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键 |
| name | VARCHAR(50) | 标签名称（唯一） |
| created_at | TIMESTAMP | 创建时间 |

**etl_test_product** —— 商品信息表（PG 三标题版）

| 列名 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键 |
| title_cn | VARCHAR(255) | 中文标题 |
| title_en | VARCHAR(255) | 英文标题 |
| title | VARCHAR(255) | 通用标题 |
| category_id | BIGINT | 所属分类 ID |
| price | NUMERIC(10,2) | 价格 |
| stock | INT | 库存量 |
| description | TEXT | 商品描述 |
| status | SMALLINT | 1=上架，0=下架 |
| is_deleted | SMALLINT | 软删除标记 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

**product_tag_x** —— 商品与标签多对多关系表

| 列名 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | 主键 |
| product_id | BIGINT | 商品 ID |
| tag_id | BIGINT | 标签 ID |

> 产品字段差异说明：MySQL 源表只用一个 `title` 字段，PG 目标表扩展为 `title_cn`、`title_en`、`title` 三个字段，通过 Python 转换脚本实现映射。

---

## 4. 演示数据填充

### 4.1 MySQL 种子数据

`seed` 命令自动读取 `dsn.json` 中 `Name` 为 `mysql8_local` 的数据源，创建表并填充演示数据。

```bash
# 填充 300 条商品演示数据（默认值）
seed

# 自定义数量
seed -count 500
```

### 4.2 写入的数据

执行成功后自动完成：

| 数据表 | 数量 | 说明 |
|--------|------|------|
| `product_category` | 10 条 | 含三级分类（电子→手机→智能手机） |
| `product_tag` | 20 条 | 热销、新品、时尚、超值等 |
| `etl_test_product` | 300 条（可配） | 按分类权重分布，含随机价格/库存 |
| `product_tag_x` | ≈ 600~1500 条 | 每商品 1~5 个标签 |

数据特点：

- 商品标题带中文前缀修饰（极简蓝牙耳机、轻奢充电宝...）
- 价格范围 9.99 ~ 9999.99 元
- 约 10% 的商品为下架状态
- 约 2% 的商品为软删除状态
- 创建时间均匀分布在过去一年

### 4.3 验证数据

```bash
# 用 MySQL 命令行检查
mysql -uroot -proot debugdb -e "
  SELECT c.name AS category, COUNT(*) AS cnt
  FROM etl_test_product p
  JOIN product_category c ON c.id = p.category_id
  GROUP BY c.name
  ORDER BY cnt DESC;
"
```

---

## 5. SQL 抽取脚本

### 5.1 MySQL 抽取脚本

在 `script/get_products.sql` 中定义从 MySQL 源表抽取数据的 SQL：

```sql
SELECT
    id,
    title,
    category_id,
    price,
    stock,
    description,
    status,
    is_deleted,
    created_at,
    updated_at
FROM etl_test_product
WHERE is_deleted = 0
ORDER BY id
```

### 5.2 PostgreSQL 抽取脚本

如要从 PG 抽取（用于测试 PG → Python → X 的流程），同样在 `script/get_pg_products.sql`：

```sql
SELECT
    id,
    title_cn,
    title_en,
    title,
    category_id,
    price,
    stock,
    description,
    status,
    is_deleted,
    created_at,
    updated_at
FROM etl_test_product
WHERE is_deleted = 0
ORDER BY id
```

> **注意**：SQL 脚本只做抽取（SELECT），不做转换。所有数据清洗、字段映射、业务逻辑都在 Python 转换层完成。

---

## 6. Python 转换脚本

### 6.1 核心转换脚本

`script/t_product.py` 是本次 ETL 的核心，负责：

1. **字段映射**：MySQL 的 `title` → PG 的 `title_cn`、`title_en`、`title`
2. **价格处理**：价格翻倍（可配置倍数）
3. **类型转换**：确保数据类型与 PG 目标表匹配

```python
"""
商品数据转换脚本

协议：stdin 读一行 JSON → stdout 写一行 JSON (flush=True)
返回 "" 或 null 跳过该行
"""
import json
import sys

# ========== 转换层控制开关 ==========
SKIP_TRANSFORM = False          # True = 跳过所有转换，原样透传
PRICE_MULTIPLIER = 2.0          # 价格倍数，1.0 即不处理
# ===================================

def transform_row(row):
    """单行转换"""
    title_cn = str(row.get("title") or row.get("title_cn") or "")
    title_en = title_to_en(title_cn)
    title = title_cn

    price = float(row.get("price", 0) or 0)
    price = round(price * PRICE_MULTIPLIER, 2)

    return {
        "id": row.get("id"),
        "title_cn": title_cn,
        "title_en": title_en,
        "title": title,
        "category_id": int(row.get("category_id", 0) or 0),
        "price": price,
        "stock": int(row.get("stock", 0) or 0),
        "description": str(row.get("description", "") or ""),
        "status": int(row.get("status", 1) or 1),
        "is_deleted": int(row.get("is_deleted", 0) or 0),
        "created_at": str(row.get("created_at", "")),
        "updated_at": str(row.get("updated_at", "")),
    }

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        row = json.loads(line)

        if SKIP_TRANSFORM:
            # 透传模式：只补充缺失字段
            row.setdefault("title_cn", row.get("title", ""))
            row.setdefault("title_en", "")
            row.setdefault("title", row.get("title", ""))
            print(json.dumps(row, ensure_ascii=False), flush=True)
            continue

        result = transform_row(row)
        print(json.dumps(result, ensure_ascii=False), flush=True)

if __name__ == "__main__":
    main()
```

### 6.2 通信协议

Python 脚本与 detl 引擎之间通过 **stdin/stdout** 传递 JSON 行：

```
detl 引擎                    Python 子进程
    │                            │
    ├── JSON 行 (stdin) ────────→ │
    │                            ├── 处理数据
    │ ←── JSON 行 (stdout) ───── │
    │                            │
    ├── JSON 行 (stdin) ────────→ │
    │                            ├── 处理数据
    │ ←── JSON 行 (stdout) ───── │
```

> **关键规则**：Python 脚本必须 `flush=True`，否则数据会卡在缓冲区。

---

## 7. YAML 任务定义

### 7.1 完整 ETL 链路：MySQL → PG2

创建 `task/product_sync_to_pg.yaml`，定义源、转换、目标三个环节：

```yaml
kind: 转换
name: 商品数据同步到 PG2

source:
  connection: mysql8_local        # dsn.json 中的 Name
  query_file: get_products.sql    # script/ 下的 SQL 文件

transform:
  mode: python                    # 使用 Python 脚本转换
  script: t_product.py            # script/ 下的 Python 文件

load:
  type: sql                       # 写入数据库
  connection: pg2                 # 目标数据源 Name
  table: debug.etl_test_product   # schema.table
  mode: upsert                    # upsert（重复则更新）
  key_columns: [id]               # upsert 唯一键
  create_table: false             # 表已存在，不自动建表
  batch_size: 50                  # 每批 50 行
```

### 7.2 MySQL → stdout（快速验证）

创建 `task/product_mysql_stdout.yaml`，适合调试 Python 脚本：

```yaml
kind: 转换
name: MySQL 商品数据转换输出到控制台

source:
  connection: mysql8_local
  query_file: get_products.sql

transform:
  mode: python
  script: t_product.py

load:
  type: stdout                    # 输出到控制台
  columns:
    - id
    - title_cn
    - title_en
    - title
    - price
    - stock
    - category_id
    - status
```

### 7.3 PG2 → stdout（独立测试）

创建 `task/product_pg_stdout.yaml`，不依赖 MySQL，适合单独调试：

```yaml
kind: 转换
name: PG2 商品数据转换输出到控制台

source:
  connection: pg2
  query_file: get_pg_products.sql

transform:
  mode: python
  script: t_product.py

load:
  type: stdout
  columns:
    - id
    - title_cn
    - title_en
    - title
    - price
    - stock
    - category_id
    - status
```

### 7.4 任务类型说明

| 字段 | 可选值 | 说明 |
|------|--------|------|
| `kind` | `转换` / `作业` | `转换`=单个 ETL，`作业`=多个转换编排（预留） |
| `source.connection` | dsn.json 中任意 `Name` | 数据从哪来 |
| `source.query_file` | SQL 文件名 | 抽取脚本 |
| `transform.mode` | `builtin` / `python` / `none` | 转换方式 |
| `load.type` | `csv` / `stdout` / `sql` | 数据写到哪 |
| `load.mode` | `insert` / `upsert` | SQL 写入模式 |

---

## 8. 运行 ETL 任务

### 8.1 快速验证（MySQL → stdout）

先不加 `-pg` 参数，只走控制台输出，确认 Python 脚本正确：

```bash
detl -task task/product_mysql_stdout.yaml
```

预期输出：

```
获取脚本目录: script
加载任务: kind=转换  name=MySQL 商品数据转换输出到控制台
Source: 连接=mysql8_local  驱动=mysql
转换模式: python  脚本=script\t_product.py
输出类型: stdout  列=[id, title_cn, title_en, title, price, ...]
id,title_cn,title_en,title,price,stock,category_id,status
3,环保数据线,Eco-friendly 数据线,环保数据线,11299.22,944,1,1
4,轻奢无线充电器,Affordable Luxury 无线充电器,轻奢无线充电器,5464.40,963,1,1
...
Pipeline 完成，共处理 300 行
```

### 8.2 完整 ETL（MySQL → PG2）

确认脚本无误后，执行完整链路写入 PG2：

```bash
detl -task task/product_sync_to_pg.yaml
```

预期输出：

```
获取脚本目录: script
加载任务: kind=转换  name=商品数据同步到 PG2
Source: 连接=mysql8_local  驱动=mysql
转换模式: python  脚本=script\t_product.py
输出类型: sql  目标=postgres.debug.etl_test_product  模式=upsert  批量=50
SQL Load: 写入 50 行到 "debug"."etl_test_product"
SQL Load: 写入 50 行到 "debug"."etl_test_product"
...（每批 50 行）
Pipeline 完成，共处理 300 行
```

### 8.3 验证 PG2 数据

```bash
psql -U postgres -d postgres -c "
  SELECT COUNT(*) AS 总数,
         ROUND(AVG(price)::numeric, 2) AS 平均价格,
         MIN(price) AS 最低价,
         MAX(price) AS 最高价
  FROM debug.etl_test_product;
"
```

### 8.4 幂等性：重复运行

`upsert` 模式保证任务可重复执行，不会产生重复数据：

```bash
# 再跑一次，不会报错
detl -task task/product_sync_to_pg.yaml
```

第二次运行同样是 `Pipeline 完成`，数据量不变。

---

## 9. 转换层灵活控制

### 9.1 价格倍数开关

在 `script/t_product.py` 顶部修改：

```python
# 当前：价格 × 2
PRICE_MULTIPLIER = 2.0

# 改为 1.0 即取消价格转换层，保留其他转换
PRICE_MULTIPLIER = 1.0

# 改为其他任意倍数
PRICE_MULTIPLIER = 1.5
```

修改后重新运行 ETL 即可生效（无需重启或编译）。

### 9.2 完全跳过转换层

```python
# 设为 True 则原样透传，不做任何数据处理
SKIP_TRANSFORM = True
```

透传模式下，Python 脚本只补充 PG 需要的额外字段（`title_cn`、`title_en`），不做任何业务转换。

### 9.3 场景说明

| 场景 | SKIP_TRANSFORM | PRICE_MULTIPLIER | 效果 |
|------|:---:|:---:|------|
| 全量转换 | False | 2.0 | 字段映射 + 价格翻倍 |
| 仅映射不调价 | False | 1.0 | 字段映射，价格不变 |
| 原样透传 | True | — | 只补齐字段名，数据不变 |

---

## 10. 完整文件结构

一个完整的 ETL 项目目录结构示例：

```
my_etl_project/
│
├── conf/
│   └── dsn.json                     # 数据源连接配置
│
├── script/
│   ├── get_products.sql             # MySQL 抽取脚本
│   ├── get_pg_products.sql          # PG 抽取脚本
│   └── t_product.py                 # Python 转换脚本
│
├── task/
│   ├── product_sync_to_pg.yaml      # MySQL → Python → PG2 完整链路
│   ├── product_mysql_stdout.yaml    # MySQL → Python → stdout
│   └── product_pg_stdout.yaml       # PG → Python → stdout
│
├── output/                          # CSV 输出目录（自动生成）
│
├── .env                             # 环境变量配置（可选）
└── .gitignore                       # 版本控制忽略规则
```

### 设计原则

1. **配置与代码分离**：`conf/` 放连接配置，`script/` 放业务脚本，`task/` 放任务编排
2. **SQL 只做抽取**：不写转换逻辑，保持 SQL 纯净
3. **Python 做转换**：所有清洗、映射、计算在 Python 中完成
4. **YAML 做编排**：声明式定义数据流，不改代码即可调整链路
5. **幂等写入**：upsert 模式保证多次运行结果一致

---

## 附录：命令速查

```bash
# 工具安装
go install github.com/iotames/detl/cmd/detl@latest
go install github.com/iotames/detl/cmd/seed@latest

# 数据源管理
seed -pg                   # PG2 建空表
seed                       # MySQL 填充 300 条演示数据
seed -count 500            # 自定义 500 条

# ETL 执行
detl -task task/product_mysql_stdout.yaml    # 仅控制台输出
detl -task task/product_sync_to_pg.yaml       # 完整写入 PG

# 环境变量覆盖
CONF_DIR=my_conf SCRIPT_DIR=my_script detl -task my_task.yaml
```
