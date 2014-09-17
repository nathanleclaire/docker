package hosts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"

	"github.com/docker/docker/hosts/drivers"
)

var (
	validHostNameChars   = `[a-zA-Z0-9_]`
	validHostNamePattern = regexp.MustCompile(`^` + validHostNameChars + `+$`)
)

type Host struct {
	Name      string
	Driver    drivers.Driver
	storePath string
}

type Config struct {
	DriverName    string
	DriverOptions map[string]string
}

func NewHost(name, driverName string, driverOptions map[string]string, storePath string) (*Host, error) {
	driver, err := drivers.NewDriver(driverName, driverOptions, storePath)
	if err != nil {
		return nil, err
	}
	return &Host{Name: name, Driver: driver, storePath: storePath}, nil
}

func LoadHost(name string, storePath string) (*Host, error) {
	host := &Host{Name: name, storePath: storePath}
	if err := host.LoadConfig(); err != nil {
		return nil, err
	}
	return host, nil
}

func ValidateHostName(name string) (string, error) {
	if !validHostNamePattern.MatchString(name) {
		return name, fmt.Errorf("Invalid host name %q, it must match %s", name, validHostNamePattern)
	}
	return name, nil
}

func (h *Host) Create() error {
	if err := os.Mkdir(h.storePath, 0700); err != nil {
		return err
	}
	if err := h.Driver.Create(); err != nil {
		return err
	}
	config := Config{DriverName: h.Driver.DriverName(), DriverOptions: h.Driver.GetOptions()}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(path.Join(h.storePath, "config.json"), data, 0600); err != nil {
		return err
	}
	return nil
}

func (h *Host) Start() error {
	return h.Driver.Start()
}

func (h *Host) Stop() error {
	return h.Driver.Stop()
}

func (h *Host) Remove() error {
	if err := h.Driver.Remove(); err != nil {
		return err
	}

	file, err := os.Stat(h.storePath)
	if err != nil {
		return err
	}
	if !file.IsDir() {
		return fmt.Errorf("%q is not a directory", h.storePath)
	}
	return os.RemoveAll(h.storePath)
}

func (h *Host) LoadConfig() error {
	data, err := ioutil.ReadFile(path.Join(h.storePath, "config.json"))
	if err != nil {
		return err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	driver, err := drivers.NewDriver(config.DriverName, config.DriverOptions, h.storePath)
	if err != nil {
		return err
	}
	h.Driver = driver
	return nil
}
