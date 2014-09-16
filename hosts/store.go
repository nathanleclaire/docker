package hosts

import (
	"io/ioutil"
	"os"
	"path"
)

// Store persists hosts on the filesystem
type Store struct {
	Path string
}

func NewStore() (*Store, error) {
	rootPath := path.Join(os.Getenv("HOME"), ".docker/hosts")
	return &Store{Path: rootPath}, nil
}

func (s *Store) Create(name string, driverName string) error {
	host, err := NewHost(name, driverName, s.Path)
	if err != nil {
		return err
	}
	return host.Create()
}

func (s *Store) Remove(name string) error {
	host, err := LoadHost(name, s.Path)
	if err != nil {
		return err
	}
	return host.Remove()
}

func (s *Store) List() ([]Host, error) {
	dir, err := ioutil.ReadDir(s.Path)
	if err != nil {
		return nil, err
	}
	var hosts []Host
	for _, file := range dir {
		if file.IsDir() {
			host, err := LoadHost(file.Name(), s.Path)
			if err != nil {
				return nil, err
			}
			hosts = append(hosts, *host)
		}
	}
	return hosts, nil
}
