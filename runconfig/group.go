package runconfig

import (
	"github.com/docker/docker/nat"
)

type GroupContainer struct {
	Image        string
	Command      []string
	ExposedPorts nat.PortSet
	PortBindings nat.PortMap
}

type GroupConfig struct {
	Name       string
	Containers map[string]*GroupContainer
}

func (c *GroupContainer) AsRunConfig() *Config {
	return &Config{
		Image:        c.Image,
		Cmd:          c.Command,
		ExposedPorts: c.ExposedPorts,
	}
}
