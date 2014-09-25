package drivers

import (
	"fmt"
	"os/exec"

	"github.com/docker/docker/hosts/drivers/digitalocean"
	"github.com/docker/docker/hosts/drivers/socket"
	"github.com/docker/docker/hosts/drivers/virtualbox"
	"github.com/docker/docker/hosts/state"
	flag "github.com/docker/docker/pkg/mflag"
)

// Driver defines how a host is created and controlled. Different types of
// driver represent different ways hosts can be created (e.g. different
// hypervisors, different cloud providers)
type Driver interface {
	DriverName() string
	GetURL() (string, error)
	GetIP() (string, error)
	GetState() (state.State, error)
	Create() error
	SetConfigFromFlags(flags interface{}) error
	Remove() error
	Start() error
	Stop() error
	Restart() error
	Kill() error
	GetSSHCommand(args ...string) *exec.Cmd
	// Pause() error
}

// NewDriver creates a new driver of type "name"
func NewDriver(name string, storePath string) (Driver, error) {
	switch name {
	case "digitalocean":
		return digitalocean.NewDriver(storePath)
	case "socket":
		return socket.NewDriver(storePath)
	case "virtualbox":
		return virtualbox.NewDriver(storePath)
	}
	return nil, fmt.Errorf("hosts: Unknown driver %q", name)
}

// RegisterCreateFlags registers the flags for all of the drivers but ignores
// the value of the flags. A second pass is done to gather the value of the
// flags once we know what driver has been picked
func RegisterCreateFlags(cmd *flag.FlagSet) map[string]interface{} {
	return map[string]interface{}{
		"digitalocean": digitalocean.RegisterCreateFlags(cmd),
		"socket":       socket.RegisterCreateFlags(cmd),
		"virtualbox":   virtualbox.RegisterCreateFlags(cmd),
	}
}
