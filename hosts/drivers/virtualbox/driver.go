package virtualbox

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"time"

	"github.com/docker/docker/hosts/state"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/utils"
)

var (
	verbose = true
)

type Driver struct {
	MachineName string
	DockerPort  uint
	SSHPort     uint
	Memory      uint // main memory (in MB)
	storePath   string
}

func NewDriver(options map[string]string, storePath string) (*Driver, error) {
	driver := &Driver{storePath: storePath}
	driver.LoadOptions(options)
	return driver, nil
}

func (d *Driver) DriverName() string {
	return "virtualbox"
}

func (d *Driver) GetOptions() map[string]string {
	return map[string]string{"MachineName": d.MachineName}
}

func (d *Driver) LoadOptions(options map[string]string) {
	d.MachineName = options["MachineName"]
	// d.Memory = options["Memory"]
	// if d.Memory == nil {
	d.Memory = 1024
	// }
	d.SSHPort = 2022
	d.DockerPort = 4243
}

func (d *Driver) GetURL() (string, error) {
	return "", nil
}

func (d *Driver) Create() error {
	d.setMachineNameIfNotSet()

	isISODownloaded, err := d.isISODownloaded()
	if err != nil {
		return err
	}
	if !isISODownloaded {
		tag, err := getLatestReleaseName()
		if err != nil {
			return err
		}
		log.Infof("Downloading boot2docker %s...", tag)
		if err := downloadISO(path.Join(d.storePath, "boot2docker.iso"), tag); err != nil {
			return err
		}
	}

	diskPath := path.Join(d.storePath, "disk.vmdk")
	if err := makeDiskImage(diskPath, 10); err != nil {
		return err
	}

	if err := vbm("createvm",
		"--name", d.MachineName,
		"--register"); err != nil {
		return err
	}

	cpus := uint(runtime.NumCPU())
	if cpus > 32 {
		cpus = 32
	}

	if err := vbm("modifyvm", d.MachineName,
		"--firmware", "bios",
		"--bioslogofadein", "off",
		"--bioslogofadeout", "off",
		"--natdnshostresolver1", "on",
		"--bioslogodisplaytime", "0",
		"--biosbootmenu", "disabled",

		"--ostype", "Linux26_64",
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%d", d.Memory),

		"--acpi", "on",
		"--ioapic", "on",
		"--rtcuseutc", "on",
		"--cpuhotplug", "off",
		"--pae", "on",
		"--longmode", "on",
		"--synthcpu", "off",
		"--hpet", "on",
		"--hwvirtex", "on",
		"--triplefaultreset", "off",
		"--nestedpaging", "on",
		"--largepages", "on",
		"--vtxvpid", "on",
		"--vtxux", "off",
		"--accelerate3d", "off",
		"--boot1", "dvd"); err != nil {
		return err
	}

	if err := vbm("modifyvm", d.MachineName,
		"--nic1", "nat",
		"--nictype1", "virtio",
		"--cableconnected1", "on"); err != nil {
		return err
	}

	if err := vbm("storagectl", d.MachineName,
		"--name", "SATA",
		"--add", "sata",
		"--hostiocache", "on"); err != nil {
		return err
	}

	if err := vbm("storageattach", d.MachineName,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", path.Join(d.storePath, "boot2docker.iso")); err != nil {
		return err
	}

	if err := vbm("storageattach", d.MachineName,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "hdd",
		"--medium", diskPath); err != nil {
		return err
	}

	return nil
}

func (d *Driver) Start() error {
	return vbm("startvm", d.MachineName, "--type", "headless")
}

func (d *Driver) Stop() error {
	if err := vbm("controlvm", d.MachineName, "acpipowerbutton"); err != nil {
		return err
	}
	for {
		s, err := d.State()
		if err != nil {
			return err
		}
		if s == state.Running {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	return nil
}

func (d *Driver) Remove() error {
	s, err := d.State()
	if err != nil {
		return err
	}
	if s == state.Running {
		if err := d.Stop(); err != nil {
			return err
		}
	}
	return vbm("unregistervm", "--delete", d.MachineName)
}

func (d *Driver) State() (state.State, error) {
	stdout, stderr, err := vbmOutErr("showvminfo", d.MachineName,
		"--machinereadable")
	if err != nil {
		if reMachineNotFound.FindString(stderr) != "" {
			return state.Unknown, ErrMachineNotExist
		}
		return state.Unknown, err
	}
	re := regexp.MustCompile(`(?m)^VMState="(\w+)"$`)
	groups := re.FindStringSubmatch(stdout)
	if len(groups) < 1 {
		return state.Unknown, nil
	}
	switch groups[1] {
	case "running":
		return state.Running, nil
	case "paused":
		return state.Paused, nil
	case "saved":
		return state.Saved, nil
	case "poweroff", "aborted":
		return state.Stopped, nil
	}
	return state.Unknown, nil
}

func (d *Driver) setMachineNameIfNotSet() {
	if d.MachineName == "" {
		d.MachineName = fmt.Sprintf("docker-host-%s", utils.GenerateRandomID())
	}
}

func (d *Driver) isISODownloaded() (bool, error) {
	if _, err := os.Stat(path.Join(d.storePath, "boot2docker.iso")); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get the latest boot2docker release tag name (e.g. "v0.6.0").
func getLatestReleaseName() (string, error) {
	rsp, err := http.Get("https://api.github.com/repos/boot2docker/boot2docker/releases")
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	var t []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&t); err != nil {
		return "", err
	}
	if len(t) == 0 {
		return "", fmt.Errorf("no releases found")
	}
	return t[0].TagName, nil
}

// Download boot2docker ISO image for the given tag and save it at dest.
func downloadISO(dest, tag string) error {
	rsp, err := http.Get(fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag))
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	// Download to a temp file first then rename it to avoid partial download.
	f, err := ioutil.TempFile("", "boot2docker-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := io.Copy(f, rsp.Body); err != nil {
		// TODO: display download progress?
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), dest); err != nil {
		return err
	}
	return nil
}

// Make a boot2docker VM disk image.
func makeDiskImage(dest string, size int) error {
	log.Debugf("Creating %d MB hard disk image...", size)
	cmd := exec.Command(VBM, "convertfromraw", "stdin", dest, fmt.Sprintf("%d", size*1024*1024), "--format", "VMDK")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	// Write the magic string so the VM auto-formats the disk upon first boot.
	if _, err := w.Write([]byte("boot2docker, please format-me")); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return cmd.Run()
}
