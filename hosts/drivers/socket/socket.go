package socket

import "fmt"

type Driver struct {
	Options map[string]string
}

func NewDriver(options map[string]string) (*Driver, error) {
	if _, ok := options["url"]; !ok {
		return nil, fmt.Errorf("The socket driver requires the option \"url\". Set it with -o url=...")
	}
	return &Driver{Options: options}, nil
}

func (d *Driver) Name() string {
	return "socket"
}

func (d *Driver) GetOptions() map[string]string {
	return d.Options
}

func (d *Driver) GetURL() string {
	return d.Options["url"]
}
