package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// --- DSN 配置结构 ---

type DSNEntry struct {
	Code       string `json:"Code"`
	Name       string `json:"Name,omitempty"`
	DriverName string `json:"DriverName"`
	Dsn        string `json:"Dsn"`
}

type DSNConfig struct {
	ActiveCode string     `json:"ActiveCode"`
	DsnList    []DSNEntry `json:"DsnList"`
}

// --- 基础数据定义 ---

var categories = []struct {
	Name     string
	ParentID int64
}{
	{"电子产品", 0},
	{"手机通讯", 1},
	{"电脑办公", 1},
	{"服装鞋帽", 0},
	{"男装", 4},
	{"女装", 4},
	{"家居生活", 0},
	{"厨房用品", 7},
	{"食品饮料", 0},
	{"图书音像", 0},
}

// categoryID -> 可能的商品类型名
var productTypes = map[int64][]string{
	1: {"智能手表", "蓝牙耳机", "充电宝", "数据线", "无线充电器", "智能音箱", "运动相机", "电子阅读器"},
	2: {"智能手机", "5G手机", "拍照手机", "游戏手机", "折叠屏手机", "商务手机", "三防手机", "老人手机"},
	3: {"笔记本电脑", "平板电脑", "机械键盘", "无线鼠标", "显示器", "USB集线器", "笔记本支架", "电竞耳机"},
	4: {"休闲鞋", "运动帽", "皮带", "钱包", "背包", "太阳镜", "手表", "袜子套装"},
	5: {"男士T恤", "男士衬衫", "男士牛仔裤", "男士夹克", "男士西服", "男士休闲裤", "男士羽绒服", "男士运动鞋"},
	6: {"女士连衣裙", "女士外套", "女士手提包", "女士高跟鞋", "女士牛仔裤", "女士毛衣", "女士围巾", "女士运动鞋"},
	7: {"台灯", "抱枕", "花瓶", "香薰机", "地毯", "收纳盒", "挂画", "懒人沙发"},
	8: {"不粘锅", "保温杯", "菜刀套装", "保鲜盒", "砧板", "筷子套装", "电饭煲", "料理机"},
	9: {"坚果礼盒", "巧克力", "咖啡豆", "有机绿茶", "蜂蜜", "即食燕窝", "红酒", "进口饼干"},
	10: {"编程入门", "设计模式", "数据结构", "算法导论", "人生哲理", "散文集", "科幻小说", "历史读物"},
}

var adjectives = []string{
	"极简", "轻奢", "北欧风", "智能", "经典", "时尚", "便携", "多功能",
	"高端", "环保", "专业", "简约", "复古", "创意", "舒适", "耐用",
	"超薄", "大容量", "降噪", "高清", "无线", "触控", "防水",
}

var tagNames = []string{
	"热销", "新品", "推荐", "限时优惠", "包邮",
	"进口", "环保", "有机", "手工", "定制",
	"经典", "时尚", "简约", "奢华", "轻奢",
	"便携", "耐用", "防水", "智能", "超值",
}

var descriptions = []string{
	"品质保证，值得信赖。",
	"全新升级，体验更佳。",
	"热销爆款，限时抢购。",
	"精选好物，品质生活。",
	"厂家直销，正品保障。",
	"口碑之选，好评如潮。",
	"时尚设计，彰显品味。",
	"实用至上，性价比之选。",
	"送礼佳品，精美包装。",
	"经久耐用，品质之选。",
}

// 各分类权重（和=300），按比例分配商品数
var catWeights = []int{30, 40, 40, 30, 35, 35, 25, 30, 15, 20}

// --- 主流程 ---

