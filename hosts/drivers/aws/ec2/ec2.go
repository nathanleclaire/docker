package ec2

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/hosts/drivers"
	"github.com/docker/docker/hosts/drivers/aws"
	"github.com/docker/docker/hosts/ssh"
	"github.com/docker/docker/hosts/state"
	"github.com/docker/docker/pkg/log"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	awsauth "github.com/smartystreets/go-aws-auth"
)

type Driver struct {
	Auth          aws.Auth
	Endpoint      string
	ImageId       string
	InstanceId    string
	InstanceName  string
	InstanceType  string
	KeyName       string
	PublicDnsName string
	IPAddress     string
	SecurityGroup string
	Region        string
	Username      string
	storePath     string
}

type CreateFlags struct {
	AccessKey     *string
	SecretKey     *string
	ImageId       *string
	InstanceName  *string
	Region        *string
	Username      *string
	SecurityGroup *string
	InstanceType  *string
}

type Instance struct {
	info aws.Ec2Instance
}

func init() {
	drivers.Register("ec2", &drivers.RegisteredDriver{
		New:                 NewDriver,
		RegisterCreateFlags: RegisterCreateFlags,
	})
}

const (
	// "Ubuntu 14.04 LTS with Docker and Runit"
	DEFAULT_IMAGE_ID       string = "ami-27939962"
	DEFAULT_INSTANCE_TYPE  string = "t1.micro"
	DEFAULT_SSH_USERNAME   string = "ubuntu"
	DEFAULT_SECURITY_GROUP string = "docker-hosts"
	DEFAULT_REGION         string = "us-west-1"
)

// RegisterCreateFlags registers the flags this driver adds to
// "docker hosts create"
func RegisterCreateFlags(cmd *flag.FlagSet) interface{} {
	createFlags := new(CreateFlags)
	createFlags.AccessKey = cmd.String(
		[]string{"-aws-access-key"},
		"",
		"AWS Access Key",
	)
	createFlags.SecretKey = cmd.String(
		[]string{"-aws-secret-key"},
		"",
		"AWS Secret Key",
	)
	createFlags.ImageId = cmd.String(
		[]string{"-aws-image-id"},
		DEFAULT_IMAGE_ID,
		"AMI to use for the selected region",
	)
	createFlags.InstanceType = cmd.String(
		[]string{"-aws-instance-name"},
		"",
		"Name of created instance",
	)
	createFlags.Region = cmd.String(
		[]string{"-aws-region"},
		DEFAULT_REGION,
		"AWS Region",
	)
	createFlags.InstanceType = cmd.String(
		[]string{"-aws-instance-type"},
		DEFAULT_INSTANCE_TYPE,
		"Type of instance to create",
	)
	createFlags.Username = cmd.String(
		[]string{"-aws-instance-username"},
		DEFAULT_SSH_USERNAME,
		"Username for SSH on the instance (depends on AMI)",
	)

	// TODO: user should be able to specify multiple security groups
	// also, the default really should be that we create one automatically
	// and/or lookup to see if there's an existing one for our use case
	createFlags.SecurityGroup = cmd.String(
		[]string{"-aws-security-group"},
		DEFAULT_SECURITY_GROUP,
		"Security group to use for the created instance",
	)
	return createFlags
}

func NewDriver(storePath string) (drivers.Driver, error) {
	return &Driver{storePath: storePath}, nil
}

func (d *Driver) DriverName() string {
	return "aws"
}

func (d *Driver) GetURL() (string, error) {
	if d.PublicDnsName == "" {
		return "", fmt.Errorf("Public URL does not exist yet")
	}
	return fmt.Sprintf("tcp://%s:2375", d.PublicDnsName), nil
}

func (d *Driver) GetIP() (string, error) {
	if d.IPAddress == "" {
		return "", fmt.Errorf("IP Address does not exist yet")
	}
	return d.IPAddress, nil
}

