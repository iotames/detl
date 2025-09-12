package conf

import (
	"fmt"
	"path/filepath"
	"sync"

	pkgdsn "github.com/iotames/detl/pkg/dsn"
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

func (c Conf) SetActiveDSN(dsn, driverName string) error {
	var err error
	var dsngp *pkgdsn.DsnGroup
	filename := "dsn.json"
	fpath := filepath.Join(c.dirPath, filename)
	dsnconf := pkgdsn.NewDsnConf(fpath, dsn, driverName)
	pkgdsn.GetDsnConf(dsnconf)

	if !miniutils.IsPathExists(fpath) {
		return dsnconf.Save()
	}
	err = dsnconf.Read()
	if err != nil {
		return err
	}
	dsngp = dsnconf.GetDsnGroup()
	if !dsngp.HasDsn(dsn) {
		dsngp.AppendDsn(dsn, driverName)
	}
	if !dsngp.HasActive(driverName) {
		err = dsngp.Active(driverName)
		if err != nil {
			return err
		}
		return dsnconf.Save()
	}
	// return fmt.Errorf("dsn has actived")
	return nil
}

func (c Conf) GetDefaultDSN() (driverName, dsn string) {
	dsnconf := pkgdsn.GetDsnConf(nil)
	fmt.Printf("---dsnconf.GetDsnGroup----(%+v)-----\n", dsnconf.GetDsnGroup())
	return dsnconf.GetDsnGroup().GetDefaultDSN()
}

func (c Conf) GetDSN(driverName string) string {
	dsngp := pkgdsn.DsnGroup{}
	return dsngp.GetDSN(driverName)
}