func main() {
	productCount := flag.Int("count", 300, "商品填充数量")
	usePG := flag.Bool("pg", false, "对 pg2 执行 DDL 建表（不填充数据）")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	dsnPath := findDSNPath()
	log.Printf("读取 DSN 配置: %s", dsnPath)

	data, err := os.ReadFile(dsnPath)
	if err != nil {
		log.Fatalf("读取 dsn.json 失败: %v", err)
	}

	var cfg DSNConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("解析 dsn.json 失败: %v", err)
	}

	code := "dev_mysql"
	driver := "mysql"
	if *usePG {
		code = "dev_pg"
		driver = "postgres"
	}

	var targetDSN string
	for _, entry := range cfg.DsnList {
		if entry.Code == code || entry.Name == code {
			targetDSN = entry.Dsn
			break
		}
	}
	if targetDSN == "" {
		log.Fatalf("未找到 %s DSN 配置", code)
	}

	db, err := sql.Open(driver, targetDSN)
	if err != nil {
		log.Fatalf("连接 %s 失败: %v", driver, err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Ping %s 失败: %v", driver, err)
	}
	log.Printf("%s(%s) 连接成功", driver, code)

	if *usePG {
		// PG 模式：仅执行 DDL，不填充数据
		execPGDDL(db)
		log.Println("PG 建表完成")
		return
	}

	// MySQL 模式：建表 + 填充数据
	execDDL(db)
	clearTables(db)

	categoryIDs := seedCategories(db)
	log.Printf("分类: 写入 %d 条", len(categoryIDs))

	tagIDs := seedTags(db)
	log.Printf("标签: 写入 %d 条", len(tagIDs))

	seedProducts(db, categoryIDs, tagIDs, *productCount)
}

func findDSNPath() string {
	candidates := []string{
		"conf/dsn.json",
		"cmd/detl/conf/dsn.json",
		"../cmd/detl/conf/dsn.json",
		"../../cmd/detl/conf/dsn.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	for range 3 {
		candidate := filepath.Join(dir, "cmd/detl/conf/dsn.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return "cmd/detl/conf/dsn.json"
}

// --- DDL 执行 ---

func execDDL(db *sql.DB) {
	ddl := `
	CREATE TABLE IF NOT EXISTS product_category (
		id         BIGINT       NOT NULL AUTO_INCREMENT,
		name       VARCHAR(100) NOT NULL DEFAULT '' COMMENT '分类名称',
		parent_id  BIGINT       NOT NULL DEFAULT 0  COMMENT '父分类ID',
		sort_order INT          NOT NULL DEFAULT 0  COMMENT '排序',
		status     TINYINT      NOT NULL DEFAULT 1  COMMENT '状态:1启用0禁用',
		created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		INDEX idx_parent_id (parent_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品分类';

	CREATE TABLE IF NOT EXISTS product_tag (
		id         BIGINT      NOT NULL AUTO_INCREMENT,
		name       VARCHAR(50) NOT NULL DEFAULT '' COMMENT '标签名称',
		created_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY uk_name (name)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品标签';

	CREATE TABLE IF NOT EXISTS etl_test_product (
		id          BIGINT        NOT NULL AUTO_INCREMENT,
		title       VARCHAR(255)  NOT NULL DEFAULT ''  COMMENT '商品标题',
		category_id BIGINT        NOT NULL DEFAULT 0   COMMENT '分类ID',
		price       DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '价格',
		stock       INT           NOT NULL DEFAULT 0   COMMENT '库存',
		description TEXT                                COMMENT '商品描述',
		status      TINYINT       NOT NULL DEFAULT 1   COMMENT '状态:1上架0下架',
		is_deleted  TINYINT       NOT NULL DEFAULT 0   COMMENT '软删除标记',
		created_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		INDEX idx_category_id (category_id),
		INDEX idx_status (status),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='测试商品信息表';

	CREATE TABLE IF NOT EXISTS product_tag_x (
		id         BIGINT NOT NULL AUTO_INCREMENT,
		product_id BIGINT NOT NULL DEFAULT 0 COMMENT '商品ID',
		tag_id     BIGINT NOT NULL DEFAULT 0 COMMENT '标签ID',
		PRIMARY KEY (id),
		UNIQUE KEY uk_product_tag (product_id, tag_id),
		INDEX idx_tag_id (tag_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品标签关系表';
	`

	for _, stmt := range splitSQL(ddl) {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("建表失败: %v\nSQL: %s", err, stmt)
		}
	}
	log.Println("建表完成")
}

func execPGDDL(db *sql.DB) {
	if _, err := db.Exec("CREATE SCHEMA IF NOT EXISTS debug"); err != nil {
		log.Fatalf("PG 创建 schema 失败: %v", err)
	}
	ddl := `
	CREATE TABLE IF NOT EXISTS debug.product_category (
		id         BIGSERIAL    PRIMARY KEY,
		name       VARCHAR(100) NOT NULL DEFAULT '',
		parent_id  BIGINT       NOT NULL DEFAULT 0,
		sort_order INT          NOT NULL DEFAULT 0,
		status     SMALLINT     NOT NULL DEFAULT 1,
		created_at TIMESTAMP    NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_debug_product_category_parent_id ON debug.product_category(parent_id);

	CREATE TABLE IF NOT EXISTS debug.product_tag (
		id         BIGSERIAL   PRIMARY KEY,
		name       VARCHAR(50) NOT NULL DEFAULT '',
		created_at TIMESTAMP   NOT NULL DEFAULT NOW()
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_debug_product_tag_name ON debug.product_tag(name);

	CREATE TABLE IF NOT EXISTS debug.etl_test_product (
		id          BIGSERIAL       PRIMARY KEY,
		title_cn    VARCHAR(255)    NOT NULL DEFAULT '',
		title_en    VARCHAR(255)    NOT NULL DEFAULT '',
		title       VARCHAR(255)    NOT NULL DEFAULT '',
		category_id BIGINT          NOT NULL DEFAULT 0,
		price       NUMERIC(10,2)   NOT NULL DEFAULT 0.00,
		stock       INT             NOT NULL DEFAULT 0,
		description TEXT,
		status      SMALLINT        NOT NULL DEFAULT 1,
		is_deleted  SMALLINT        NOT NULL DEFAULT 0,
		created_at  TIMESTAMP       NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMP       NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_debug_product_category_id ON debug.etl_test_product(category_id);
	CREATE INDEX IF NOT EXISTS idx_debug_product_status ON debug.etl_test_product(status);
	CREATE INDEX IF NOT EXISTS idx_debug_product_created_at ON debug.etl_test_product(created_at);

	CREATE TABLE IF NOT EXISTS debug.product_tag_x (
		id         BIGSERIAL  PRIMARY KEY,
		product_id BIGINT     NOT NULL DEFAULT 0,
		tag_id     BIGINT     NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_debug_product_tag_x_product_tag ON debug.product_tag_x(product_id, tag_id);
	CREATE INDEX IF NOT EXISTS idx_debug_product_tag_x_tag_id ON debug.product_tag_x(tag_id);
	`

	for _, stmt := range splitSQL(ddl) {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("PG 建表失败: %v\nSQL: %s", err, stmt)
		}
	}
}

