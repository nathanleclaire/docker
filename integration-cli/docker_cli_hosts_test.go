package main

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestHostsEnsureHostIsListed(t *testing.T) {
	createCmd := exec.Command(dockerBinary,
		"hosts",
		"create",
		"-o", "url=tcp://10.11.12.13:2375",
		"test")
	out, _, err := runCommandWithOutput(createCmd)
	errorOut(err, t, fmt.Sprintf("creating host failed with errors: %v", err))

	hostsCmd := exec.Command(dockerBinary, "hosts")
	out, _, err = runCommandWithOutput(hostsCmd)
	errorOut(err, t, fmt.Sprintf("listing hosts failed with errors: %v", err))

	if !strings.Contains(out, "test") {
		t.Fatal("hosts should've listed 'test'")
	}

	if !strings.Contains(out, "tcp://10.11.12.13:2375") {
		t.Fatal("hosts should've listed tcp://10.11.12.13:2375")
	}

	logDone("hosts - host is created")
}

func TestHostsEnsureHostConnects(t *testing.T) {
	// Set up server to listen on
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
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

	// Create host which points at server
	createCmd := exec.Command(dockerBinary,
		"hosts",
		"create",
		"-o", "url=tcp://"+ln.Addr().String(),
		"test")
	_, _, err = runCommandWithOutput(createCmd)
	errorOut(err, t, fmt.Sprintf("creating host failed with errors: %v", err))

	// Run command to connect to host
	psCmd := exec.Command(dockerBinary, "-H", "test", "ps")
	_, _, err = runCommandWithOutput(psCmd)

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	select {
	case err = <-ch:
		if err != nil {
			t.Error(err)
		}
	case <-timeout:
		t.Fatal("no connection was made to server")
	}

	logDone("hosts - host connects to server")
}
