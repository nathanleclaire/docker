package virtualbox

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/utils"
)

var (
	verbose = true
)

type Flag int

// Flag names in lowercases to be consistent with VBoxManage options.
const (
	F_acpi Flag = 1 << iota
	F_ioapic
	F_rtcuseutc
	F_cpuhotplug
	F_pae
	F_longmode
	F_synthcpu
	F_hpet
	F_hwvirtex
	F_triplefaultreset
	F_nestedpaging
	F_largepages
	F_vtxvpid
	F_vtxux
	F_accelerate3d
)

type Driver struct {
	MachineName string
	DockerPort  uint
	SSHPort     uint
	CPUs        uint
	Memory      uint // main memory (in MB)
	VRAM        uint // video memory (in MB)
	OSType      string
	Flag        Flag
	BootOrder   []string // max 4 slots, each in {none|floppy|dvd|disk|net}
}

func NewDriver(options map[string]string) (*Driver, error) {
	driver := &Driver{}
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
}

func (d *Driver) GetURL() string {
	return ""
}

func (d *Driver) Create() error {
	d.setMachineNameIfNotSet()

	args := []string{"createvm", "--name", d.MachineName, "--register"}
	if err := vbm(args...); err != nil {
		return err
	}

	if err := d.Load(); err != nil {
		return err
	}

	return nil
}

func (d *Driver) Load() error {
	stdout, stderr, err := vbmOutErr("showvminfo", d.MachineName, "--machinereadable")
	if err != nil {
		if reMachineNotFound.FindString(stderr) != "" {
			return ErrMachineNotExist
		}
		return err
	}
	s := bufio.NewScanner(strings.NewReader(stdout))
	for s.Scan() {
		res := reVMInfoLine.FindStringSubmatch(s.Text())
		if res == nil {
			continue
		}
		key := res[1]
		if key == "" {
			key = res[2]
		}
		val := res[3]
		if val == "" {
			val = res[4]
		}

		switch key {
		// case "VMState":
		// 	d.State = driver.MachineState(val)
		case "memory":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return err
			}
			d.Memory = uint(n)
		case "cpus":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return err
			}
			d.CPUs = uint(n)
		case "vram":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return err
			}
			d.VRAM = uint(n)
		// case "CfgFile":
		// 	m.CfgFile = val
		// 	m.BaseFolder = filepath.Dir(val)
		// case "uartmode1":
		// 	// uartmode1="server,/home/sven/.boot2docker/boot2docker-vm.sock"
		// 	vals := strings.Split(val, ",")
		// 	if len(vals) >= 2 {
		// 		m.SerialFile = vals[1]
		// 	}
		default:
			if strings.HasPrefix(key, "Forwarding(") {
				// "Forwarding(\d*)" are ordered by the name inside the val, not fixed order.
				// Forwarding(0)="docker,tcp,127.0.0.1,5555,,"
				// Forwarding(1)="ssh,tcp,127.0.0.1,2222,,22"
				vals := strings.Split(val, ",")
				n, err := strconv.ParseUint(vals[3], 10, 32)
				if err != nil {
					return err
				}
				switch vals[0] {
				case "docker":
					d.DockerPort = uint(n)
				case "ssh":
					d.SSHPort = uint(n)
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}

func (d *Driver) setMachineNameIfNotSet() {
	if d.MachineName == "" {
		d.MachineName = fmt.Sprintf("docker-host-%s", utils.GenerateRandomID())
	}
}

func (d *Driver) setExtra(key, val string) error {
	return vbm("setextradata", d.MachineName, key, val)
}
