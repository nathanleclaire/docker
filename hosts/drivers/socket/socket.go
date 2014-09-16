package socket

type Driver struct {
	Config map[string]string
}

func NewDriver() *Driver {
	return &Driver{}
}

func (d *Driver) Name() string {
	return "socket"
}

func (d *Driver) GetConfig() map[string]string {
	return map[string]string{}
}
