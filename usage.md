# detl 使用指南

## 概述

detl 是一个命令行 ETL 工具，通过**环境变量 + 配置文件 + 脚本**驱动，无需修改源代码。

---

## 安装

```bash
go build -o main.exe ./main
```

---

## 环境变量配置

通过环境变量控制完整的 ETL 流程：

| 模块 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| **Source** | `CONF_DIR` | `conf` | 配置目录（存放 dsn.json） |
| | `SCRIPT_DIR` | `script` | ETL 业务脚本目录 |
| | `DB_DRIVER` | `postgres` | 数据库驱动：`postgres` / `mysql` |
| | `SCRIPT_FILE` | `e_detl_users.sql` | 抽取脚本文件名 |
| | `ACTIVE_DSN` | （PG 默认） | 默认 DSN 连接字符串 |
| **Transform** | `TRANSFORM_MODE` | `builtin` | 转换模式：`builtin` / `python` / `none` |
| | `TRANSFORM_SCRIPT` | `t_users.py` | Python 转换脚本（`python` 模式） |
| **Load** | `LOAD_TYPE` | `csv` | 输出类型：`csv` / `stdout` |
| | `OUTPUT_DIR` | `output` | 输出目录 |
| | `OUTPUT_FILE` | `etl_output.csv` | 输出文件名 |
| | `OUTPUT_COLUMNS` | `id,full_name,...` | CSV 列名（逗号分隔） |

---

## Source（数据抽取）

### SQL

```bash
DB_DRIVER=postgres SCRIPT_FILE=e_getusers.sql ./main.exe
DB_DRIVER=mysql    SCRIPT_FILE=e_detl_users.sql ./main.exe
```

程序从 `CONF_DIR/dsn.json` 中查找与 `DB_DRIVER` 匹配的 DSN。

### 文件（CSV/JSON）— 📋 待实现

---

## Transform（数据转换）

### 模式一：内置转换（builtin）

默认模式。自动执行：姓名拼接、邮箱小写、NULL age → 0、日期格式化。

```bash
TRANSFORM_MODE=builtin ./main.exe
```

### 模式二：Python 脚本（python）

启动一个常驻 Python 子进程，逐行处理数据。脚本通过 stdin/stdout 以 JSON 格式通信。

```bash
TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py ./main.exe
```

#### Python 脚本接口

| 方向 | 格式 | 说明 |
|---|---|---|
| stdin（输入） | 每行一个 JSON | 原始数据行 |
| stdout（输出） | 每行一个 JSON | 转换后的数据行 |
| stdout | `null` | 跳过该行（不输出） |
| stderr | 任意 | 错误日志透传 |

#### 示例脚本

`main/script/t_users.py`：

```python
import sys, json

for line in sys.stdin:
    row = json.loads(line.strip())
    # 转换逻辑
    row["full_name"] = f"{row.get('first_name','')} {row.get('last_name','')}".strip()
    row["email"] = (row.get("email") or "").lower()
    if row.get("age") is None:
        row["age"] = 0
    # 输出结果（必须 flush）
    print(json.dumps(row), flush=True)
```

### 模式三：透传（none）

不对数据做任何转换，原始数据直接输出：

```bash
TRANSFORM_MODE=none ./main.exe
```

---

## Load（数据载入）

### CSV 文件

```bash
LOAD_TYPE=csv OUTPUT_DIR=output OUTPUT_FILE=result.csv ./main.exe
```

CSV 列名通过 `OUTPUT_COLUMNS` 指定。未指定时按 map key 排序输出。

### 控制台输出

```bash
LOAD_TYPE=stdout ./main.exe
```

以 CSV 格式打印到终端，适用于调试和管道。

---

## 完整示例

### 示例 1：Postgres → 内置转换 → CSV

```bash
CONF_DIR=main/conf SCRIPT_DIR=main/script \
  DB_DRIVER=postgres SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=builtin \
  LOAD_TYPE=csv OUTPUT_FILE=pg_users.csv \
  ./main.exe
```

### 示例 2：MySQL → Python 转换 → Stdout

```bash
CONF_DIR=main/conf SCRIPT_DIR=main/script \
  DB_DRIVER=mysql SCRIPT_FILE=e_detl_users.sql \
  TRANSFORM_MODE=python TRANSFORM_SCRIPT=t_users.py \
  LOAD_TYPE=stdout \
  ./main.exe
```

---

## 目录结构

```
detl/
├── conf/                   # 配置文件目录
├── script/                 # ETL 业务脚本
│   ├── e_*.sql            # 抽取脚本
│   └── t_*.py             # 转换脚本
├── main/                   # 程序入口
│   ├── main.go
│   ├── main_func.go
│   ├── conf/dsn.json      # 🔒 本地 DSN 配置
│   └── script/             # 🔒 本地脚本
├── internal/               # 核心库
│   ├── engine/             # Pipeline 编排
│   ├── source/             # 数据抽取
│   ├── transform/          # 数据转换
│   └── load/               # 数据载入
└── output/                 # 输出文件
```
