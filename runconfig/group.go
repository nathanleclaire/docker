package runconfig

type GroupContainer struct {
	Image   string
	Command []string
}

type GroupConfig struct {
	Name       string
	Containers map[string]*GroupContainer
}

func (c *GroupContainer) AsRunConfig() *Config {
	return &Config{
		Image: c.Image,
		Cmd:   c.Command,
	}
}
