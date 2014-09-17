package runconfig

type GroupVolume struct {
	Name          string
	ContainerPath string
}

type GroupContainer struct {
	Image   string
	Command []string
	Volumes []*GroupVolume
}

type GroupConfig struct {
	Name  string
	Ports struct {
		HostPort      int
		ContainerPort int
		HostIP        string
	}
	VolumesRoot string
	Containers  map[string]*GroupContainer
}

func (c *GroupContainer) AsRunConfig() *Config {
	return &Config{
		Image: c.Image,
		Cmd:   c.Command,
	}
}