func clearTables(db *sql.DB) {
	tables := []string{"product_tag_x", "etl_test_product", "product_tag", "product_category"}
	for _, t := range tables {
		if _, err := db.Exec("DELETE FROM " + t); err != nil {
			log.Printf("清空表 %s 失败(可能不存在): %v", t, err)
		}
	}
	// Reset auto_increment
	for _, t := range tables {
		db.Exec("ALTER TABLE " + t + " AUTO_INCREMENT = 1")
	}
}

// --- 数据填充 ---

func seedCategories(db *sql.DB) []int64 {
	stmt, err := db.Prepare("INSERT INTO product_category (name, parent_id, sort_order, status, created_at) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("准备分类插入语句失败: %v", err)
	}
	defer stmt.Close()

	ids := make([]int64, 0, len(categories))
	now := time.Now()
	for i, c := range categories {
		createdAt := now.Add(-time.Duration(len(categories)-i) * time.Hour)
		r, err := stmt.Exec(c.Name, c.ParentID, i, 1, createdAt)
		if err != nil {
			log.Fatalf("插入分类 %q 失败: %v", c.Name, err)
		}
		id, _ := r.LastInsertId()
		ids = append(ids, id)
	}
	return ids
}

func seedTags(db *sql.DB) []int64 {
	stmt, err := db.Prepare("INSERT INTO product_tag (name, created_at) VALUES (?, ?)")
	if err != nil {
		log.Fatalf("准备标签插入语句失败: %v", err)
	}
	defer stmt.Close()

	ids := make([]int64, 0, len(tagNames))
	now := time.Now()
	for i, name := range tagNames {
		createdAt := now.Add(-time.Duration(len(tagNames)-i) * time.Hour)
		r, err := stmt.Exec(name, createdAt)
		if err != nil {
			log.Fatalf("插入标签 %q 失败: %v", name, err)
		}
		id, _ := r.LastInsertId()
		ids = append(ids, id)
	}
	return ids
}

