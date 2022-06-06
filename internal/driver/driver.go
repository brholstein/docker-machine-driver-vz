//go:build darwin
// +build darwin

/*
Copyright 2016 The Kubernetes Authors All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	vznet "github.com/brholstein/docker-machine-driver-vz/internal/net"
	"github.com/brholstein/docker-machine-driver-vz/internal/vz"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"

	ps "github.com/mitchellh/go-ps"

	"github.com/pkg/errors"
)

const (
	defaultCPU            = 1
	defaultMemory         = 1024
	defaultDiskSize       = 20000
	defaultBoot2DockerURL = ""

	defaultSSHUser = "docker"

	isoFileName  = "boot2docker.iso"
	isoMountPath = "b2d-image"

	baseCmdLineOptions = "irqaffinity=0 module_blacklist=vboxguest,vboxsf"

	pidFileName = "vz.pid"
)

var (
	kernelRegexp                   = regexp.MustCompile(`(vmlinu[xz]|bzImage)[\d]*`)
	kernelCommandLineOptionsRegexp = regexp.MustCompile(`(?:\t|\s{2})append\s+([[:print:]]+)`)
)

type Driver struct {
	*drivers.BaseDriver

	CPU      uint
	Memory   uint
	DiskSize uint

	Boot2DockerURL string

	Initrd  string
	Kernel  string
	Cmdline string

	ShareDirectory bool

	MACAddress vznet.HardwareAddr
}

func NewDriver(hostname, storePath string) drivers.Driver {
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: hostname,
			StorePath:   storePath,
		},

		CPU:            defaultCPU,
		Memory:         defaultMemory,
		DiskSize:       defaultDiskSize,
		Boot2DockerURL: defaultBoot2DockerURL,

		ShareDirectory: true,
	}
}

func (d *Driver) PreCreateCheck() error {
	// Downloading boot2docker to cache should be done here to make sure
	// that a download failure will not leave a machine half created.
	b2dutils := mcnutils.NewB2dUtils(d.StorePath)
	if err := b2dutils.UpdateISOCache(d.Boot2DockerURL); err != nil {
		return err
	}

	return nil
}

// Create a host using the driver's config
func (d *Driver) Create() error {
	log.Info("Creating the VM preamble...")

	//TODO(r2d4): rewrite this, not using b2dutils
	b2dutils := mcnutils.NewB2dUtils(d.StorePath)
	if err := b2dutils.CopyIsoToMachineDir(d.Boot2DockerURL, d.MachineName); err != nil {
		return errors.Wrap(err, "Error copying ISO to machine dir")
	}

	log.Info("Extracting kernel...")
	if err := d.extractKernel(); err != nil {
		return errors.Wrap(err, "extracting kernel")
	}

	log.Info("Creating ssh key...")
	if err := ssh.GenerateSSHKey(d.GetSSHKeyPath()); err != nil {
		return errors.Wrap(err, "creating ssh key")
	}

	log.Info("Creating raw disk image...")
	if err := createRawDiskImage(publicSSHKeyPath(d.BaseDriver), GetDiskPath(d.BaseDriver), d.DiskSize); err != nil {
		return errors.Wrap(err, "creating disk image")
	}

	// Must start VM as part of creation.
	return d.Start()
}

// DriverName returns the name of the driver
func (d *Driver) DriverName() string {
	return "vz"
}

// GetCreateFlags returns the mcnflag.Flag slice representing the flags
// that can be set, their descriptions and defaults.
func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.IntFlag{
			EnvVar: "VZ_CPU_COUNT",
			Name:   "vz-cpu-count",
			Usage:  "Number of CPUs for the machine (-1 to use the number of CPUs available)",
			Value:  defaultCPU,
		},
		mcnflag.IntFlag{
			EnvVar: "VZ_MEMORY_SIZE",
			Name:   "vz-memory-size",
			Usage:  "Size of memory for host VM (in MB)",
			Value:  defaultMemory,
		},
		mcnflag.IntFlag{
			EnvVar: "VZ_DISK_SIZE",
			Name:   "vz-disk-size",
			Usage:  "Size of disk for host VM (in MB)",
			Value:  defaultDiskSize,
		},

		mcnflag.StringFlag{
			EnvVar: "VZ_BOOT2DOCKER_URL",
			Name:   "vz-boot2docker-url",
			Usage:  "URL for boot2docker image",
			Value:  "",
		},

		mcnflag.BoolFlag{
			Name:  "vz-no-share-directory",
			Usage: "Disable the mount of your home directory",
		},
	}
}

// GetIP returns an IP or hostname that this host is available at
// e.g. 1.2.3.4 or docker-host-d60b70a14d3a.cloudapp.net
func (d *Driver) GetIP() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", err
	}

	if d.IPAddress != "" {
		return d.IPAddress, nil
	}

	getIP := func() bool {
		var err error
		d.IPAddress, err = GetIPAddressByMACAddress(d.getMacAddress().String())
		if err != nil {
			log.Debug(err)
			return false
		}
		return true
	}

	if err := mcnutils.WaitForSpecific(getIP, 30, 2*time.Second); err != nil {
		return "", errors.Wrap(err, "IP address not found in dhcp leases file")
	}

	return d.IPAddress, nil
}

// GetSSHHostname returns hostname for use with ssh
func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

// GetSSHUsername returns username for use with ssh
func (d *Driver) GetSSHUsername() string {
	if d.SSHUser == "" {
		d.SSHUser = defaultSSHUser
	}

	return d.SSHUser
}

// GetState returns the state that the host is in (running, stopped, etc)
func (d *Driver) GetState() (state.State, error) {
	pid := d.getPid()

	if pid == 0 {
		return state.Stopped, nil
	}

	proc, err := ps.FindProcess(pid)
	if err != nil {
		return state.Error, err
	}

	if proc == nil {
		log.Errorf("vz pid %d not found", pid)
		return state.Error, nil
	}

	if proc.Executable() != "vz" {
		log.Debugf("pid %d is stale, and is being used by %s", pid, proc.Executable())
		return state.Error, nil
	}

	return state.Running, nil
}

// GetURL returns a Docker compatible host URL for connecting to this host
// e.g. tcp://1.2.3.4:2376
func (d *Driver) GetURL() (string, error) {
	if err := drivers.MustBeRunning(d); err != nil {
		return "", err
	}

	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

// Kill stops a host forcefully
func (d *Driver) Kill() error {
	return d.sendSignal(os.Kill)
}

// Remove a host
func (d *Driver) Remove() error {
	if !drivers.MachineInState(d, state.Running)() {
		return nil
	}

	if err := d.Stop(); err != nil {
		return d.Kill()
	}

	return nil
}

// Restart a host. This may just call Stop(); Start() if the provider does not
// have any special restart behaviour.
func (d *Driver) Restart() error {
	// Stop VM gracefully
	if err := d.Stop(); err != nil {
		return err
	}
	// Start it again
	return d.Start()
}

// SetConfigFromFlags configures the driver with the object that was returned
// by RegisterCreateFlags
func (d *Driver) SetConfigFromFlags(opts drivers.DriverOptions) error {
	d.CPU = uint(opts.Int("vz-cpu-count"))
	d.Memory = uint(opts.Int("vz-memory-size"))
	d.DiskSize = uint(opts.Int("vz-disk-size"))

	d.Boot2DockerURL = opts.String("vz-boot2docker-url")

	d.ShareDirectory = !opts.Bool("vz-no-share-directory")

	return nil
}

// Start a host
func (d *Driver) Start() error {
	if err := d.recoverFromUncleanShutdown(); err != nil {
		return nil
	}

	// Unset any saved IP address
	d.IPAddress = ""

	config, err := d.generateVmConfig()
	if err != nil {
		return err
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	configJson := string(configBytes[:])
	log.Debugf("Config string: '%s'", configJson)

	cmd := exec.Command("vz", "--pid", d.ResolveStorePath(pidFileName), configJson)

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "Failed to start VM")
	}

	if err := d.mountSharedDirectories(config); err != nil {
		return err
	}

	if err := cmd.Process.Release(); err != nil {
		return errors.Wrap(err, "Failed to release VM")
	}

	return nil
}

// Stop a host gracefully
func (d *Driver) Stop() error {
	if err := d.sendSignal(os.Interrupt); err != nil {
		return err
	}
	// Maybe give it some time to gracefully shutdown..?

	return nil
}

func (d *Driver) getPid() int {
	pidPath := d.ResolveStorePath(pidFileName)

	// Check file exists
	// Read file
	bytes, err := os.ReadFile(pidPath)
	if err != nil {
		log.Debugf("Unable to read PID file (%s): %s", pidPath, err)
		return 0
	}

	// Parse the PID from the file contents
	pid, err := strconv.Atoi(string(bytes))
	if err != nil {
		log.Errorf("Unable to parse PID")
		return 0
	}

	return pid
}

func (d *Driver) sendSignal(s os.Signal) error {
	if err := drivers.MustBeRunning(d); err != nil {
		return err
	}

	pid := d.getPid()

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if proc == nil {
		return errors.Errorf("vz process not found")
	}

	return proc.Signal(s)
}

//recoverFromUncleanShutdown searches for an existing vz.pid file in
//the machine directory. If it can't find it, a clean shutdown is assumed.
//If it finds the pid file, it checks for a running vz process with that pid
//as the existence of a file might not indicate an unclean shutdown but an actual running
//vz server. If the PID in the pidfile does not belong to a running vz
//process, we can safely delete it, and there is a good chance the machine will recover when restarted.
func (d *Driver) recoverFromUncleanShutdown() error {
	pidFile := d.ResolveStorePath(pidFileName)

	if _, err := os.Stat(pidFile); err != nil {
		if os.IsNotExist(err) {
			log.Debugf("clean start, vz pid file doesn't exist: %s", pidFile)
			return nil
		}
		return errors.Wrap(err, "stat")
	}

	log.Warnf("vz pid file still exists: %s", pidFile)
	bytes, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return errors.Wrapf(err, "reading pidfile %s", pidFile)
	}

	content := strings.TrimSpace(string(bytes))
	pid, err := strconv.Atoi(content)
	if err != nil {
		return errors.Wrapf(err, "parsing pidfile %s", pidFile)
	}

	st, err := d.GetState()
	if err != nil {
		return errors.Wrap(err, "pidState")
	}

	log.Debugf("pid %d is in state %q", pid, st)
	if st == state.Running {
		return nil
	}

	log.Debugf("Removing stale pid file %s...", pidFile)
	if err := os.Remove(pidFile); err != nil {
		return errors.Wrap(err, fmt.Sprintf("removing pidFile %s", pidFile))
	}

	return nil
}

func (d *Driver) extractKernel() error {
	log.Debugf("Mounting %s", isoFileName)

	volumeRootDir := d.ResolveStorePath(isoMountPath)
	err := hdiutil("attach", d.ResolveStorePath(isoFileName), "-mountpoint", volumeRootDir)
	if err != nil {
		return err
	}
	defer func() error {
		log.Debugf("Unmounting %s", isoFileName)
		return hdiutil("detach", volumeRootDir)
	}()

	log.Debugf("Extracting Kernel Command Line Options...")
	if err := d.extractKernelCommandLineOptions(); err != nil {
		return err
	}

	isoKernel := ""
	isoInitrd := ""
	if d.Kernel == "" && d.Initrd == "" {
		filepath.Walk(volumeRootDir, func(path string, f os.FileInfo, err error) error {
			if kernelRegexp.MatchString(path) {
				isoKernel = path
				_, d.Kernel = filepath.Split(isoKernel)
			}
			if strings.Contains(path, "initrd") {
				isoInitrd = path
				_, d.Initrd = filepath.Split(isoInitrd)
			}
			return nil
		})
	}

	if d.Kernel == "" || d.Initrd == "" {
		err := fmt.Errorf("Unable to locate Kernel and/or Initial Ramdisk file(s)")
		return err
	}

	dest := d.ResolveStorePath(d.Kernel)
	log.Debugf("Extracting %s into %s", isoKernel, dest)
	if err := mcnutils.CopyFile(isoKernel, dest); err != nil {
		return err
	}

	dest = d.ResolveStorePath(d.Initrd)
	log.Debugf("Extracting %s into %s", isoInitrd, dest)
	if err := mcnutils.CopyFile(isoInitrd, dest); err != nil {
		return err
	}

	return nil
}

func (d *Driver) extractKernelCommandLineOptions() error {
	volumeRootDir := d.ResolveStorePath(isoMountPath)

	var cmdLine string
	extractKernelCommandLineOptions := func(path string, f os.FileInfo, err error) error {
		if strings.Contains(path, "isolinux.cfg") {
			cmdLine, err = readKernelCommandLine(path)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := filepath.Walk(volumeRootDir, extractKernelCommandLineOptions)
	if err != nil {
		return err
	}

	if cmdLine == "" {
		return errors.New("Not able to parse isolinux.cfg")
	}
	log.Debugf("Extracted Options %q", cmdLine)

	d.Cmdline = cmdLine

	return nil
}

func (d *Driver) generateVmConfig() (*vz.VirtualMachineConfig, error) {
	networkInterfaces := make([]vz.VirtualMachineNetworkInterface, 1)
	if d.getMacAddress() == nil {
		d.setMacAddress(vz.NewRandomLocallyAdministeredHardwareAddr())
	}
	networkInterfaces[0].MACAddress = d.MACAddress

	sharedDirectories := []vz.VirtualMachineSharedDirectory{}
	if d.ShareDirectory {
		directory, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		sharedDirectories = append(sharedDirectories, vz.VirtualMachineSharedDirectory{
			Directory: directory,
			Tag:       "Home",
		})
	}

	config := vz.VirtualMachineConfig{
		CPUs:   d.CPU,
		Memory: d.Memory,
		Disks: []vz.VirtualMachineDiskConfig{
			{
				Path:     d.ResolveStorePath(isoFileName),
				ReadOnly: true,
			},
			{
				Path:     GetDiskPath(d.BaseDriver),
				ReadOnly: false,
			},
		},
		Kernel:            d.ResolveStorePath(d.Kernel),
		Initrd:            d.ResolveStorePath(d.Initrd),
		CmdLine:           fmt.Sprintln(baseCmdLineOptions, d.Cmdline),
		NetworkInterfaces: networkInterfaces,
		SharedDirectories: sharedDirectories,
	}

	return &config, nil
}

func (d *Driver) mountSharedDirectories(config *vz.VirtualMachineConfig) error {
	if len(config.SharedDirectories) > 0 {
		mountCommands := fmt.Sprintf("#!/bin/sh\\n")

		for _, sharedDirectory := range config.SharedDirectories {
			mountCommands += fmt.Sprintf("sudo mkdir -p \"%s\" && ", sharedDirectory.Directory)
			mountCommands += fmt.Sprintf("sudo mount -t virtiofs \"%s\" \"%s\"\\n", sharedDirectory.Tag, sharedDirectory.Directory)
		}

		writeScriptCmd := fmt.Sprintf("echo -e \"%s\" | sh", mountCommands)

		mountSharedDirectories := func() bool {
			if msg, err := drivers.RunSSHCommandFromDriver(d, writeScriptCmd); err != nil {
				log.Debug(msg)
				return false
			}
			return true
		}

		if err := drivers.WaitForSSH(d); err != nil {
			return err
		}

		if err := mcnutils.WaitForSpecific(mountSharedDirectories, 6, 5*time.Second); err != nil {
			return fmt.Errorf("Unable to mount shared directories %v", err)
		}
	}

	return nil
}

func (d *Driver) getMacAddress() net.HardwareAddr {
	return d.MACAddress.ToNetHardwareAddr()
}

func (d *Driver) setMacAddress(a net.HardwareAddr) {
	d.MACAddress = vznet.FromNetHardwareAddr(a)
}
