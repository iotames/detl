package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTask_Transform(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
kind: 转换
name: 测试任务
source:
  connection: dev_pg
  query_file: test.sql
transform:
  mode: builtin
load:
  type: csv
  file: out.csv
  columns: [id, name]
`
	fpath := filepath.Join(dir, "test_task.yaml")
	if err := os.WriteFile(fpath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadTask(fpath)
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}
	if cfg.Kind != "转换" {
		t.Errorf("kind = %q, want 转换", cfg.Kind)
	}
	if cfg.Name != "测试任务" {
		t.Errorf("name = %q, want 测试任务", cfg.Name)
	}
	if cfg.Source.Connection != "dev_pg" {
		t.Errorf("source.connection = %q, want dev_pg", cfg.Source.Connection)
	}
	if cfg.Transform.Mode != "builtin" {
		t.Errorf("transform.mode = %q, want builtin", cfg.Transform.Mode)
	}
	if cfg.Load.Type != "csv" {
		t.Errorf("load.type = %q, want csv", cfg.Load.Type)
	}
	if len(cfg.Load.Columns) != 2 || cfg.Load.Columns[0] != "id" {
		t.Errorf("load.columns = %v, want [id name]", cfg.Load.Columns)
	}
	if !cfg.IsTransform() {
		t.Error("IsTransform() = false, want true")
	}
	if cfg.IsJob() {
		t.Error("IsJob() = true, want false")
	}
}

func TestLoadTask_Job(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
kind: 作业
name: 每日同步
tasks:
  - task: step1.yaml
  - task: step2.yaml
`
	fpath := filepath.Join(dir, "test_job.yaml")
	if err := os.WriteFile(fpath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadTask(fpath)
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}
	if cfg.Kind != "作业" {
		t.Errorf("kind = %q, want 作业", cfg.Kind)
	}
	if len(cfg.Tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(cfg.Tasks))
	}
	if !cfg.IsJob() {
		t.Error("IsJob() = false, want true")
	}
	if cfg.IsTransform() {
		t.Error("IsTransform() = true, want false")
	}
}

func TestLoadTask_UnknownKind(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `kind: 未知类型`
	fpath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(fpath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadTask(fpath)
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}

func TestLoadTask_FileNotFound(t *testing.T) {
	_, err := LoadTask("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
