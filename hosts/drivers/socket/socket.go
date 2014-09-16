package socket

import "fmt"

type Driver struct {
	url string
}

func NewDriver(options map[string]string) (*Driver, error) {
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

func (d *Driver) GetURL() string {
	return d.url
}

func (d *Driver) Create() error {
	return nil
}

func (d *Driver) Remove() error {
	return nil
}