func seedProducts(db *sql.DB, categoryIDs, tagIDs []int64, totalCount int) {
	pStmt, err := db.Prepare(`INSERT INTO etl_test_product
		(title, category_id, price, stock, description, status, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatalf("准备商品插入语句失败: %v", err)
	}
	defer pStmt.Close()

	txStmt, err := db.Prepare("INSERT INTO product_tag_x (product_id, tag_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE product_id=product_id")
	if err != nil {
		log.Fatalf("准备标签关系插入语句失败: %v", err)
	}
	defer txStmt.Close()

	// 按权重分配商品到各分类
	type dist struct {
		catID int64
		count int
	}
	distribution := make([]dist, len(categoryIDs))
	totalWeight := 0
	for _, w := range catWeights {
		totalWeight += w
	}
	allocated := 0
	for i, id := range categoryIDs {
		w := catWeights[i]
		c := totalCount * w / totalWeight
		if c < 1 && totalCount >= len(categoryIDs) {
			c = 1
		}
		distribution[i] = dist{catID: id, count: c}
		allocated += c
	}
	// 补足余数
	for i := 0; allocated < totalCount && i < len(distribution); i++ {
		distribution[i].count++
		allocated++
	}

	productIndex := 0
	now := time.Now()
	for _, d := range distribution {
		for i := 0; i < d.count; i++ {
			productIndex++
			title := genProductTitle(d.catID, productIndex)
			price := genPrice()
			stock := rand.Intn(1000)
			desc := descriptions[rand.Intn(len(descriptions))]
			status := int64(1)
			if rand.Intn(10) == 0 {
				status = 0 // 10% 下架
			}
			isDeleted := int64(0)
			if rand.Intn(50) == 0 {
				isDeleted = 1 // 2% 软删除
			}
			createdAt := now.Add(-time.Duration(rand.Intn(365*24)) * time.Hour)
			updatedAt := createdAt.Add(time.Duration(rand.Intn(72)) * time.Hour)

			r, err := pStmt.Exec(title, d.catID, price, stock, desc, status, isDeleted, createdAt, updatedAt)
			if err != nil {
				log.Fatalf("插入商品失败 (idx=%d): %v", productIndex, err)
			}
			productID, _ := r.LastInsertId()

			// 标签关系：每商品 1~5 个随机标签
			nTags := 1 + rand.Intn(5)
			shuffled := make([]int64, len(tagIDs))
			copy(shuffled, tagIDs)
			rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			for j := 0; j < nTags && j < len(shuffled); j++ {
				if _, err := txStmt.Exec(productID, shuffled[j]); err != nil {
					log.Printf("插入标签关系失败 (pid=%d, tid=%d): %v", productID, shuffled[j], err)
				}
			}
		}
	}

	log.Printf("商品: 写入 %d 条，标签关系已同步", productIndex)
}

func genProductTitle(catID int64, idx int) string {
	types := productTypes[catID]
	if len(types) == 0 {
		return fmt.Sprintf("商品_%d", idx)
	}
	base := types[idx%len(types)]
	adj := adjectives[rand.Intn(len(adjectives))]
	return adj + base
}

func genPrice() float64 {
	base := rand.Intn(9990) + 10
	return float64(base) + float64(rand.Intn(100))/100
}

// --- 工具函数 ---

func splitSQL(text string) []string {
	parts := strings.Split(text, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lines := strings.Split(p, "\n")
		var cleaned []string
		for _, l := range lines {
			lt := strings.TrimSpace(l)
			if strings.HasPrefix(lt, "--") {
				continue
			}
			cleaned = append(cleaned, l)
		}
		stmt := strings.TrimSpace(strings.Join(cleaned, "\n"))
		if stmt != "" {
			result = append(result, stmt)
		}
	}
	return result
}
