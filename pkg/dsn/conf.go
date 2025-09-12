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

type DsnConfData struct {
	ActiveDriverName string
	DsnList          []DataSource
}

type DsnConf struct {
	fpath    string
	dsnGroup *DsnGroup
	jsonfile *iofile.JsonConf
}

func NewDsnConf(fpath, dsn, drivername string) *DsnConf {
	dsnGroup := DsnGroup{
		activeDriver: drivername,
		DsnList:      []DataSource{{DriverName: drivername, Dsn: dsn}},
	}
	return &DsnConf{
		fpath:    fpath,
		jsonfile: iofile.NewJsonConf(""),
		dsnGroup: &dsnGroup,
	}
}

func (d DsnConf) getDsnConfData() DsnConfData {
	data := DsnConfData{d.dsnGroup.activeDriver, d.dsnGroup.DsnList}
	return data
}

func (d DsnConf) GetDsnGroup() *DsnGroup {
	return d.dsnGroup
}

func (d DsnConf) Save() error {
	data := d.getDsnConfData()
	return d.jsonfile.Save(data, d.fpath)
}

func (d *DsnConf) Read() error {
	data := DsnConfData{}
	err := d.jsonfile.Read(&data, d.fpath)
	if err != nil {
		return err
	}
	d.dsnGroup.activeDriver = data.ActiveDriverName
	d.dsnGroup.DsnList = data.DsnList
	return nil
}
