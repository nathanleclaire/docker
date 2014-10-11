package ec2

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"

	"github.com/docker/docker/hosts/drivers"
	"github.com/docker/docker/hosts/state"
	flag "github.com/docker/docker/pkg/mflag"
	awsauth "github.com/smartystreets/go-aws-auth"
)

type Driver struct {
	auth         Auth
	endpoint     string
	AccessKey    string
	ImageId      string
	InstanceType string
	Region       string
	SecretKey    string
}

type CreateFlags struct {
	AccessKey    *string
	SecretKey    *string
	ImageId      *string
	Region       *string
	InstanceType *string
}

type Instance struct {
	info io.ReadCloser
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
	return createFlags
}

func NewDriver(storePath string) (drivers.Driver, error) {
	return &Driver{}, nil
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
	d.endpoint = fmt.Sprintf("https://ec2.%s.amazonaws.com", d.Region)

	if (d.Region == DEFAULT_REGION) != (d.ImageId == DEFAULT_IMAGE_ID) {
		return fmt.Errorf("Setting --aws-region without setting --aws-ami is disallowed")
	}

	if d.AccessKey == "" || d.SecretKey == "" {
		var err error
		d.auth, err = EnvAuth()
		if err != nil {
			return fmt.Errorf("Setting the AWS_ACCESS_TOKEN and AWS_SECRET_KEY environment variables or the --aws-access-token and --aws-secret-key flags")
		}
	} else {
		d.auth.AccessKey = *flags.AccessKey
		d.auth.SecretKey = *flags.SecretKey
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
	return state.Stopped, nil
}

func (d *Driver) Create() error {
	instance, err := d.runInstance()
	if err != nil {
		return fmt.Errorf("Error running the EC2 instance: %s", err)
	}
	fmt.Println(instance)
	return nil
}

func (d *Driver) runInstance() (Instance, error) {
	instance := Instance{}
	v := url.Values{}
	v.Set("Action", "RunInstances")
	v.Set("ImageId", d.ImageId)
	v.Set("Version", "2014-06-15")
	v.Set("Placement.AvailabilityZone", d.Region+"a")
	v.Set("MinCount", "1")
	v.Set("MaxCount", "1")
	client := &http.Client{}
	finalEndpoint := fmt.Sprintf("%s?%s", d.endpoint, v.Encode())
	req, err := http.NewRequest("GET", finalEndpoint, nil)
	if err != nil {
		return Instance{}, fmt.Errorf("Error creating request from client")
	}
	req.Header.Add("Content-type", "application/json")
	awsauth.Sign(req, awsauth.Credentials{
		AccessKeyID:     d.AccessKey,
		SecretAccessKey: d.SecretKey,
	})
	resp, err := client.Do(req)
	if err != nil {
		return Instance{}, fmt.Errorf("Problem with HTTPS request to %s", finalEndpoint)
	}
	instance.info = resp.Body

	return instance, nil
}

func (d *Driver) Remove() error {
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

func (d *Driver) GetSSHCommand(args ...string) *exec.Cmd {
	return &exec.Cmd{}
}
