package conf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/iotames/detl/pkg"
	"github.com/iotames/miniutils"
)

var cf *Conf
var once sync.Once

func GetConf(confdir string) *Conf {
	once.Do(func() {
		cf = newConf(confdir)
	})
	return cf
}

func newConf(confdir string) *Conf {
	if err := miniutils.Mkdir(confdir); err != nil {
		panic(confdir)
	}
	return &Conf{
		dirPath: confdir,
		envMap:  make(map[string]string, 5),
	}
}

type Conf struct {
	dirPath string
	envMap  map[string]string
}

func (c *Conf) SetScriptDir(d string) error {
	var err error
	if err = miniutils.Mkdir(d); err != nil {
		return err
	}
	c.envMap["SCRIPT_DIR"] = d
	return err
}

func (c Conf) GetScriptDir() string {
	return c.envMap["SCRIPT_DIR"]
}

func (c Conf) saveFile(v any, filename string) error {
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

func (c Conf) readFile(v any, filename string) error {
	var b []byte
	var err error
	b, err = os.ReadFile(filepath.Join(c.dirPath, filename))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (c Conf) SetActiveDSN(dsn, driverName string) error {
	var err error
	var dsngp *pkg.DsnGroup
	filename := "dsn.json"
	fpath := filepath.Join(c.dirPath, filename)
	if !miniutils.IsPathExists(fpath) {
		dsngp = pkg.NewDsnConf(dsn, driverName)
		return c.saveFile(dsngp, filename)
	}
	dsngp = &pkg.DsnGroup{}
	err = c.readFile(dsngp, filename)
	if err != nil {
		return err
	}
	if !dsngp.HasDsn(dsn) {
		dsngp.AppendDsn(dsn, driverName)
	}
	if !dsngp.HasActive(driverName) {
		dsngp.Active(driverName)
		return c.saveFile(dsngp, filename)
	}
	return fmt.Errorf("dsn has actived")
}

func (c Conf) GetDSN(driverName string) string {
	dsngp := pkg.DsnGroup{}
	return dsngp.GetDSN(driverName)
}
