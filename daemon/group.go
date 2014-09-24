package daemon

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/runconfig"
)

func (daemon *Daemon) GroupsCreate(config *api.Group) error {
	if config.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	if err := daemon.createGroup(config); err != nil {
		if !os.IsExist(err) {
			return err
		}

		if err := daemon.updateGroup(config); err != nil {
			return err
		}
	}

	for _, c := range config.Containers {
		if err := daemon.updateGroupContainer(config.Name, c); err != nil {
			return err
		}
	}

	return nil
}

func (daemon *Daemon) GroupsStop(name string) error {
	containers, err := daemon.fetchGroupsContainers(name)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := c.Stop(10); err != nil {
			return err
		}
	}

	return nil
}

func (daemon *Daemon) GroupsDelete(name string) error {
	containers, err := daemon.fetchGroupsContainers(name)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := daemon.eng.Job("rm", c.ID).Run(); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(filepath.Join(daemon.Config().Root, "groups", name)); err != nil {
		return err
	}

	if _, err := daemon.containerGraph.Purge(name); err != nil {
		return err
	}

	return nil
}

func (daemon *Daemon) createGroup(config *api.Group) error {
	var (
		groupsRoot = filepath.Join(daemon.Config().Root, "groups")
		groupDir   = filepath.Join(groupsRoot, config.Name)
	)

	if err := os.MkdirAll(groupsRoot, 0644); err != nil {
		return err
	}

	if err := os.Mkdir(groupDir, 0644); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(groupDir, "volumes"), 0644); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(groupDir, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	config.Created = time.Now()

	if err := json.NewEncoder(f).Encode(config); err != nil {
		return err
	}

	if _, err := daemon.containerGraph.Set("group-"+config.Name, config.Name); err != nil {
		return fmt.Errorf("path: %s id: %s: %s", "group-"+config.Name, config.Name, err)
	}

	return nil
}

func (daemon *Daemon) updateGroup(config *api.Group) error {
	oldConfig, err := daemon.fetchGroupConfig(config.Name)
	if err != nil {
		return err
	}

	config.Created = oldConfig.Created

	return daemon.updateGroupConfig(config)
}

func asRunConfig(groupDir string, c *api.Container) (*runconfig.Config, error) {
	r := &runconfig.Config{
		Image:        c.Image,
		Cmd:          c.Cmd,
		ExposedPorts: make(map[nat.Port]struct{}),
		Volumes:      make(map[string]struct{}),
		User:         c.User,
	}

	for _, p := range c.Ports {
		proto := p.Proto
		if proto == "" {
			proto = "tcp"
		}

		r.ExposedPorts[nat.Port(fmt.Sprintf("%d/%s", p.Container, proto))] = struct{}{}
	}

	return r, nil
}

func hashPath(p string) string {
	h := md5.New()
	fmt.Fprint(h, p)

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (daemon *Daemon) updateGroupContainer(groupName string, c *api.Container) error {
	fullName := filepath.Join("group-"+groupName, c.Name)

	if container := daemon.Get(fullName); container != nil {
		if err := daemon.Destroy(container); err != nil {
			return err
		}
	}

	groupDir := filepath.Join(daemon.Config().Root, "groups", groupName)

	config, err := asRunConfig(groupDir, c)
	if err != nil {
		return err
	}

	// do not pass a container name here and let docker auto generate the default name
	// we will set the name scoped to the group later
	container, _, err := daemon.Create(config, "")
	if err != nil {
		// TODO: atomic abort and cleanup??????
		return err
	}

	if err := setHostConfig(groupDir, c, container); err != nil {
		return err
	}

	if err := container.WriteHostConfig(); err != nil {
		return err
	}

	container.Group = groupName
	if err := container.ToDisk(); err != nil {
		return err
	}

	if _, err := daemon.containerGraph.Set(fullName, container.ID); err != nil {
		return fmt.Errorf("%s %s: %s", fullName, container.ID, err)
	}

	log.Printf("created %s (%s)\n", fullName, container.ID)

	return nil
}

func (daemon *Daemon) fetchGroupConfig(name string) (*api.Group, error) {
	var config *api.Group

	f, err := os.Open(daemon.groupConfigPath(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (daemon *Daemon) updateGroupConfig(config *api.Group) error {
	f, err := os.OpenFile(daemon.groupConfigPath(config.Name), os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(config)
}

func (daemon *Daemon) groupConfigPath(name string) string {
	return filepath.Join(daemon.Config().Root, "groups", name, "config.json")
}

func (daemon *Daemon) fetchGroupsContainers(name string) (map[string]*Container, error) {
	config, err := daemon.fetchGroupConfig(name)
	if err != nil {
		return nil, err
	}

	containers := make(map[string]*Container)
	for _, cconfig := range config.Containers {
		c := daemon.Get(filepath.Join("group-"+config.Name, cconfig.Name))
		if c == nil {
			return nil, fmt.Errorf("container does not exist for group %s", cconfig.Name)
		}

		containers[cconfig.Name] = c
	}

	return containers, nil
}

func setHostConfig(groupDir string, c *api.Container, cc *Container) error {
	if cc.hostConfig.PortBindings == nil {
		cc.hostConfig.PortBindings = nat.PortMap{}
	}

	cc.Volumes = make(map[string]string)
	cc.VolumesRW = make(map[string]bool)

	for p := range cc.Config.ExposedPorts {
	ports:
		for _, pp := range c.Ports {
			if p.Int() == pp.Container {
				cc.hostConfig.PortBindings[p] = append(cc.hostConfig.PortBindings[p], nat.PortBinding{
					HostPort: strconv.Itoa(pp.Host),
				})

				break ports
			}
		}
	}

	// TODO: @crosbymichael this does not belong here
	for _, v := range c.Volumes {
		if v.Host == "" {
			var (
				hash = hashPath(v.Container)
				path = filepath.Join(groupDir, "volumes", c.Name, hash)
			)

			if err := os.MkdirAll(path, 0644); err != nil {
				return err
			}

			v.Host = path
		}

		cc.Volumes[v.Container] = v.Host

		if v.Mode != "RO" {
			cc.VolumesRW[v.Container] = true
		}
	}

	return nil
}

func (daemon *Daemon) GroupsStart(name string) error {
	var (
		lines     = []string{}
		groupRoot = filepath.Join(daemon.Config().Root, "groups", name)
		hostsPath = filepath.Join(groupRoot, "hosts")
	)

	containers, err := daemon.fetchGroupsContainers(name)
	if err != nil {
		return err
	}

	for cname, c := range containers {
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

		lines = append(lines, fmt.Sprintf("%s %s", network.IP, cname))

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

		if err := c.setupMounts(); err != nil {
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

func (daemon *Daemon) GroupsGet(name string) ([]*api.Group, error) {
	groupsRoot := filepath.Join(daemon.Config().Root, "groups")

	groups := []*api.Group{}
	files, err := ioutil.ReadDir(groupsRoot)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Mode().IsDir() {
			if name == "" || name == file.Name() {
				groupRoot := filepath.Join(groupsRoot, file.Name())

				f, err := os.Open(filepath.Join(groupRoot, "config.json"))
				if err != nil {
					return nil, err
				}

				var group *api.Group
				if err := json.NewDecoder(f).Decode(&group); err != nil {
					f.Close()
					return nil, err
				}
				f.Close()

				groups = append(groups, group)
			}
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
