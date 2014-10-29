package hosts

import (
	"os"
	"path"
	"testing"

	none "github.com/docker/docker/hosts/drivers/none"
	homedir "github.com/mitchellh/go-homedir"
)

func clearHosts() error {
	homeDir, err := homedir.Dir()
	if err != nil {
		return err
	}
	return os.RemoveAll(path.Join(homeDir, ".docker/hosts"))
}

func TestStoreCreate(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	url := "unix:///var/run/docker.sock"
	host, err := store.Create("test", "none", &none.CreateFlags{URL: &url})
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "test" {
		t.Fatal("Host name is incorrect")
	}
	homeDir, err := homedir.Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := path.Join(homeDir, ".docker/hosts/test")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Host path doesn't exist: %s", path)
	}
}

func TestStoreRemove(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	url := "unix:///var/run/docker.sock"
	_, err := store.Create("test", "none", &none.CreateFlags{URL: &url})
	if err != nil {
		t.Fatal(err)
	}
	homeDir, err := homedir.Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := path.Join(homeDir, ".docker/hosts/test")
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
	url := "unix:///var/run/docker.sock"
	_, err := store.Create("test", "none", &none.CreateFlags{URL: &url})
	if err != nil {
		t.Fatal(err)
	}
	hosts, err := store.List()
	if len(hosts) != 2 {
		t.Fatalf("List returned %d items", len(hosts))
	}
	if hosts[0].Name != "default" {
		t.Fatalf("hosts[0] name is incorrect, got: %s", hosts[0].Name)
	}
	if hosts[1].Name != "test" {
		t.Fatalf("hosts[1] name is incorrect, got: %s", hosts[1].Name)
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
	url := "unix:///var/run/docker.sock"
	_, err = store.Create("test", "none", &none.CreateFlags{URL: &url})
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

func TestStoreExistsDefault(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()
	exists, err := store.Exists("default")
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
	url := "unix:///var/run/docker.sock"
	_, err := store.Create("test", "none", &none.CreateFlags{URL: &url})
	if err != nil {
		t.Fatal(err)
	}
	host, err := store.Load("test")
	if host.Name != "test" {
		t.Fatal("Host name is incorrect")
	}
}

func TestStoreLoadDefault(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()

	host, err := store.Load("default")
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "default" {
		t.Fatal("Loading default host failed")
	}
}

func TestStoreGetSetActive(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	store := NewStore()

	// No hosts set
	host, err := store.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "default" {
		t.Fatalf("Active host is not 'default', got %s", host.Name)
	}

	// Set normal host
	url := "unix:///var/run/docker.sock"
	originalHost, err := store.Create("test", "none", &none.CreateFlags{URL: &url})
	if err != nil {
		t.Fatal(err)
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

	// Set default host
	if host, err = store.Load("default"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetActive(host); err != nil {
		t.Fatal(err)
	}

	host, err = store.GetActive()
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "default" {
		t.Fatalf("Active host is not 'default', got %s", host.Name)
	}

}