func (d *Driver) SetConfigFromFlags(flagsInterface interface{}) error {
	flags := flagsInterface.(*CreateFlags)
	d.Auth = aws.Auth{
		*flags.AccessKey,
		*flags.SecretKey,
	}
	d.ImageId = *flags.ImageId
	d.Region = *flags.Region
	d.InstanceType = *flags.InstanceType
	d.Endpoint = fmt.Sprintf("https://ec2.%s.amazonaws.com", d.Region)
	d.Username = *flags.Username
	d.SecurityGroup = *flags.SecurityGroup

	if (d.Region == DEFAULT_REGION) != (d.ImageId == DEFAULT_IMAGE_ID) {
		return fmt.Errorf("Setting --aws-region without setting --aws-ami is disallowed")
	}

	if d.Auth.AccessKey == "" || d.Auth.SecretKey == "" {
		var err error
		d.Auth, err = aws.EnvAuth()
		if err != nil {
			return fmt.Errorf("Error setting the AWS_ACCESS_TOKEN and AWS_SECRET_KEY environment variables :%s", err)
		}
	} else {
		d.Auth.AccessKey = *flags.AccessKey
		d.Auth.SecretKey = *flags.SecretKey
	}

	return nil
}

func (d *Driver) Create() error {
	d.setInstanceNameIfNotSet()

	log.Infof("Creating key pair...")
	if err := d.createKeyPair(); err != nil {
		return fmt.Errorf("Error creating key pair: %s", err)
	}

	log.Infof("Creating AWS EC2 instance...")
	instance, err := d.runInstance()
	if err != nil {
		return fmt.Errorf("Error running the EC2 instance: %s", err)
	}

	d.InstanceId = instance.info.InstanceId

	if err := d.provision(); err != nil {
		return fmt.Errorf("Error provisioning instance: %s", err)
	}

	log.Infof("Tagging instance %s", d.InstanceName)
	if err := d.tagInstance("Name", d.InstanceName); err != nil {
		return fmt.Errorf("Error tagging instance name: %s", err)
	}

	return nil
}

func (d *Driver) provision() error {
	log.Infof("Waiting for SSH to become available...")
	ticker := time.NewTicker(1 * time.Second)
	for {
		// wait for instance to come up
		fmt.Print(".")
		<-ticker.C
		instanceState, err := d.GetState()
		if err != nil {
			return fmt.Errorf("Error getting instance state: %s", err)
		}
		if instanceState == state.Running {
			fmt.Println()
			break
		}
	}

	log.Infof("Provisioning instance...")

	// change daemon options
	if err := d.GetSSHCommand("echo 'DOCKER_OPTS=\"--host 0.0.0.0:2375\"' | sudo tee /etc/default/docker").Run(); err != nil {
		return fmt.Errorf("Error running command to add daemon options over SSH : %s", err)
	}

	// restart daemon
	if err := d.GetSSHCommand("sudo service docker restart").Run(); err != nil {
		return fmt.Errorf("Error restarting docker service over SSH : %s", err)
	}

	return nil
}

func (d *Driver) tagInstance(key string, val string) error {
	v := url.Values{}
	v.Set("Action", "CreateTags")
	v.Set("ResourceId.1", d.InstanceId)
	v.Set("Tag.1.Key", key)
	v.Set("Tag.1.Value", val)
	if _, err := d.makeAwsApiCall(v); err != nil {
		return NewApiCallError(err)
	}
	return nil
}

func (d *Driver) createKeyPair() error {
	d.KeyName = fmt.Sprintf("%s-key", d.InstanceName)
	v := url.Values{}
	v.Set("Action", "CreateKeyPair")
	v.Set("KeyName", d.KeyName)
	resp, err := d.makeAwsApiCall(v)
	if err != nil {
		return fmt.Errorf("Error trying API call to create keypair: %s", err)
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading AWS response body")
	}

	unmarshalledResponse := aws.CreateKeyPairResponse{}
	if xml.Unmarshal(contents, &unmarshalledResponse); err != nil {
		return fmt.Errorf("Error unmarshalling AWS response XML: %s", err)
	}

	key := unmarshalledResponse.KeyMaterial
	if err := ioutil.WriteFile(d.sshKeyPath(), key, 0400); err != nil {
		return fmt.Errorf("Error writing SSH key to file: %s", err)
	}

	return nil
}

func (d *Driver) setInstanceNameIfNotSet() {
	if d.InstanceName == "" {
		d.InstanceName = fmt.Sprintf("docker-host-%s", utils.GenerateRandomID())
	}
}

