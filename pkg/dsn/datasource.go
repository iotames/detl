package dsn

import "fmt"

type DataSource struct {
	DriverName string
	Dsn        string
}

type DsnGroup struct {
	activeDriver string
	DsnList      []DataSource
}

func (d DsnGroup) GetDefaultDSN() (driverName, dsn string) {
	if len(d.DsnList) == 0 {
		return "", ""
	}
	driverName, dsn = d.GetActiveDSN()
	if d.activeDriver == "" || driverName == "" {
		return d.DsnList[0].DriverName, d.DsnList[0].Dsn
	}
	return driverName, dsn
}

func (d DsnGroup) GetActiveDSN() (driverName, dsn string) {
	mp := d.getDsnMap()
	var ok bool
	if dsn, ok = mp[d.activeDriver]; !ok {
		return "", ""
	}
	return d.activeDriver, dsn
}

func (d DsnGroup) getDsnMap() map[string]string {
	mp := make(map[string]string, len(d.DsnList))
	for _, dd := range d.DsnList {
		mp[dd.DriverName] = dd.Dsn
	}
	return mp
}

func (d DsnGroup) GetDSN(driverName string) string {
	ddd := ""
	for _, dd := range d.DsnList {
		if dd.DriverName == driverName {
			ddd = dd.Dsn
			break
		}
	}
	return ddd
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

func (d DsnGroup) HasActive(driverName string) bool {
	return d.activeDriver == driverName
}

func (d *DsnGroup) Active(driverName string) error {
	if d.activeDriver == driverName {
		return fmt.Errorf("driverName %s has actived", driverName)
	}
	found := false
	for _, dd := range d.DsnList {
		if dd.DriverName == driverName {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("not found driverName: %s", driverName)
	}
	d.activeDriver = driverName
	return nil
}

func (d *DsnGroup) AppendDsn(dsn, driverName string) {
	d.DsnList = append(d.DsnList, DataSource{Dsn: dsn, DriverName: driverName})
}
