package socket

type Driver struct {
	Options map[string]string
}

func NewDriver(options map[string]string) *Driver {
	return &Driver{Options: options}
}

func (d *Driver) Name() string {
	return "socket"
}

func (d *Driver) GetOptions() map[string]string {
	return d.Options
}
