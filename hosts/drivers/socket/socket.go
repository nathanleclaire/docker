package socket

import (
	"fmt"
	"os/exec"

	"github.com/docker/docker/api"
	"github.com/docker/docker/hosts/state"
	flag "github.com/docker/docker/pkg/mflag"
)

// Driver is a socket host driver. It is used to connect to existing Docker
// hosts by specifying the URL of the host as an option.
type Driver struct {
	URL       string
	storePath string
}

type CreateFlags struct {
	URL *string
}

// RegisterCreateFlags registers the flags this driver adds to
// "docker hosts create"
func RegisterCreateFlags(cmd *flag.FlagSet) *CreateFlags {
	createFlags := new(CreateFlags)
	createFlags.URL = cmd.String([]string{"-socket-url"}, "", "Socket driver: URL of host")
	return createFlags
}

func NewDriver(storePath string) (*Driver, error) {
	return &Driver{storePath: storePath}, nil
}

func (d *Driver) DriverName() string {
	return "socket"
}

func (d *Driver) SetConfigFromFlags(flagsInterface interface{}) error {
	flags := flagsInterface.(*CreateFlags)
	if *flags.URL == "" {
		return fmt.Errorf("--socket-url option is required for socket driver")
	}
	url, err := api.ValidateHostURL(*flags.URL)
	if err != nil {
		return err
	}
	d.URL = url
	return nil
}

func (d *Driver) GetURL() (string, error) {
	return d.URL, nil
}

func (d *Driver) GetIP() (string, error) {
	return "", nil
}

func (d *Driver) GetState() (state.State, error) {
	return state.None, nil
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
