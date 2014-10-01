package none

import (
	"fmt"
	"os/exec"

	"github.com/docker/docker/api"
	"github.com/docker/docker/hosts/drivers"
	"github.com/docker/docker/hosts/state"
	flag "github.com/docker/docker/pkg/mflag"
)

// Driver is the driver used when no driver is selected. It is used to
// connect to existing Docker hosts by specifying the URL of the host as
// an option.
type Driver struct {
	URL       string
	storePath string
}

type CreateFlags struct {
	URL *string
}

func init() {
	drivers.Register("none", &drivers.RegisteredDriver{
		New:                 NewDriver,
		RegisterCreateFlags: RegisterCreateFlags,
	})
}

// RegisterCreateFlags registers the flags this driver adds to
// "docker hosts create"
func RegisterCreateFlags(cmd *flag.FlagSet) interface{} {
	createFlags := new(CreateFlags)
	createFlags.URL = cmd.String([]string{"-url"}, "", "URL of host when no driver is selected")
	return createFlags
}

func NewDriver(storePath string) (drivers.Driver, error) {
	return &Driver{storePath: storePath}, nil
}

func (d *Driver) DriverName() string {
	return "none"
}

func (d *Driver) SetConfigFromFlags(flagsInterface interface{}) error {
	flags := flagsInterface.(*CreateFlags)
	if *flags.URL == "" {
		return fmt.Errorf("--url option is required when no driver is selected")
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
