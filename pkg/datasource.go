package pkg

import "fmt"

type DataSource struct {
	driverName string
	Dsn        string
}

type DsnGroup struct {
	activeDriver string
	DsnList      []DataSource
}

func (d DsnGroup) GetDSN(driverName string) string {
	ddd := ""
	for _, dd := range d.DsnList {
		if dd.driverName == driverName {
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
		if dd.driverName == driverName {
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
	d.DsnList = append(d.DsnList, DataSource{Dsn: dsn, driverName: driverName})
}

func NewDsnConf(dsn, drivername string) *DsnGroup {
	return &DsnGroup{
		activeDriver: drivername,
		DsnList:      []DataSource{{driverName: drivername, Dsn: dsn}},
	}
}
