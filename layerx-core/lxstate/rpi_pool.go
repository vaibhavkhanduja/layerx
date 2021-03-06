package lxstate

import (
	"encoding/json"
	"github.com/emc-advanced-dev/layerx/layerx-core/layerx_rpi_client"
	"github.com/emc-advanced-dev/pkg/errors"
	"github.com/layer-x/layerx-commons/lxdatabase"
)

type RpiPool struct {
	rootKey string
}

func (RpiPool *RpiPool) GetKey() string {
	return RpiPool.rootKey
}

func (RpiPool *RpiPool) Initialize() error {
	err := lxdatabase.Mkdir(RpiPool.GetKey())
	if err != nil {
		return errors.New("initializing "+RpiPool.GetKey()+" directory", err)
	}
	return nil
}

func (RpiPool *RpiPool) AddRpi(rpi *layerx_rpi_client.RpiInfo) error {
	if rpi.Name == "" || rpi.Url == "" {
		return errors.New("cannot accept rpi "+rpi.Name+" with no name or url!", nil)
	}
	rpiData, err := json.Marshal(rpi)
	if err != nil {
		return errors.New("could not marshal rpi to json", err)
	}
	err = lxdatabase.Set(RpiPool.GetKey()+"/"+rpi.Name, string(rpiData))
	if err != nil {
		return errors.New("setting key/value pair for rpi", err)
	}
	return nil
}

func (RpiPool *RpiPool) GetRpi(name string) (*layerx_rpi_client.RpiInfo, error) {
	rpiJson, err := lxdatabase.Get(RpiPool.GetKey() + "/" + name)
	if err != nil {
		return nil, errors.New("retrieving rpi "+name+" from database", err)
	}
	var rpi layerx_rpi_client.RpiInfo
	err = json.Unmarshal([]byte(rpiJson), &rpi)
	if err != nil {
		return nil, errors.New("unmarshalling json into Rpi struct", err)
	}
	return &rpi, nil
}

func (RpiPool *RpiPool) GetRpis() (map[string]*layerx_rpi_client.RpiInfo, error) {
	rpis := make(map[string]*layerx_rpi_client.RpiInfo)
	knownRpis, err := lxdatabase.GetKeys(RpiPool.GetKey())
	if err != nil {
		return nil, errors.New("retrieving list of known rpis", err)
	}
	for _, rpiJson := range knownRpis {
		var rpi layerx_rpi_client.RpiInfo
		err = json.Unmarshal([]byte(rpiJson), &rpi)
		if err != nil {
			return nil, errors.New("unmarshalling json into Rpi struct", err)
		}
		rpis[rpi.Name] = &rpi
	}
	return rpis, nil
}

func (RpiPool *RpiPool) DeleteRpi(name string) error {
	_, err := RpiPool.GetRpi(name)
	if err != nil {
		return errors.New("rpi "+name+" not found", err)
	}
	err = lxdatabase.Rm(RpiPool.GetKey() + "/" + name)
	if err != nil {
		return errors.New("removing rpi "+name+" from database", err)
	}
	return nil
}
