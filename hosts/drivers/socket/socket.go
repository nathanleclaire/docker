package socket

import (
	"fmt"

	"github.com/docker/docker/hosts/state"
)

type Driver struct {
	url string
}

func NewDriver(options map[string]string, storePath string) (*Driver, error) {
	if _, ok := options["url"]; !ok {
		return nil, fmt.Errorf("The socket driver requires the option \"url\". Set it with -o url=...")
	}
	return &Driver{url: options["url"]}, nil
}

func (d *Driver) DriverName() string {
	return "socket"
}

func (d *Driver) GetOptions() map[string]string {
	return map[string]string{"url": d.url}
}

func (d *Driver) GetURL() (string, error) {
	return d.url, nil
}

func (d *Driver) GetIP() (string, error) {
	return "", nil
}

func (d *Driver) Create() error {
	return nil
}

func (d *Driver) Start() error {
	return nil
}

func (d *Driver) Stop() error {
	return nil
}

func (d *Driver) Remove() error {
	return nil
}

func (d *Driver) State() (state.State, error) {
	return state.Unknown, nil
}
