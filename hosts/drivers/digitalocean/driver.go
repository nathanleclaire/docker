package digitalocean

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strconv"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/docker/docker/hosts/ssh"
	"github.com/docker/docker/hosts/state"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/vendor/src/github.com/digitalocean/godo"
)

type Driver struct {
	accessToken string
	storePath   string
	dropletName string
	dropletID   int
	sshKeyID    int
	ipAddress   string
}

func NewDriver(options map[string]string, storePath string) (*Driver, error) {
	driver := &Driver{
		storePath: storePath,
	}
	if err := driver.LoadOptions(options); err != nil {
		return driver, err
	}
	return driver, nil
}

func (d *Driver) DriverName() string {
	return "digitalocean"
}

func (d *Driver) GetOptions() map[string]string {
	return map[string]string{
		"accessToken": d.accessToken,
		"dropletName": d.dropletName,
		"dropletID":   fmt.Sprintf("%d", d.dropletID),
		"sshKeyID":    fmt.Sprintf("%d", d.sshKeyID),
		"ipAddress":   d.ipAddress,
	}
}

func (d *Driver) LoadOptions(options map[string]string) error {
	var (
		ok  bool
		err error
	)

	if d.accessToken, ok = options["accessToken"]; !ok {
		return fmt.Errorf("The digitalocean driver requires the option \"accessToken\". Set it with -o accessToken=...")
	}
	d.dropletName, _ = options["dropletName"]
	d.ipAddress, _ = options["ipAddress"]

	if dropletID, ok := options["dropletID"]; ok {
		d.dropletID, err = strconv.Atoi(dropletID)
		if err != nil {
			return err
		}
	}

	if sshKeyID, ok := options["sshKeyID"]; ok {
		d.sshKeyID, err = strconv.Atoi(sshKeyID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver) Create() error {
	d.setDropletNameIfNotSet()

	key, err := d.createSSHKey()
	if err != nil {
		return err
	}

	d.sshKeyID = key.ID

	client := d.getClient()

	createRequest := &godo.DropletCreateRequest{
		Name:    d.dropletName,
		Region:  "nyc3",
		Size:    "512mb",
		Image:   "docker",
		SSHKeys: []interface{}{d.sshKeyID},
	}

	newDroplet, _, err := client.Droplets.Create(createRequest)
	if err != nil {
		return err
	}

	d.dropletID = newDroplet.Droplet.ID

	for {
		newDroplet, _, err = client.Droplets.Get(d.dropletID)
		if err != nil {
			return err
		}
		for _, network := range newDroplet.Droplet.Networks.V4 {
			if network.Type == "public" {
				d.ipAddress = network.IPAddress
			}
		}

		if d.ipAddress != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	log.Debugf("Created droplet ID %d, IP address %s",
		newDroplet.Droplet.ID,
		d.ipAddress)

	log.Debugf("Waiting for SSH...")

	if err := ssh.WaitForTCP(fmt.Sprintf("%s:%d", d.ipAddress, 22)); err != nil {
		return err
	}

	log.Debugf("Updating /etc/default/docker to listen on all interfaces...")

	cmd := d.GetSSHCommand("echo 'export DOCKER_OPTS=\"--host=tcp://0.0.0.0:2375\"' >> /etc/default/docker")

	if err := cmd.Run(); err != nil {
		return err
	}
	if err := d.GetSSHCommand("restart docker").Run(); err != nil {
		return err
	}

	return nil
}

func (d *Driver) createSSHKey() (*godo.Key, error) {
	if err := ssh.GenerateSSHKey(d.sshKeyPath()); err != nil {
		return nil, err
	}

	publicKey, err := ioutil.ReadFile(d.publicSSHKeyPath())
	if err != nil {
		return nil, err
	}

	createRequest := &godo.KeyCreateRequest{
		Name:      d.dropletName,
		PublicKey: string(publicKey),
	}

	key, _, err := d.getClient().Keys.Create(createRequest)
	if err != nil {
		return key, err
	}

	return key, nil
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2375", ip), nil
}

func (d *Driver) GetIP() (string, error) {
	return d.ipAddress, nil
}

func (d *Driver) GetState() (state.State, error) {
	droplet, _, err := d.getClient().Droplets.Get(d.dropletID)
	if err != nil {
		return state.Unknown, err
	}
	switch droplet.Droplet.Status {
	case "new":
		return state.Starting, nil
	case "active":
		return state.Running, nil
	case "off":
		return state.Stopped, nil
	}
	return state.Unknown, nil
}

func (d *Driver) Start() error {
	_, _, err := d.getClient().DropletActions.PowerOn(d.dropletID)
	return err
}

func (d *Driver) Stop() error {
	_, _, err := d.getClient().DropletActions.Shutdown(d.dropletID)
	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()
	if _, err := client.Keys.DeleteByID(d.sshKeyID); err != nil {
		return err
	}
	if _, err := client.Droplets.Delete(d.dropletID); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Restart() error {
	_, _, err := d.getClient().DropletActions.Reboot(d.dropletID)
	return err
}

func (d *Driver) Kill() error {
	_, _, err := d.getClient().DropletActions.PowerOff(d.dropletID)
	return err
}

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return ssh.GetSSHCommand(d.ipAddress, 22, "root", d.sshKeyPath(), args...)
}

func (d *Driver) setDropletNameIfNotSet() {
	if d.dropletName == "" {
		d.dropletName = fmt.Sprintf("docker-host-%s", utils.GenerateRandomID())
	}
}

func (d *Driver) getClient() *godo.Client {
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: d.accessToken},
	}

	return godo.NewClient(t.Client())
}

func (d *Driver) sshKeyPath() string {
	return path.Join(d.storePath, "id_rsa")
}

func (d *Driver) publicSSHKeyPath() string {
	return d.sshKeyPath() + ".pub"
}
