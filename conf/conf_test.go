package conf

import (
	"os"
	"path/filepath"
	"testing"

	pkgdsn "github.com/iotames/easydb/dsn"
)

func writeDSNFile(t *testing.T, dir, content string) {
	t.Helper()
	fpath := filepath.Join(dir, "dsn.json")
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestDSNGroup_GetDSNByName 测试 DSN 分组按连接名查找
func TestDSNGroup_GetDSNByName(t *testing.T) {
	dir := t.TempDir()
	writeDSNFile(t, dir, `{
		"DsnList": [
			{"Name": "dev_pg", "DriverName": "postgres", "Dsn": "user=postgres dbname=test"},
			{"Name": "dev_ms", "DriverName": "mysql", "Dsn": "root:root@tcp(127.0.0.1:3306)/test"}
		]
	}`)

	fpath := filepath.Join(dir, "dsn.json")
	dsnconf := pkgdsn.NewDsnConf(fpath)
	var dgp pkgdsn.DsnGroup
	if err := dsnconf.GetDsnGroup(&dgp); err != nil {
		t.Fatalf("GetDsnGroup failed: %v", err)
	}

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
	writeDSNFile(t, dir, `{"DsnList": [{"Name": "dev_pg", "DriverName": "pg", "Dsn": "..."}]}`)

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

	_, ok := dgp.GetDSNByName("pg")
	if ok {
		t.Error("GetDSNByName on old format = true, want false")
	}
	if len(dgp.DsnList) != 1 {
		t.Fatalf("len(DsnList) = %d, want 1", len(dgp.DsnList))
	}
}

// TestAppendNamedDsn_BackwardCompat 测试 AppendDsn 向下兼容
func TestAppendNamedDsn_BackwardCompat(t *testing.T) {
	var dgp pkgdsn.DsnGroup
	if err := dgp.AppendNamedDsn("dev_pg", "postgres", "user=postgres dbname=b"); err != nil {
		t.Skipf("AppendNamedDsn 需要已注册的驱动: %v", err)
	}
	if dgp.DsnList[0].Name != "dev_pg" {
		t.Errorf("Name = %q, want dev_pg", dgp.DsnList[0].Name)
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
