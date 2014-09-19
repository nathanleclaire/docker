package daemon

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/graphdb"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers/filters"
)

// List returns an array of all containers registered in the daemon.
func (daemon *Daemon) List() []*Container {
	return daemon.containers.List()
}

type listOptions struct {
	all         bool
	since       string
	sinceCont   *Container
	before      string
	beforeCont  *Container
	limit       int
	size        bool
	filt_exited []int
}

func (daemon *Daemon) Containers(job *engine.Job) engine.Status {
	options := listOptions{
		all:    job.GetenvBool("all"),
		since:  job.Getenv("since"),
		before: job.Getenv("before"),
		limit:  job.GetenvInt("limit"),
		size:   job.GetenvBool("size"),
	}

	psFilters, err := filters.FromParam(job.Getenv("filters"))
	if err != nil {
		return job.Error(err)
	}
	if i, ok := psFilters["exited"]; ok {
		for _, value := range i {
			code, err := strconv.Atoi(value)
			if err != nil {
				return job.Error(err)
			}
			options.filt_exited = append(options.filt_exited, code)
		}
	}

	if options.before != "" {
		options.beforeCont = daemon.Get(options.before)
		if options.beforeCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", options.before))
		}
	}

	if options.since != "" {
		options.sinceCont = daemon.Get(options.since)
		if options.sinceCont == nil {
			return job.Error(fmt.Errorf("Could not find container with name or id %s", options.since))
		}
	}

	outs := engine.NewTable("Created", 0)

	if err := daemon.listTopLevelContainersAndGroups(outs, options); err != nil {
		return job.Error(err)
	}

	outs.ReverseSort()
	if _, err := outs.WriteListTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}

func (daemon *Daemon) listTopLevelContainersAndGroups(outs *engine.Table, options listOptions) error {
	var (
		ungroupedContainers []*Container
	)

	for _, container := range filterContainers(daemon.List(), options) {
		ungroupedContainers = append(ungroupedContainers, container)
	}

	if err := daemon.listContainers(outs, ungroupedContainers, options); err != nil {
		return err
	}

	return nil
}

func (daemon *Daemon) listGroupContainers(outs *engine.Table, groupName string, options listOptions) error {
	var groupContainers []*Container

	for _, c := range filterContainers(daemon.List(), options) {
		if c.Group == groupName {
			groupContainers = append(groupContainers, c)
		}
	}

	return daemon.listContainers(outs, groupContainers, options)
}

func filterContainers(unfiltered []*Container, options listOptions) []*Container {
	var (
		foundBefore bool
		displayed   int
		filtered    []*Container
	)

	for _, container := range unfiltered {
		container.Lock()
		defer container.Unlock()
		if !container.Running && !options.all && options.limit <= 0 && options.since == "" && options.before == "" {
			continue
		}
		if options.before != "" && !foundBefore {
			if container.ID == options.beforeCont.ID {
				foundBefore = true
			}
			continue
		}
		if options.limit > 0 && displayed == options.limit {
			break
		}
		if options.since != "" {
			if container.ID == options.sinceCont.ID {
				break
			}
		}
		if len(options.filt_exited) > 0 && !container.Running {
			should_skip := true
			for _, code := range options.filt_exited {
				if code == container.GetExitCode() {
					should_skip = false
					break
				}
			}
			if should_skip {
				continue
			}
		}
		displayed++
		filtered = append(filtered, container)
	}

	return filtered
}

type nameMap map[string][]string

func (daemon *Daemon) listContainers(outs *engine.Table, containers []*Container, options listOptions) error {
	names := nameMap{}
	daemon.ContainerGraph().Walk("/", func(p string, e *graphdb.Entity) error {
		names[e.ID()] = append(names[e.ID()], p)
		return nil
	}, -1)

	for _, c := range containers {
		env, err := daemon.envForContainer(c, names, options)
		if err != nil {
			return err
		}
		outs.Add(env)
	}

	return nil
}

func (daemon *Daemon) envForContainer(container *Container, names nameMap, options listOptions) (*engine.Env, error) {
	out := &engine.Env{}
	out.Set("Type", "container")
	out.Set("Id", container.ID)

	out.SetList("Names", names[container.ID])
	out.Set("Image", daemon.Repositories().ImageName(container.Image))
	if len(container.Args) > 0 {
		args := []string{}
		for _, arg := range container.Args {
			if strings.Contains(arg, " ") {
				args = append(args, fmt.Sprintf("'%s'", arg))
			} else {
				args = append(args, arg)
			}
		}
		argsAsString := strings.Join(args, " ")

		out.Set("Command", fmt.Sprintf("\"%s %s\"", container.Path, argsAsString))
	} else {
		out.Set("Command", fmt.Sprintf("\"%s\"", container.Path))
	}
	out.SetInt64("Created", container.Created.Unix())
	out.Set("Status", container.State.String())
	str, err := container.NetworkSettings.PortMappingAPI().ToListString()
	if err != nil {
		return nil, err
	}
	out.Set("Ports", str)
	if options.size {
		sizeRw, sizeRootFs := container.GetSize()
		out.SetInt64("SizeRw", sizeRw)
		out.SetInt64("SizeRootFs", sizeRootFs)
	}
	return out, nil
}
