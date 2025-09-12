package dsn

import (
	"database/sql"
	"fmt"

	"github.com/iotames/miniutils"
)

type DataSource struct {
	Code       string
	DriverName string
	Dsn        string
}

type DsnGroup struct {
	ActiveCode string
	DsnList    []DataSource
}

func (d DsnGroup) GetDefaultDSN() DataSource {
	if len(d.DsnList) == 0 {
		return DataSource{}
	}
	dsn := d.GetActiveDSN()
	if d.ActiveCode == "" || dsn.Code == "" {
		return d.DsnList[0]
	}
	return dsn
}

func (d DsnGroup) GetActiveDSN() DataSource {
	mp := d.getDsnMap()
	var ok bool
	var dsn DataSource
	if dsn, ok = mp[d.ActiveCode]; !ok {
		return DataSource{}
	}
	return dsn
}

func (d DsnGroup) getDsnMap() map[string]DataSource {
	mp := make(map[string]DataSource, len(d.DsnList))
	for _, dd := range d.DsnList {
		mp[dd.Code] = dd
	}
	return mp
}

func (d DsnGroup) HasDsn(dsn string) bool {
	hasd := false
	for _, dd := range d.DsnList {
		if dd.Dsn == dsn {
			hasd = true
			break
		}
	}
	return hasd
}

func (d DsnGroup) HasActive(dsnCode string) bool {
	mp := d.getDsnMap()
	_, ok := mp[dsnCode]
	if !ok {
		return false
	}
	return d.ActiveCode == dsnCode
}

func (d *DsnGroup) Active(dsnCode string) error {
	found := false
	for _, dd := range d.DsnList {
		if dd.Code == dsnCode {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("not found dsnCode: %s", dsnCode)
	}
	d.ActiveCode = dsnCode
	return nil
}

func (d *DsnGroup) AppendDsn(driverName, dsn string) error {
	drivers := sql.Drivers()
	if miniutils.GetIndexOf(driverName, drivers) == -1 {
		return fmt.Errorf("数据库驱动%s未注册。已注册的数据库驱动有：%v", driverName, drivers)
	}
	code := miniutils.Md5(dsn)
	ds := DataSource{Code: code, DriverName: driverName, Dsn: dsn}
	if len(d.DsnList) == 0 {
		d.ActiveCode = code
	}
	d.DsnList = append(d.DsnList, ds)
	return nil
}
