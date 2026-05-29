package conf

import (
	"os"
	"path/filepath"
	"testing"

	pkgdsn "github.com/iotames/easydb/dsn"
)

func writeDSNFile(t *testing.T, dir, content string) string {
	t.Helper()
	fpath := filepath.Join(dir, "dsn.json")
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return fpath
}

// TestDSNGroup_GetDSNByName 测试 DSN 分组按连接名查找
func TestDSNGroup_GetDSNByName(t *testing.T) {
	// 写 dsn.json
	dir := t.TempDir()
	writeDSNFile(t, dir, `{
		"DsnList": [
			{"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres dbname=test"},
			{"Name": "dev_ms", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/test"}
		]
	}`)

	// 直接读 dsn.json 解析 DsnGroup，绕过全局单例
	fpath := filepath.Join(dir, "dsn.json")
	dsnconf := pkgdsn.NewDsnConf(fpath) // 注意：pkgdsn 需要正确导入
	var dgp pkgdsn.DsnGroup
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		t.Fatalf("GetDsnGroup failed: %v", err)
	}

	// 测试按 Name 查找
	ds, ok := dgp.GetDSNByName("dev_pg")
	if !ok {
		t.Fatal("GetDSNByName(dev_pg) = false, want true")
	}
	if ds.DriverName != "postgres" {
		t.Errorf("driver = %q, want postgres", ds.DriverName)
	}
	if ds.Dsn != "user=postgres dbname=test" {
		t.Errorf("dsn = %q, want user=postgres dbname=test", ds.Dsn)
	}
}

// TestDSNGroup_GetDSNByName_NotFound 测试查找不存在的 Name
func TestDSNGroup_GetDSNByName_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeDSNFile(t, dir, `{"DsnList": [{"DriverName": "pg", "Dsn": "..."}]}`)

	fpath := filepath.Join(dir, "dsn.json")
	dsnconf := pkgdsn.NewDsnConf(fpath)
	var dgp pkgdsn.DsnGroup
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		t.Fatalf("GetDsnGroup failed: %v", err)
	}

	_, ok := dgp.GetDSNByName("nonexistent")
	if ok {
		t.Error("GetDSNByName(nonexistent) = true, want false")
	}
}

// TestDSNGroup_GetDSNByName_OldFormat 测试旧版 dsn.json（无 Name 字段）
func TestDSNGroup_GetDSNByName_OldFormat(t *testing.T) {
	dir := t.TempDir()
	writeDSNFile(t, dir, `{"DsnList": [{"DriverName": "pg", "Dsn": "user=postgres dbname=test"}]}`)

	fpath := filepath.Join(dir, "dsn.json")
	dsnconf := pkgdsn.NewDsnConf(fpath)
	var dgp pkgdsn.DsnGroup
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		t.Fatalf("GetDsnGroup failed: %v", err)
	}

	// 旧版无 Name，按 Name 找应失败
	_, ok := dgp.GetDSNByName("pg")
	if ok {
		t.Error("GetDSNByName on old format = true, want false")
	}

	// 但数据本身仍然存在
	if len(dgp.DsnList) != 1 {
		t.Fatalf("len(DsnList) = %d, want 1", len(dgp.DsnList))
	}
	if dgp.DsnList[0].DriverName != "pg" {
		t.Errorf("DriverName = %q, want pg", dgp.DsnList[0].DriverName)
	}
}

// TestAppendNamedDsn_BackwardCompat 测试 AppendDsn 向下兼容
func TestAppendNamedDsn_BackwardCompat(t *testing.T) {
	var dgp pkgdsn.DsnGroup

	// 旧方法：不传入 Name
	if err := dgp.AppendDsn("postgres", "user=postgres dbname=a"); err != nil {
		// 可能因为驱动未注册而失败，跳过
		t.Skipf("AppendDsn 需要已注册的驱动: %v", err)
	}

	// 新方法：传入 Name
	if err := dgp.AppendNamedDsn("dev_pg", "postgres", "user=postgres dbname=b"); err != nil {
		t.Skipf("AppendNamedDsn 需要已注册的驱动: %v", err)
	}

	if len(dgp.DsnList) != 2 {
		t.Fatalf("len(DsnList) = %d, want 2", len(dgp.DsnList))
	}
	// 第一个 DSN Name 应为空（旧方法）
	if dgp.DsnList[0].Name != "" {
		t.Errorf("first DSN Name = %q, want empty", dgp.DsnList[0].Name)
	}
	// 第二个 DSN Name 应为 dev_pg（新方法）
	if dgp.DsnList[1].Name != "dev_pg" {
		t.Errorf("second DSN Name = %q, want dev_pg", dgp.DsnList[1].Name)
	}
}

// TestLoadSystemConfig_FileExists 测试加载存在的 system.yaml
func TestLoadSystemConfig_FileExists(t *testing.T) {
	dir := t.TempDir()
	yamlContent := "script_dir: my_script\noutput_dir: my_output\n"
	if err := os.WriteFile(filepath.Join(dir, "system.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := newConf(dir)
	sys := c.LoadSystemConfig()
	if sys.ScriptDir != "my_script" {
		t.Errorf("ScriptDir = %q, want my_script", sys.ScriptDir)
	}
	if sys.OutputDir != "my_output" {
		t.Errorf("OutputDir = %q, want my_output", sys.OutputDir)
	}
}

// TestLoadSystemConfig_FileNotExist 测试 system.yaml 不存在时不报错
func TestLoadSystemConfig_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	c := newConf(dir)
	sys := c.LoadSystemConfig()
	if sys.ScriptDir != "" {
		t.Errorf("ScriptDir = %q, want empty", sys.ScriptDir)
	}
	if sys.OutputDir != "" {
		t.Errorf("OutputDir = %q, want empty", sys.OutputDir)
	}
}
