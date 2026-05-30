package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTaskPath_Absolute(t *testing.T) {
	absPath := filepath.Join(string(filepath.Separator)+"absolute", "path", "to", "task.yaml")
	if !filepath.IsAbs(absPath) {
		t.Skip("当前平台不支持该绝对路径格式")
	}
	path := resolveTaskPath(absPath, "/script")
	if path != absPath {
		t.Errorf("absolute path = %q, want %q", path, absPath)
	}
}

func TestResolveTaskPath_RelativeExists(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "mytask.yaml")
	if err := os.WriteFile(fpath, []byte("kind: 转换"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// 相对路径且文件存在，应直接返回
	path := resolveTaskPath("mytask.yaml", "/script")
	if path != "mytask.yaml" {
		t.Errorf("existing relative path = %q, want mytask.yaml", path)
	}
}

func TestResolveTaskPath_RelativeFallback(t *testing.T) {
	// 相对路径文件不存在，回退到 SCRIPT_DIR
	path := resolveTaskPath("notexist.yaml", "/my/script/dir")
	expected := filepath.Join("/my/script/dir", "notexist.yaml")
	if path != expected {
		t.Errorf("fallback path = %q, want %q", path, expected)
	}
}

func TestResolveTaskPath_RelativeWithDot(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "sub", "task.yaml")
	if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpath, []byte("kind: 转换"), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// ./sub/task.yaml 存在，应直接返回
	path := resolveTaskPath("./sub/task.yaml", "/script")
	if path != "./sub/task.yaml" {
		t.Errorf("dot-relative path = %q, want ./sub/task.yaml", path)
	}
}

func TestResolveTaskPath_ScriptDirPath(t *testing.T) {
	// 文件不存在于当前目录，但在 SCRIPT_DIR 下存在
	scriptDir := t.TempDir()
	fpath := filepath.Join(scriptDir, "task.yaml")
	if err := os.WriteFile(fpath, []byte("kind: 转换"), 0644); err != nil {
		t.Fatal(err)
	}

	// 在干净的临时目录中运行
	cleanDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(cleanDir)
	defer os.Chdir(origDir)

	path := resolveTaskPath("task.yaml", scriptDir)
	if path != fpath {
		t.Errorf("scriptDir fallback = %q, want %q", path, fpath)
	}
}
