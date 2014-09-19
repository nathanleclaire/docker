package client

import (
	"fmt"
	"strconv"

	"github.com/docker/docker/api"
	"github.com/docker/docker/nat"
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
func (cli *DockerCli) processGroupConfig(raw *GroupConfig) (*api.Group, error) {
	group := &api.Group{
		Name: raw.Name,
	}

	for containerName, c := range raw.Containers {
		container := &api.Container{
			Name: containerName,
		}

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
			tag := c.Image

			imageExists, err := cli.checkImageExists(tag)
			if err != nil {
				return nil, err
			}

			if !imageExists {
				if err := cli.pullImage(tag); err != nil {
					return nil, err
				}
			}

			container.Image = tag
		}

		container.Command = c.Command

		_, portBindings, err := nat.ParsePortSpecs(c.Ports)
		if err != nil {
			return nil, err
		}

		for p, b := range portBindings {
			pp := &api.Port{
				Container: p.Int(),
				Proto:     p.Proto(),
			}

			if len(b) > 0 {
				// FIXME: support more than one
				bb := b[0]

				hp, err := strconv.Atoi(bb.HostPort)
				if err != nil {
					return nil, err
				}

				pp.Host = hp
			}

			container.Ports = append(container.Ports, pp)
		}

		// TODO: volumes....

		group.Containers = append(group.Containers, container)
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
