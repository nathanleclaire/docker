package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	homedir "github.com/mitchellh/go-homedir"
)

func TestHostsEnsureHostIsListed(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	createCmd := exec.Command(dockerBinary,
		"hosts",
		"create",
		"--url", "tcp://10.11.12.13:2375",
		"test")
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	hostsCmd := exec.Command(dockerBinary, "hosts")
	out, _, err = runCommandWithOutput(hostsCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	if !strings.Contains(out, "test") {
		t.Fatal("hosts should've listed 'test'")
	}
	if !strings.Contains(out, "tcp://10.11.12.13:2375") {
		t.Fatal("hosts should've listed tcp://10.11.12.13:2375")
	}

	lsCmd := exec.Command(dockerBinary, "hosts", "ls")
	out, _, err = runCommandWithOutput(lsCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	if !strings.Contains(out, "test") {
		t.Fatal("hosts should've listed 'test'")
	}

	urlCmd := exec.Command(dockerBinary, "hosts", "url", "test")
	out, _, err = runCommandWithOutput(urlCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	if !strings.Contains(out, "tcp://10.11.12.13:2375") {
		t.Fatal("hosts should've listed tcp://10.11.12.13:2375")
	}

	inspectCmd := exec.Command(dockerBinary, "hosts", "inspect", "test")
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatal(out, err)
	}
	if !strings.Contains(out, "tcp://10.11.12.13:2375") {
		t.Fatal("hosts should've listed tcp://10.11.12.13:2375")
	}

	logDone("hosts - host is created")
}

func TestHostsEnsureConnectsToActiveHost(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	err := assertConnectionIsMadeToServer(func(addr string) error {
		// Create host which points at server
		createCmd := exec.Command(dockerBinary,
			"hosts",
			"create",
			"--url", addr,
			"test")
		_, _, err := runCommandWithOutput(createCmd)
		if err != nil {
			return err
		}

		// Run command to connect to host
		psCmd := exec.Command(dockerBinary, "ps")
		_, _, err = runCommandWithOutput(psCmd)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	logDone("hosts - connects to active host")
}

func TestHostsEnsureOptionWithNameOverridesActiveHost(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	err := assertConnectionIsMadeToServer(func(addr string) error {
		// Create host which points at server
		createCmd := exec.Command(dockerBinary,
			"hosts",
			"create",
			"--url", addr,
			"test")
		_, _, err := runCommandWithOutput(createCmd)
		if err != nil {
			return err
		}
		activeCmd := exec.Command(dockerBinary, "hosts", "active", "default")
		_, _, err = runCommandWithOutput(activeCmd)
		if err != nil {
			return err
		}

		// Run command to connect to host
		psCmd := exec.Command(dockerBinary, "-H", "test", "ps")
		_, _, err = runCommandWithOutput(psCmd)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	logDone("hosts - host option with name overrides active host")
}

func TestHostsEnsureOptionWithURLOverridesActiveHost(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	err := assertConnectionIsMadeToServer(func(addr string) error {
		// Create host which points at server
		createCmd := exec.Command(dockerBinary,
			"hosts",
			"create",
			"--url", "unix://another/url",
			"test")
		_, _, err := runCommandWithOutput(createCmd)
		if err != nil {
			return err
		}

		// Run command to connect to host
		psCmd := exec.Command(dockerBinary, "-H", addr, "ps")
		_, _, err = runCommandWithOutput(psCmd)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	logDone("hosts - host option with URL overrides active host")

}

func TestHostsEnsureHostIsRemoved(t *testing.T) {
	if err := clearHosts(); err != nil {
		t.Fatal(err)
	}

	createCmd := exec.Command(dockerBinary,
		"hosts",
		"create",
		"--url", "tcp://10.11.12.13:2375",
		"test")
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "hosts"))
	if err != nil {
		t.Fatal(out, err)
	}

	if !strings.Contains(out, "test") {
		t.Fatal("hosts should've listed 'test'")
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary,
		"hosts", "rm", "test",
	))
	if err != nil {
		t.Fatal(out, err)
	}

	if strings.Contains(out, "test") {
		t.Fatal("hosts should not list 'test'")
	}

	logDone("hosts - host is removed")
}

// assertConnectionIsMadeToServer creates a server to listen on and ensures
// that the function inside makes a connection to that server.
func assertConnectionIsMadeToServer(f func(addr string) error) error {
	// Set up server to listen on
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("Listen failed: %v", err)
	}
	defer ln.Close()
	ch := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			ch <- fmt.Errorf("Accept failed: %v", err)
			return
		}
		defer c.Close()
		ch <- nil
	}()

	err = f(fmt.Sprintf("tcp://%s", ln.Addr().String()))

	select {
	case err = <-ch:
		if err != nil {
			return err
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("no connection was made to server")
	}

	return nil
}

func clearHosts() error {
	homeDir, err := homedir.Dir()
	if err != nil {
		return err
	}
	return os.RemoveAll(path.Join(homeDir, ".docker/hosts"))
}
