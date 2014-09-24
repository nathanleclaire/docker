package client

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/units"
)

type GroupContainer struct {
	Image   string
	Build   string
	Command []string

	Ports   []string
	Volumes []string

	User string

	Memory    string
	CpuShares int64  `yaml:"cpu_shares"`
	Cpuset    string `yaml:"cpu_set"`
}

type GroupConfig struct {
	Name       string
	Containers map[string]*GroupContainer
}

// Transforms a GroupConfig (read from YAML) into an api.Group (for posting as JSON)
// Does not handle auto-build or auto-pull of images - see cli.transformGroupConfig
func preprocessGroupConfig(raw *GroupConfig) (*api.Group, error) {
	group := &api.Group{
		Name: raw.Name,
	}

	for containerName, c := range raw.Containers {
		container := &api.Container{
			Name:  containerName,
			Image: c.Image,
			Cmd:   c.Command,

			User: c.User,

			CpuShares: c.CpuShares,
			Cpuset:    c.Cpuset,
		}

		if c.Memory != "" {
			ram, err := units.RAMInBytes(c.Memory)
			if err != nil {
				return nil, err
			}
			container.Memory = ram
		}

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

		for _, rawVolume := range c.Volumes {
			parts := strings.Split(rawVolume, ":")
			switch len(parts) {
			case 0:
				return nil, fmt.Errorf("invalid volume format %s", rawVolume)
			case 1:
				container.Volumes = append(container.Volumes, &api.Volume{
					Container: parts[0],
					Mode:      "rw",
				})
			case 2:
				container.Volumes = append(container.Volumes, &api.Volume{
					Container: parts[1],
					Host:      parts[0],
					Mode:      "rw",
				})
			case 3:
				container.Volumes = append(container.Volumes, &api.Volume{
					Container: parts[1],
					Host:      parts[0],
					Mode:      strings.ToUpper(parts[2]),
				})
			}
		}

		group.Containers = append(group.Containers, container)
	}

	return group, nil
}

// Transforms a GroupConfig (read from YAML) into an api.Group (for posting as JSON)
func (cli *DockerCli) transformGroupConfig(raw *GroupConfig) (*api.Group, error) {
	group, err := preprocessGroupConfig(raw)
	if err != nil {
		return nil, err
	}

	for _, processedContainer := range group.Containers {
		c := raw.Containers[processedContainer.Name]

		tag, err := cli.resolveContainerConfigImageTag(group.Name, processedContainer.Name, c)
		if err != nil {
			return nil, err
		}

		processedContainer.Image = tag
	}

	return group, nil
}

func (cli *DockerCli) resolveContainerConfigImageTag(groupName string, containerName string, c *GroupContainer) (string, error) {
	if c.Build != "" {
		if c.Image != "" {
			return "", fmt.Errorf("%s specifies both 'build' and 'image'", containerName)
		}

		tag := fmt.Sprintf("%s-%s", groupName, containerName)

		imageExists, err := cli.checkImageExists(tag)
		if err != nil {
			return "", err
		}

		if !imageExists {
			if err := cli.build(c.Build, buildOptions{tag: tag}); err != nil {
				return "", err
			}
		}

		return tag, nil
	} else {
		imageExists, err := cli.checkImageExists(c.Image)
		if err != nil {
			return "", err
		}

		if !imageExists {
			if err := cli.pullImage(c.Image); err != nil {
				return "", err
			}
		}

		return c.Image, nil
	}
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
