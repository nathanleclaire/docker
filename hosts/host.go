package hosts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

type Host struct {
	Name      string
	Driver    Driver
	storePath string
}

type Config struct {
	DriverName   string
	DriverConfig map[string]string
}

func NewHost(name, driverName, storePath string) (*Host, error) {
	driver, err := NewDriver(driverName)
	if err != nil {
		return nil, err
	}
	return &Host{Name: name, Driver: driver, storePath: storePath}, nil
}

func LoadHost(name string, storePath string) (*Host, error) {
	host := &Host{Name: name, storePath: storePath}
	if err := host.LoadConfig(path.Join(storePath, name)); err != nil {
		return nil, err
	}
	return host, nil
}

func (h *Host) Create() error {
	if err := os.Mkdir(path.Join(h.storePath, h.Name), 0700); err != nil {
		return err
	}
	config := Config{DriverName: h.Driver.Name(), DriverConfig: h.Driver.GetConfig()}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path.Join(h.storePath, h.Name, "config.json"), data, 0600); err != nil {
		return err
	}
	return nil
}

func (h *Host) Remove() error {
	hostPath := path.Join(h.storePath, h.Name)
	file, err := os.Stat(hostPath)
	if err != nil {
		return err
	}
	if !file.IsDir() {
		return fmt.Errorf("%q is not a directory", hostPath)
	}
	return os.RemoveAll(hostPath)
}

func (h *Host) LoadConfig(storePath string) error {
	data, err := ioutil.ReadFile(path.Join(storePath, "config.json"))
	if err != nil {
		return err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	driver, err := NewDriver(config.DriverName)
	if err != nil {
		return err
	}
	h.Driver = driver
	return nil
}
