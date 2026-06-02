package transform

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// PythonConfig Python 脚本转换配置
type PythonConfig struct {
	ScriptPath string            // Python 脚本文件路径
	Env        map[string]string // 附加环境变量（注入子进程）
}

type pyscript struct {
	cfg     PythonConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	mu      sync.Mutex
	started bool
}

// NewPython 创建 Python 脚本转换器
func NewPython(cfg PythonConfig) Transformer {
	return &pyscript{cfg: cfg}
}

func (t *pyscript) Transform(row map[string]any) ([]map[string]any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.started {
		if err := t.start(); err != nil {
			return nil, err
		}
	}

	// 发送行到 Python stdin
	data, err := json.Marshal(row)
	if err != nil {
		return nil, fmt.Errorf("python transform marshal: %w", err)
	}
	if _, err := fmt.Fprintln(t.stdin, string(data)); err != nil {
		return nil, fmt.Errorf("python transform write: %w", err)
	}

	// 读取 Python stdout 结果
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("python transform read: %w", err)
	}

	line = trimNewline(line)
	if line == "" || line == "null" {
		return nil, nil
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, fmt.Errorf("python transform unmarshal: %w", err)
	}
	return []map[string]any{result}, nil
}

func (t *pyscript) start() error {
	// 尝试 python 命令（Windows 上 python3 可能是商店 stub）
	cmdName := "python"
	if _, err := exec.LookPath("python"); err != nil {
		cmdName = "python3"
	}
	t.cmd = exec.Command(cmdName, t.cfg.ScriptPath)

	// 注入环境变量：继承父进程 + 附加变量
	t.cmd.Env = os.Environ()
	for k, v := range t.cfg.Env {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("python stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("python stdout pipe: %w", err)
	}
	t.reader = bufio.NewReader(stdout)

	t.cmd.Stderr = os.Stderr

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("python start: %w", err)
	}
	t.started = true
	return nil
}

func trimNewline(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s
}
