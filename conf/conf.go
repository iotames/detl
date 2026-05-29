package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	pkgdsn "github.com/iotames/easydb/dsn"
	"github.com/iotames/miniutils"
	"gopkg.in/yaml.v3"
)

// SystemConfig 系统配置（system.yaml），环境变量可覆盖
type SystemConfig struct {
	ScriptDir string `yaml:"script_dir"`
	OutputDir string `yaml:"output_dir"`
}

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

func (c Conf) GetScriptFilePath(fname string) string {
	return filepath.Join(c.GetScriptDir(), fname)
}

func (c Conf) InitDSN(driverName, dsn string) (dsnconf *pkgdsn.DsnConf, err error, isInit bool) {
	filename := "dsn.json"
	dgp := &pkgdsn.DsnGroup{}
	fpath := filepath.Join(c.dirPath, filename)
	dsnconf = pkgdsn.NewDsnConf(fpath)
	pkgdsn.GetDsnConf(dsnconf)
	if !miniutils.IsPathExists(fpath) {
		fmt.Println("create conf file:", fpath)
		err = dgp.AppendDsn(driverName, dsn)
		if err != nil {
			return dsnconf, err, true
		}
		err = dsnconf.SaveDsnGroup(*dgp)
		return dsnconf, err, true
	}
	return dsnconf, err, false
}

// LoadSystemConfig 加载 system.yaml（可选），不存在则返回零值
func (c Conf) LoadSystemConfig() SystemConfig {
	var sysCfg SystemConfig
	fpath := filepath.Join(c.dirPath, "system.yaml")
	data, err := os.ReadFile(fpath)
	if err != nil {
		return sysCfg
	}
	if err := yaml.Unmarshal(data, &sysCfg); err != nil {
		fmt.Printf("解析 system.yaml 失败: %v\n", err)
	}
	return sysCfg
}

// GetDSNByName 按连接名查找数据源
func (c Conf) GetDSNByName(name string) (pkgdsn.DataSource, bool) {
	dgp, err := c.GetDSNGroup()
	if err != nil {
		return pkgdsn.DataSource{}, false
	}
	return dgp.GetDSNByName(name)
}

// GetDSNGroup 读取完整数据源分组（解析 dsn.json）
func (c Conf) GetDSNGroup() (*pkgdsn.DsnGroup, error) {
	dsnconf := pkgdsn.GetDsnConf(nil)
	if dsnconf == nil {
		return nil, fmt.Errorf("DSN 配置未初始化")
	}
	dgp := &pkgdsn.DsnGroup{}
	if err := dsnconf.GetDsnGroup(dgp); err != nil {
		return nil, fmt.Errorf("读取 DSN 配置失败: %w", err)
	}
	return dgp, nil
}

// GetDSNByDriver 按驱动名查找第一个匹配的数据源（兼容旧逻辑）
func (c Conf) GetDSNByDriver(driver string) (pkgdsn.DataSource, bool) {
	dsnconf := pkgdsn.GetDsnConf(nil)
	if dsnconf == nil {
		return pkgdsn.DataSource{}, false
	}
	dgp := &pkgdsn.DsnGroup{}
	if err := dsnconf.GetDsnGroup(dgp); err != nil {
		return pkgdsn.DataSource{}, false
	}
	for _, ds := range dgp.DsnList {
		if ds.DriverName == driver {
			return ds, true
		}
	}
	return pkgdsn.DataSource{}, false
}

func (c Conf) SetActiveDSN(driverName, dsn string) error {
	dsnconf, err, isInit := c.InitDSN(driverName, dsn)
	if err != nil {
		return err
	}
	if isInit {
		return err
	}
	dgp := &pkgdsn.DsnGroup{}
	err = dsnconf.GetDsnGroup(dgp)
	if err != nil {
		return err
	}
	dsnCode := miniutils.Md5(dsn)
	if dgp.HasActive(dsnCode) {
		return nil
	}
	if !dgp.HasDsn(dsn) {
		dgp.AppendDsn(driverName, dsn)
	}
	err = dgp.Active(dsnCode)
	if err != nil {
		return err
	}
	return dsnconf.SaveDsnGroup(*dgp)
}
