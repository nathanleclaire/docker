package drivers

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/docker/api"
	"github.com/docker/docker/hosts/state"
)

type DefaultDriver struct {
	URL string
}

func (d *DefaultDriver) DriverName() string {
	return ""
}

func (d *DefaultDriver) SetConfigFromFlags(flagsInterface interface{}) error {
	return nil
}

func (d *DefaultDriver) GetURL() (string, error) {
	if d.URL != "" {
		return d.URL, nil
	}
	if os.Getenv("DOCKER_HOST") != "" {
		return os.Getenv("DOCKER_HOST"), nil
	}
	return fmt.Sprintf("unix://%s", api.DEFAULTUNIXSOCKET), nil
}

func (d *DefaultDriver) GetIP() (string, error) {
	return "", nil
}

func (d *DefaultDriver) GetState() (state.State, error) {
	return state.None, nil
}

func (d *DefaultDriver) Create() error {
	return nil
}

func (d *DefaultDriver) Start() error {
	return nil
}

func (d *DefaultDriver) Stop() error {
	return nil
}

func (d *DefaultDriver) Remove() error {
	return fmt.Errorf("default driver cannot be removed")
}

func (d *DefaultDriver) Restart() error {
	return nil
}

func (d *DefaultDriver) Kill() error {
	return nil
}

func (d *DefaultDriver) Upgrade() error {
	return nil
}

func (d *DefaultDriver) GetSSHCommand(args ...string) *exec.Cmd {
	return nil
}
