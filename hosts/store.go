package hosts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
	hostPath := path.Join(s.Path, name)

	if _, err := os.Stat(hostPath); err == nil {
		return nil, fmt.Errorf("Host %q already exists", name)
	}

	if err := os.MkdirAll(hostPath, 0700); err != nil {
		return nil, err
	}

	host, err := NewHost(name, driverName, hostPath)
	if err != nil {
		return host, err
	}
	if createFlags != nil {
		if err := host.Driver.SetConfigFromFlags(createFlags); err != nil {
			return host, err
		}
	}
	if err := host.SaveConfig(); err != nil {
		return host, err
	}

	if err := host.Create(); err != nil {
		return host, err
	}
	return host, nil
}

func (s *Store) Remove(name string) error {
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
	var hosts []Host
	for _, file := range dir {
		if file.IsDir() {
			host, err := s.Load(file.Name())
			if err != nil {
				return nil, err
			}
			hosts = append(hosts, *host)
		}
	}
	return hosts, nil
}

func (s *Store) Exists(name string) (bool, error) {
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
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return s.Load(string(hostName))
}

func (s *Store) SetActive(host *Host) error {
	if err := os.MkdirAll(path.Dir(s.activePath()), 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(s.activePath(), []byte(host.Name), 0600)
}

// activePath returns the path to the file that stores the name of the
// active host
func (s *Store) activePath() string {
	return path.Join(s.Path, ".active")
}