func (d *Driver) makeAwsApiCall(v url.Values) (http.Response, error) {
	v.Set("Version", "2014-06-15")
	client := &http.Client{}
	finalEndpoint := fmt.Sprintf("%s?%s", d.Endpoint, v.Encode())
	req, err := http.NewRequest("GET", finalEndpoint, nil)
	if err != nil {
		return http.Response{}, fmt.Errorf("Error creating request from client")
	}
	req.Header.Add("Content-type", "application/json")
	awsauth.Sign(req, awsauth.Credentials{
		AccessKeyID:     d.Auth.AccessKey,
		SecretAccessKey: d.Auth.SecretKey,
	})
	resp, err := client.Do(req)
	if err != nil {
		return http.Response{}, fmt.Errorf("Client encountered error while doing the request: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		baseErr := "Non-200 API Response"
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return http.Response{}, fmt.Errorf("%s and there was an error decoding the response body: %d %s", baseErr, resp.StatusCode, err)
		}
		return http.Response{}, fmt.Errorf("%s : %d\n%s", baseErr, resp.StatusCode, string(content))
	}
	return *resp, nil
}

func NewApiCallError(err error) error {
	return fmt.Errorf("Problem with AWS API call: %s", err)
}

func (d *Driver) runInstance() (Instance, error) {
	instance := Instance{}
	v := url.Values{}
	v.Set("Action", "RunInstances")
	v.Set("ImageId", d.ImageId)
	v.Set("Placement.AvailabilityZone", d.Region+"a")
	v.Set("SecurityGroup.1", d.SecurityGroup)
	v.Set("MinCount", "1")
	v.Set("MaxCount", "1")
	v.Set("KeyName", d.KeyName)
	resp, err := d.makeAwsApiCall(v)
	if err != nil {
		return instance, NewApiCallError(err)
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return instance, fmt.Errorf("Error reading AWS response body")
	}
	unmarshalledResponse := aws.RunInstancesResponse{}
	err = xml.Unmarshal(contents, &unmarshalledResponse)
	if err != nil {
		return instance, fmt.Errorf("Error unmarshalling AWS response XML: %s", err)
	}

	instance.info = unmarshalledResponse.Instances[0]
	return instance, nil
}

func (d *Driver) performStandardAction(action string) (http.Response, error) {
	v := url.Values{}
	v.Set("Action", action)
	v.Set("InstanceId.1", d.InstanceId)
	resp, err := d.makeAwsApiCall(v)
	if err != nil {
		return resp, NewApiCallError(err)
	}
	return resp, nil
}

func (d *Driver) GetState() (state.State, error) {
	resp, err := d.performStandardAction("DescribeInstances")
	if err != nil {
		return state.Error, err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return state.Error, fmt.Errorf("Error reading AWS response body: %s", err)
	}

	unmarshalledResponse := aws.DescribeInstancesResponse{}
	if err = xml.Unmarshal(contents, &unmarshalledResponse); err != nil {
		return state.Error, fmt.Errorf("Error unmarshalling AWS response XML: %s", err)
	}

	reservationSet := unmarshalledResponse.ReservationSet[0]
	instanceState := reservationSet.InstancesSet[0].InstanceState
	networkInterfaceSet := reservationSet.InstancesSet[0].NetworkInterfaceSet

	association := networkInterfaceSet[0].Association
	d.IPAddress = association.PublicIp
	d.PublicDnsName = association.PublicDnsName

	shortState := strings.TrimSpace(instanceState.Name)
	switch shortState {
	case "pending":
		return state.Starting, nil
	case "running":
		return state.Running, nil
	case "stopped":
		return state.Stopped, nil
	}

	return state.Error, nil
}

// TODO: Do something useful with the following API responses
//       which are currently just getting discarded?
func (d *Driver) Remove() error {
	if _, err := d.performStandardAction("TerminateInstances"); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Start() error {
	if _, err := d.performStandardAction("StartInstances"); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Stop() error {
	if _, err := d.performStandardAction("StopInstances"); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Restart() error {
	if _, err := d.performStandardAction("RebootInstances"); err != nil {
		return err
	}
	return nil
}

func (d *Driver) Kill() error {
	// Not really anything like a hard power-off / kill
	// in the AWS API that I can find.  Perhaps I am wrong!
	if _, err := d.performStandardAction("StopInstances"); err != nil {
		return err
	}
	return nil
}

func (d *Driver) sshKeyPath() string {
	return path.Join(d.storePath, fmt.Sprintf("%s.pem", d.KeyName))
}

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return ssh.GetSSHCommand(d.IPAddress, 22, d.Username, d.sshKeyPath(), args...)
}
