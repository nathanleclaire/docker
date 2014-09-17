package drivers

import (
	"fmt"

	"github.com/docker/docker/hosts/drivers/socket"
	"github.com/docker/docker/hosts/drivers/virtualbox"
	"github.com/docker/docker/hosts/state"
)

type Driver interface {
	DriverName() string
	GetOptions() map[string]string
	GetURL() string
	Create() error
	Remove() error
	Start() error
	State() (state.State, error)
	Stop() error
	// Kill() error
	// Restart() error
	// Pause() error
}

func NewDriver(name string, options map[string]string, storePath string) (Driver, error) {
	switch name {
	case "socket":
		return socket.NewDriver(options, storePath)
	case "virtualbox":
		return virtualbox.NewDriver(options, storePath)
	}
	return nil, fmt.Errorf("hosts: Unknown driver %q", name)
}
