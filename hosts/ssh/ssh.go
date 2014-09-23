package ssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/docker/pkg/log"
)

func GetSSHCommand(host string, port uint, user string, sshKey string, args ...string) *exec.Cmd {

	defaultSSHArgs := []string{
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts."
		"-p", fmt.Sprintf("%d", port),
		"-i", sshKey,
		fmt.Sprintf("%s@%s", user, host),
	}

	sshArgs := append(defaultSSHArgs, args...)
	cmd := exec.Command("ssh", sshArgs...)
	log.Debugf("executing: %v", strings.Join(cmd.Args, " "))

	return cmd
}

func GenerateSSHKey(path string) error {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		cmd := exec.Command("ssh-keygen", "-t", "rsa", "-N", "", "-f", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Debugf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))

		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func WaitForTCP(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.Read(make([]byte, 1)); err != nil {
		return err
	}
	return nil
}
