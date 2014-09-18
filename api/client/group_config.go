package client

import (
	"fmt"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/runconfig"
)

type GroupContainer struct {
	Image   string
	Build   string
	Command []string
	Ports   []string
}

type GroupConfig struct {
	Name       string
	Containers map[string]*GroupContainer
}

// Transforms a client.GroupConfig into a runconfig.GroupConfig by:
//
//  - automatically building images
//  - parsing port specs
//
func (cli *DockerCli) processGroupConfig(raw *GroupConfig) (*runconfig.GroupConfig, error) {
	group := &runconfig.GroupConfig{
		Name:       raw.Name,
		Containers: make(map[string]*runconfig.GroupContainer),
	}

	for containerName, c := range raw.Containers {
		container := &runconfig.GroupContainer{}

		if c.Build != "" {
			if c.Image != "" {
				return nil, fmt.Errorf("%s specifies both 'build' and 'image'", containerName)
			}

			tag := fmt.Sprintf("%s-%s", group.Name, containerName)
			imageExists, err := cli.checkImageExists(tag)
			if err != nil {
				return nil, err
			}

			if !imageExists {
				if err := cli.build(c.Build, buildOptions{tag: tag}); err != nil {
					return nil, err
				}
			}

			container.Image = tag
		} else {
			container.Image = c.Image
		}

		container.Command = c.Command

		exposedPorts, portBindings, err := nat.ParsePortSpecs(c.Ports)
		if err != nil {
			return nil, err
		}

		container.ExposedPorts = exposedPorts
		container.PortBindings = portBindings

		group.Containers[containerName] = container
	}

	return group, nil
}

func (cli *DockerCli) checkImageExists(image string) (bool, error) {
	_, status, err := readBody(cli.call("GET", "/images/"+image+"/json", nil, false))
	if err == nil {
		return true, nil
	} else {
		if status == 404 {
			return false, nil
		} else {
			return false, err
		}
	}
}
