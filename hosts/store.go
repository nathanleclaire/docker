package hosts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/docker/docker/pkg/log"
)

// Store persists hosts on the filesystem
type Store struct {
	Path string
}

func NewStore() *Store {
	rootPath := path.Join(os.Getenv("HOME"), ".docker/hosts")
	return &Store{Path: rootPath}
}

func (s *Store) Create(name string, driverName string, createFlags interface{}) (*Host, error) {
	exists, err := s.Exists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("Host %q already exists", name)
	}

	hostPath := path.Join(s.Path, name)

	host, err := NewHost(name, driverName, hostPath)
	if err != nil {
		return host, err
	}
	if createFlags != nil {
		if err := host.Driver.SetConfigFromFlags(createFlags); err != nil {
			return host, err
		}
	}

	if err := os.MkdirAll(hostPath, 0700); err != nil {
		return nil, err
	}

	if err := host.Create(); err != nil {
		return host, err
	}

	if err := host.SaveConfig(); err != nil {
		return host, err
	}

	return host, nil
}

func (s *Store) Remove(name string) error {
	active, err := s.GetActive()
	if err != nil {
		return err
	}
	if active != nil && active.Name == name {
		if err := s.RemoveActive(); err != nil {
			return err
		}
	}

	host, err := s.Load(name)
	if err != nil {
		return err
	}
	return host.Remove()
}

func (s *Store) List() ([]Host, error) {
	dir, err := ioutil.ReadDir(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Default host always exists
	defaultHost, err := s.Load("default")
	if err != nil {
		return nil, err
	}

	hosts := []Host{*defaultHost}

	for _, file := range dir {
		if file.IsDir() {
			// Ignore directories called default
			if file.Name() == "default" {
				continue
			}
			host, err := s.Load(file.Name())
			if err != nil {
				log.Errorf("error loading host %q: %s", file.Name(), err)
				continue
			}
			hosts = append(hosts, *host)
		}
	}
	return hosts, nil
}

func (s *Store) Exists(name string) (bool, error) {
	if name == "default" {
		return true, nil
	}
	_, err := os.Stat(path.Join(s.Path, name))
	if os.IsNotExist(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	}
	return false, err
}

func (s *Store) Load(name string) (*Host, error) {
	hostPath := path.Join(s.Path, name)
	return LoadHost(name, hostPath)
}

func (s *Store) GetActive() (*Host, error) {
	hostName, err := ioutil.ReadFile(s.activePath())
	if os.IsNotExist(err) {
		return s.Load("default")
	} else if err != nil {
		return nil, err
	}
	return s.Load(string(hostName))
}

func (s *Store) IsActive(host *Host) (bool, error) {
	active, err := s.GetActive()
	if err != nil {
		return false, err
	}
	if active == nil {
		return false, nil
	}
	return active.Name == host.Name, nil
}

func (s *Store) SetActive(host *Host) error {
	if err := os.MkdirAll(path.Dir(s.activePath()), 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(s.activePath(), []byte(host.Name), 0600)
}

func (s *Store) RemoveActive() error {
	return os.Remove(s.activePath())
}

// activePath returns the path to the file that stores the name of the
// active host
func (s *Store) activePath() string {
	return path.Join(s.Path, ".active")
}
