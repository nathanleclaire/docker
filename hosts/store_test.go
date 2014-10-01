package hosts

import (
	"os"
	"path"
	"testing"
)

func clearHosts() error {
	return os.RemoveAll(path.Join(os.Getenv("HOME"), ".docker/hosts"))
}

func TestStoreCreate(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	host, err := store.Create("test", "none", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "test" {
		t.Fatal("Host name is incorrect")
	}
	path := path.Join(os.Getenv("HOME"), ".docker/hosts/test")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Host path doesn't exist: %s", path)
	}
}

func TestStoreRemove(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	_, err := store.Create("test", "none", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	path := path.Join(os.Getenv("HOME"), ".docker/hosts/test")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Host path doesn't exist: %s", path)
	}
	err = store.Remove("test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("Host path still exists after remove: %s", path)
	}
}

func TestStoreList(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	_, err := store.Create("test", "none", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	hosts, err := store.List()
	if len(hosts) != 1 {
		t.Fatalf("List returned %d items", len(hosts))
	}
	if hosts[0].Name != "test" {
		t.Fatal("Host name is incorrect")
	}
}

func TestStoreExists(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	exists, err := store.Exists("test")
	if exists {
		t.Fatal("Exists returned true when it should have been false")
	}
	_, err = store.Create("test", "none", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	exists, err = store.Exists("test")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("Exists returned false when it should have been true")
	}
}

func TestStoreLoad(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	_, err := store.Create("test", "none", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	host, err := store.Load("test")
	if host.Name != "test" {
		t.Fatal("Host name is incorrect")
	}
}

func TestStoreGetSetActive(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	originalHost, err := store.Create("test", "socket", map[string]string{
		"url": "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Fatal(err)
	}

	host, err := store.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if host != nil {
		t.Fatalf("Active host should not exist, got %s", host.Name)
	}

	if err := store.SetActive(originalHost); err != nil {
		t.Fatal(err)
	}

	host, err = store.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "test" {
		t.Fatalf("Active host is not 'test', got %s", host.Name)
	}
}
