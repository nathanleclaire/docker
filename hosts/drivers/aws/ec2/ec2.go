package ec2

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"path"

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
	Auth         aws.Auth
	Endpoint     string
	AccessKey    string
	ImageId      string
	InstanceId   string
	InstanceName string
	InstanceType string
	IPAddress    string
	Region       string
	SecretKey    string
	Username     string
	storePath    string
}

type CreateFlags struct {
	AccessKey    *string
	SecretKey    *string
	ImageId      *string
	InstanceName *string
	Region       *string
	Username     *string
	InstanceType *string
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

const DEFAULT_REGION string = "us-west-1"

// "Ubuntu 14.04 LTS with Docker and Runit"
const DEFAULT_IMAGE_ID string = "ami-014f4144"
const DEFAULT_INSTANCE_TYPE string = "t1.micro"
const DEFAULT_SSH_USERNAME string = "ubuntu"

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
	return createFlags
}

func NewDriver(storePath string) (drivers.Driver, error) {
	return &Driver{storePath: storePath}, nil
}

func (d *Driver) DriverName() string {
	return "aws"
}

func (d *Driver) SetConfigFromFlags(flagsInterface interface{}) error {
	flags := flagsInterface.(*CreateFlags)
	d.AccessKey = *flags.AccessKey
	d.SecretKey = *flags.SecretKey
	d.ImageId = *flags.ImageId
	d.Region = *flags.Region
	d.InstanceType = *flags.InstanceType
	d.Endpoint = fmt.Sprintf("https://ec2.%s.amazonaws.com", d.Region)

	if (d.Region == DEFAULT_REGION) != (d.ImageId == DEFAULT_IMAGE_ID) {
		return fmt.Errorf("Setting --aws-region without setting --aws-ami is disallowed")
	}

	if d.AccessKey == "" || d.SecretKey == "" {
		var err error
		d.Auth, err = aws.EnvAuth()
		if err != nil {
			return fmt.Errorf("Setting the AWS_ACCESS_TOKEN and AWS_SECRET_KEY environment variables or the --aws-access-token and --aws-secret-key flags")
		}
	} else {
		d.Auth.AccessKey = *flags.AccessKey
		d.Auth.SecretKey = *flags.SecretKey
	}

	return nil
}

func (d *Driver) GetURL() (string, error) {
	return "", nil
}

func (d *Driver) GetIP() (string, error) {
	return "", nil
}

func (d *Driver) GetState() (state.State, error) {
	v := url.Values{}
	v.Set("Action", "DescribeInstances")
	v.Set("InstanceId.1", d.InstanceName)
	return state.Stopped, nil
}

func (d *Driver) Create() error {
	d.setInstanceNameIfNotSet()
	log.Infof("Creating AWS EC2 instance...")
	instance, err := d.runInstance()
	if err != nil {
		return fmt.Errorf("Error running the EC2 instance: %s", err)
	}
	d.InstanceId = instance.info.InstanceId
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
		AccessKeyID:     d.AccessKey,
		SecretAccessKey: d.SecretKey,
	})
	resp, err := client.Do(req)
	if err != nil {
		return http.Response{}, fmt.Errorf("Client encountered error while doing the request: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return http.Response{}, fmt.Errorf("Non-200 API Response : \n%s", resp.StatusCode)
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
	v.Set("MinCount", "1")
	v.Set("MaxCount", "1")
	resp, err := d.makeAwsApiCall(v)
	defer resp.Body.Close()
	if err != nil {
		return instance, NewApiCallError(err)
	}
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return instance, fmt.Errorf("Error reading AWS response body")
	}
	unmarshalledResponse := aws.RunInstancesResponse{}
	err = xml.Unmarshal(contents, &unmarshalledResponse)
	if err != nil {
		return instance, fmt.Errorf("Error unmarshalling AWS response XML: %s")
	}

	instance.info = unmarshalledResponse.Instances[0]
	return instance, nil
}

func (d *Driver) Remove() error {
	log.Infof("Removing AWS EC2 instance...")
	v := url.Values{}
	v.Set("Action", "TerminateInstances")
	v.Set("InstanceId.1", d.InstanceId)
	resp, err := d.makeAwsApiCall(v)
	if err != nil {
		return NewApiCallError(err)
	}
	defer resp.Body.Close()
	return nil
}

func (d *Driver) Start() error {
	return nil
}

func (d *Driver) Stop() error {
	return nil

}

func (d *Driver) Restart() error {
	return nil

}

func (d *Driver) Kill() error {
	return nil

}

func (d *Driver) sshKeyPath() string {
	return path.Join(d.storePath, "id_rsa")
}

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return ssh.GetSSHCommand(d.IPAddress, 22, d.Username, d.sshKeyPath(), args...)
}
