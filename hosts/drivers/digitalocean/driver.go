package digitalocean

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/docker/docker/hosts/ssh"
	"github.com/docker/docker/hosts/state"
	"github.com/docker/docker/pkg/log"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/vendor/src/github.com/digitalocean/godo"
)

type Driver struct {
	AccessToken string
	DropletName string
	DropletID   int
	SSHKeyID    int
	IPAddress   string
	storePath   string
}

type CreateFlags struct {
	AccessToken *string
}

// RegisterCreateFlags registers the flags this driver adds to
// "docker hosts create"
func RegisterCreateFlags(cmd *flag.FlagSet) *CreateFlags {
	createFlags := new(CreateFlags)
	createFlags.AccessToken = cmd.String([]string{"-digitalocean-access-token"}, "", "Digital Ocean access token")
	return createFlags
}

func NewDriver(storePath string) (*Driver, error) {
	return &Driver{storePath: storePath}, nil
}

func (d *Driver) DriverName() string {
	return "digitalocean"
}

func (d *Driver) SetConfigFromFlags(flagsInterface interface{}) error {
	flags := flagsInterface.(*CreateFlags)
	d.AccessToken = *flags.AccessToken
	if d.AccessToken == "" {
		return fmt.Errorf("digitalocean driver requires the --digitalocean-access-token option")
	}
	return nil
}

func (d *Driver) Create() error {
	d.setDropletNameIfNotSet()

	key, err := d.createSSHKey()
	if err != nil {
		return err
	}

	d.SSHKeyID = key.ID

	client := d.getClient()

	createRequest := &godo.DropletCreateRequest{
		Name:    d.DropletName,
		Region:  "nyc3",
		Size:    "512mb",
		Image:   "docker",
		SSHKeys: []interface{}{d.SSHKeyID},
	}

	newDroplet, _, err := client.Droplets.Create(createRequest)
	if err != nil {
		return err
	}

	d.DropletID = newDroplet.Droplet.ID

	for {
		newDroplet, _, err = client.Droplets.Get(d.DropletID)
		if err != nil {
			return err
		}
		for _, network := range newDroplet.Droplet.Networks.V4 {
			if network.Type == "public" {
				d.IPAddress = network.IPAddress
			}
		}

		if d.IPAddress != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	log.Debugf("Created droplet ID %d, IP address %s",
		newDroplet.Droplet.ID,
		d.IPAddress)

	log.Debugf("Waiting for SSH...")

	if err := ssh.WaitForTCP(fmt.Sprintf("%s:%d", d.IPAddress, 22)); err != nil {
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
		Name:      d.DropletName,
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
	return d.IPAddress, nil
}

func (d *Driver) GetState() (state.State, error) {
	droplet, _, err := d.getClient().Droplets.Get(d.DropletID)
	if err != nil {
		return state.None, err
	}
	switch droplet.Droplet.Status {
	case "new":
		return state.Starting, nil
	case "active":
		return state.Running, nil
	case "off":
		return state.Stopped, nil
	}
	return state.None, nil
}

func (d *Driver) Start() error {
	_, _, err := d.getClient().DropletActions.PowerOn(d.DropletID)
	return err
}

func (d *Driver) Stop() error {
	_, _, err := d.getClient().DropletActions.Shutdown(d.DropletID)
	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()
	if _, err := client.Keys.DeleteByID(d.SSHKeyID); err != nil {
		return err
	}
	if _, err := client.Droplets.Delete(d.DropletID); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Restart() error {
	_, _, err := d.getClient().DropletActions.Reboot(d.DropletID)
	return err
}

func (d *Driver) Kill() error {
	_, _, err := d.getClient().DropletActions.PowerOff(d.DropletID)
	return err
}

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return ssh.GetSSHCommand(d.IPAddress, 22, "root", d.sshKeyPath(), args...)
}

func (d *Driver) setDropletNameIfNotSet() {
	if d.DropletName == "" {
		d.DropletName = fmt.Sprintf("docker-host-%s", utils.GenerateRandomID())
	}
}

func (d *Driver) getClient() *godo.Client {
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: d.AccessToken},
	}

	return godo.NewClient(t.Client())
}

func (d *Driver) sshKeyPath() string {
	return path.Join(d.storePath, "id_rsa")
}

func (d *Driver) publicSSHKeyPath() string {
	return d.sshKeyPath() + ".pub"
}
