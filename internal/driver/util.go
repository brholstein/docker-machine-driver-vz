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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnutils"
	"github.com/pkg/errors"
)

func GetDiskPath(d *drivers.BaseDriver) string {
	return d.ResolveStorePath(d.GetMachineName() + ".rawdisk")
}

func createRawDiskImage(sshKeyPath, diskPath string, diskSizeMb uint) error {
	tarBuf, err := mcnutils.MakeDiskImage(sshKeyPath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(diskPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Seek(0, os.SEEK_SET)

	if _, err := file.Write(tarBuf.Bytes()); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return errors.Wrapf(err, "closing file %s", diskPath)
	}

	if err := os.Truncate(diskPath, int64(diskSizeMb*1000000)); err != nil {
		return err
	}
	return nil
}

func publicSSHKeyPath(d *drivers.BaseDriver) string {
	return d.GetSSHKeyPath() + ".pub"
}

func hdiutil(args ...string) error {
	cmd := exec.Command("hdiutil", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Debugf("executing: %v %v", cmd, strings.Join(args, " "))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func readKernelCommandLine(path string) (string, error) {
	inFile, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer inFile.Close()

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		if kernelCommandLineOptionsRegexp.Match(scanner.Bytes()) {
			m := kernelCommandLineOptionsRegexp.FindSubmatch(scanner.Bytes())
			return string(m[1]), nil
		}
	}
	return "", fmt.Errorf("couldn't find kernel option from %s image", path)
}
