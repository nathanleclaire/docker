package drivers

import (
	"fmt"
	"os/exec"
	"sort"

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

type RegisteredDriver struct {
	New                 func(storePath string) (Driver, error)
	RegisterCreateFlags func(cmd *flag.FlagSet) interface{}
}

var (
	drivers map[string]*RegisteredDriver
)

func init() {
	drivers = make(map[string]*RegisteredDriver)
}

func Register(name string, registeredDriver *RegisteredDriver) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}
	drivers[name] = registeredDriver

	return nil
}

// NewDriver creates a new driver of type "name"
func NewDriver(name string, storePath string) (Driver, error) {
	driver, exists := drivers[name]
	if !exists {
		return nil, fmt.Errorf("hosts: Unknown driver %q", name)
	}
	return driver.New(storePath)
}

// RegisterCreateFlags registers the flags for all of the drivers but ignores
// the value of the flags. A second pass is done to gather the value of the
// flags once we know what driver has been picked
func RegisterCreateFlags(cmd *flag.FlagSet) map[string]interface{} {
	flags := make(map[string]interface{})
	for driverName := range drivers {
		driver := drivers[driverName]
		flags[driverName] = driver.RegisterCreateFlags(cmd)
	}
	return flags
}

// GetDriverNames returns a slice of all registered driver names
func GetDriverNames() []string {
	names := make([]string, 0, len(drivers))
	for k := range drivers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
