package iofile

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type JsonConf struct {
	dirPath string
}

func NewJsonConf(dirPath string) *JsonConf {
	return &JsonConf{dirPath: dirPath}
}

func (c JsonConf) Save(v any, filename string) error {
	var err error
	var b []byte
	b, err = json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	var f io.WriteCloser
	fpath := filepath.Join(c.dirPath, filename)
	f, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func (c JsonConf) Read(v any, filename string) error {
	var b []byte
	var err error
	b, err = os.ReadFile(filepath.Join(c.dirPath, filename))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
