package socket

import (
	"fmt"
	"os/exec"

	"github.com/docker/docker/api"
	"github.com/docker/docker/hosts/state"
)

// Driver is a socket host driver. It is used to connect to existing Docker
// hosts by specifying the URL of the host as an option.
type Driver struct {
	url string
}

func NewDriver(options map[string]string, storePath string) (*Driver, error) {
	if _, ok := options["url"]; !ok {
		return nil, fmt.Errorf("The socket driver requires the option \"url\". Set it with -o url=...")
	}
	url, err := api.ValidateHostURL(options["url"])
	if err != nil {
		return nil, err
	}
	return &Driver{url: url}, nil
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

func (d *Driver) GetState() (state.State, error) {
	return state.Unknown, nil
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

func (d *Driver) Restart() error {
	return nil
}

func (d *Driver) Kill() error {
	return nil
}

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return nil
}
