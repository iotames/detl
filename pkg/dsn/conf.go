package dsn

import (
	"sync"

	"github.com/iotames/detl/iofile"
)

var conf *DsnConf
var once sync.Once

func GetDsnConf(cf *DsnConf) *DsnConf {
	once.Do(func() {
		conf = cf
	})
	return conf
}

type DsnConfData = DsnGroup

type DsnConf struct {
	fpath    string
	jsonfile *iofile.JsonConf
}

// NewDsnConf 初始化数据源配置。
func NewDsnConf(fpath string) *DsnConf {
	return &DsnConf{
		fpath:    fpath,
		jsonfile: iofile.NewJsonConf(""),
	}
}

func (d DsnConf) GetDsnGroup(dgp *DsnGroup) error {
	data := DsnConfData{}
	err := d.jsonfile.Read(&data, d.fpath)
	if err != nil {
		return err
	}
	*dgp = data
	return err
}

func (d DsnConf) SaveDsnGroup(dgp DsnGroup) error {
	return d.jsonfile.Save(dgp, d.fpath)
}
