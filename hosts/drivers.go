package hosts

import (
	"fmt"

	"github.com/docker/docker/hosts/drivers/socket"
)

type Driver interface {
	Name() string
	GetConfig() map[string]string
	// Create(name string) error
	// Start() error
	// Stop() error
	// Kill() error
	// Restart() error
	// Pause() error
	// Remove() error
	//State() (State, error)
}

func NewDriver(name string) (Driver, error) {
	switch name {
	case "socket":
		return socket.NewDriver(), nil
	}
	return nil, fmt.Errorf("hosts: Unknown driver %q", name)
}
