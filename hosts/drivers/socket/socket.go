package socket

import "fmt"

type Driver struct {
	options map[string]string
}

func NewDriver(options map[string]string) (*Driver, error) {
	if _, ok := options["url"]; !ok {
		return nil, fmt.Errorf("The socket driver requires the option \"url\". Set it with -o url=...")
	}
	return &Driver{options: options}, nil
}

func (d *Driver) DriverName() string {
	return "socket"
}

func (d *Driver) GetOptions() map[string]string {
	return d.options
}

func (d *Driver) GetURL() string {
	return d.options["url"]
}

func (d *Driver) Create() error {
	return nil
}
