package drivers

import (
	"fmt"

	"github.com/docker/docker/hosts/drivers/socket"
	"github.com/docker/docker/hosts/drivers/virtualbox"
	"github.com/docker/docker/hosts/state"
)

// Driver defines how a host is created and controlled. Different types of
// driver represent different ways hosts can be created (e.g. different
// hypervisors, different cloud providers)
type Driver interface {
	DriverName() string
	GetOptions() map[string]string
	GetURL() (string, error)
	GetIP() (string, error)
	GetState() (state.State, error)
	Create() error
	Remove() error
	Start() error
	Stop() error
	Restart() error
	Kill() error
	// Pause() error
}

// NewDriver creates a new driver of type "name"
func NewDriver(name string, options map[string]string, storePath string) (Driver, error) {
	switch name {
	case "socket":
		return socket.NewDriver(options, storePath)
	case "virtualbox":
		return virtualbox.NewDriver(options, storePath)
	}
	return nil, fmt.Errorf("hosts: Unknown driver %q", name)
}
