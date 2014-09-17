package daemon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/runconfig"
)

type Group struct {
	runconfig.GroupConfig

	Created time.Time
}

// create root
// create hosts file
// create volumes dir
func (daemon *Daemon) CreateGroup(config *runconfig.GroupConfig) error {
	groupRoot := filepath.Join(daemon.Config().Root, "groups", config.Name)

	if err := os.Mkdir(groupRoot, 0644); err != nil {
		return err
	}

	if err := os.Mkdir(filepath.Join(groupRoot, "volumes"), 0644); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(groupRoot, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	group := &Group{GroupConfig: *config, Created: time.Now()}

	if err := json.NewEncoder(f).Encode(group); err != nil {
		return err
	}

	for name, c := range config.Containers {
		container, _, err := daemon.Create(c.AsRunConfig(), config.Name+"-"+name)
		if err != nil {
			// TODO: atomic abort and cleanup??????
			return err
		}

		container.Group = config.Name

		log.Printf("group %s container %s with id%s\n", config.Name, name, container.ID)
	}

	return nil
}

func (daemon *Daemon) StartGroup(name string) error {
	var (
		config    *runconfig.GroupConfig
		groupRoot = filepath.Join(daemon.Config().Root, "groups", name)
		hostsPath = filepath.Join(groupRoot, "hosts")
	)

	f, err := os.Open(filepath.Join(groupRoot, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return err
	}

	lines := []string{}

	containers := []*Container{}
	for name := range config.Containers {
		c := daemon.Get(config.Name + "-" + name)
		if c == nil {
			return fmt.Errorf("container does not exist for group %s", name)
		}

		if err := c.setupContainerDns(); err != nil {
			return err
		}

		if err := c.Mount(); err != nil {
			return err
		}

		network, err := allocateNetwork(daemon.eng, name)
		if err != nil {
			return err
		}

		lines = append(lines, fmt.Sprintf("%s %s", network.IP, name))

		c.NetworkSettings.Bridge = network.Bridge
		c.NetworkSettings.IPAddress = network.IP
		c.NetworkSettings.IPPrefixLen = network.Len
		c.NetworkSettings.Gateway = network.Gateway

		if err := c.buildHostnameAndHostsFiles(c.NetworkSettings.IPAddress); err != nil {
			return err
		}

		// reset the hosts file
		c.HostsPath = hostsPath

		if err := c.setupWorkingDirectory(); err != nil {
			return err
		}

		env := c.createDaemonEnvironment(nil)
		if err := populateCommand(c, env); err != nil {
			return err
		}

		if err := setupMountsForContainer(c); err != nil {
			return err
		}

		containers = append(containers, c)
	}

	// write the groups hosts file
	if err := ioutil.WriteFile(hostsPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}

	for _, c := range containers {
		if err := c.waitForStart(); err != nil {
			return err
		}
	}

	return nil
}

func (daemon *Daemon) Groups() ([]*Group, error) {
	groups := []*Group{}

	groupsRoot := filepath.Join(daemon.Config().Root, "groups")
	files, err := ioutil.ReadDir(groupsRoot)
	if err != nil {
		return []*Group{}, err
	}

	for _, file := range files {
		if file.Mode().IsDir() {
			groupRoot := filepath.Join(groupsRoot, file.Name())
			f, err := os.Open(filepath.Join(groupRoot, "config.json"))
			if err != nil {
				return []*Group{}, err
			}
			defer f.Close()

			group := &Group{}
			if err := json.NewDecoder(f).Decode(group); err != nil {
				return []*Group{}, err
			}

			groups = append(groups, group)
		}
	}

	return groups, nil
}

type network struct {
	IP      string
	Bridge  string
	Len     int
	Gateway string
}

func allocateNetwork(eng *engine.Engine, id string) (*network, error) {
	var (
		err error
		env *engine.Env
		job = eng.Job("allocate_interface", id)
	)

	if env, err = job.Stdout.AddEnv(); err != nil {
		return nil, err
	}

	if err := job.Run(); err != nil {
		return nil, err
	}

	return &network{
		IP:      env.Get("IP"),
		Gateway: env.Get("Gateway"),
		Len:     env.GetInt("IPPrefixLen"),
		Bridge:  env.Get("Bridge"),
	}, nil
}
