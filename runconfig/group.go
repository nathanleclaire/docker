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

type GroupContainerInput struct {
	Image   string
	Command []string
	Ports   []string
}

type GroupConfigInput struct {
	Name       string
	Containers map[string]*GroupContainerInput
}

func (i *GroupConfigInput) AsGroupConfig() (*GroupConfig, error) {
	group := &GroupConfig{
		Name:       i.Name,
		Containers: make(map[string]*GroupContainer),
	}

	for name, c := range i.Containers {
		exposedPorts, portBindings, err := nat.ParsePortSpecs(c.Ports)
		if err != nil {
			return nil, err
		}

		container := &GroupContainer{
			Image:        c.Image,
			Command:      c.Command,
			ExposedPorts: exposedPorts,
			PortBindings: portBindings,
		}

		group.Containers[name] = container
	}

	return group, nil
}

func (c *GroupContainer) AsRunConfig() *Config {
	return &Config{
		Image:        c.Image,
		Cmd:          c.Command,
		ExposedPorts: c.ExposedPorts,
	}
}
