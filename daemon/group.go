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

func (daemon *Daemon) CreateGroup(config *runconfig.GroupConfig) error {
	if err := daemon.createGroup(config); err != nil {
		return err
	}

	for containerName, containerConfig := range config.Containers {
		daemon.createGroupContainer(config.Name, containerName, containerConfig)
	}

	return nil
}

func (daemon *Daemon) createGroup(config *runconfig.GroupConfig) error {
	groupsRoot := filepath.Join(daemon.Config().Root, "groups")

	if err := os.MkdirAll(groupsRoot, 0644); err != nil {
		return err
	}

	groupDir := filepath.Join(groupsRoot, config.Name)

	if err := os.Mkdir(groupDir, 0644); err != nil {
		return err
	}

	if err := os.Mkdir(filepath.Join(groupDir, "volumes"), 0644); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(groupDir, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	group := &Group{GroupConfig: *config, Created: time.Now()}

	if err := json.NewEncoder(f).Encode(group); err != nil {
		return err
	}

	daemon.containerGraph.Set(config.Name, "group-"+config.Name)

	return nil
}

func (daemon *Daemon) createGroupContainer(groupName string, containerName string, containerConfig *runconfig.GroupContainer) error {
	fullName := filepath.Join(groupName, containerName)

	container, _, err := daemon.Create(containerConfig.AsRunConfig(), "")
	if err != nil {
		// TODO: atomic abort and cleanup??????
		return err
	}

	container.Group = groupName
	if err := container.ToDisk(); err != nil {
		return err
	}

	daemon.containerGraph.Set(fullName, container.ID)

	log.Printf("created %s (%s)\n", fullName, container.ID)

	return nil
}

func (daemon *Daemon) groupConfig(name string) (*runconfig.GroupConfig, error) {
	var (
		config    *runconfig.GroupConfig
		groupRoot = filepath.Join(daemon.Config().Root, "groups", name)
	)

	f, err := os.Open(filepath.Join(groupRoot, "config.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (daemon *Daemon) groupContainers(name string) ([]*Container, error) {
	config, err := daemon.groupConfig(name)
	if err != nil {
		return nil, err
	}

	containers := []*Container{}
	for name := range config.Containers {
		c := daemon.Get(filepath.Join(config.Name, name))
		if c == nil {
			return nil, fmt.Errorf("container does not exist for group %s", name)
		}

		containers = append(containers, c)
	}

	return containers, nil
}

func (daemon *Daemon) StartGroup(name string) error {
	var (
		lines     = []string{}
		groupRoot = filepath.Join(daemon.Config().Root, "groups", name)
		hostsPath = filepath.Join(groupRoot, "hosts")
	)

	containers, err := daemon.groupContainers(name)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := c.setupContainerDns(); err != nil {
			return err
		}

		if err := c.Mount(); err != nil {
			return err
		}

		network, err := allocateNetwork(daemon.eng, c.ID)
		if err != nil {
			return err
		}

		lines = append(lines, fmt.Sprintf("%s %s", network.IP, name))

		c.NetworkSettings.Bridge = network.Bridge
		c.NetworkSettings.IPAddress = network.IP
		c.NetworkSettings.IPPrefixLen = network.Len
		c.NetworkSettings.Gateway = network.Gateway

		for port := range c.Config.ExposedPorts {
			if err := c.allocatePort(daemon.eng, port, c.hostConfig.PortBindings); err != nil {
				return err
			}
		}

		c.NetworkSettings.PortMapping = nil
		c.NetworkSettings.Ports = c.hostConfig.PortBindings

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
	groupsRoot := filepath.Join(daemon.Config().Root, "groups")

	if err := os.MkdirAll(groupsRoot, 0644); err != nil {
		return nil, err
	}

	groups := []*Group{}

	files, err := ioutil.ReadDir(groupsRoot)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Mode().IsDir() {
			groupRoot := filepath.Join(groupsRoot, file.Name())
			f, err := os.Open(filepath.Join(groupRoot, "config.json"))
			if err != nil {
				return nil, err
			}
			defer f.Close()

			group := &Group{}
			if err := json.NewDecoder(f).Decode(group); err != nil {
				return nil, err
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
