/*
(c) Copyright 2017 Hewlett Packard Enterprise Development LP

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

package linux

import (
	"errors"
	"fmt"
	"github.com/hpe-storage/dory/common/model"
	"github.com/hpe-storage/dory/common/util"
	"strconv"
)

const (
	mountUUIDErr = 32
)

// MountDeviceWithFileSystem : Mount device with filesystem at the mountPoint
func MountDeviceWithFileSystem(devPath string, mountPoint string) (*model.Mount, error) {
	util.LogDebug.Printf("MountDeviceWithFileSystem called with %s %s", devPath, mountPoint)
	if devPath == "" || mountPoint == "" {
		return nil, errors.New("Neither arg can be nul devPath :" + devPath + " mountPoint :" + mountPoint)
	}

	// check if the mountpoint exists
	err := checkIfMountExists(devPath, mountPoint)
	if err != nil {
		return nil, err
	}

	// check if mountpoint already has a device
	mountedDevice, err := GetDeviceFromMountPoint(mountPoint)
	if mountedDevice != "" || err != nil {
		return nil, errors.New(devPath + " is already mounted at " + mountPoint)
	}

	// if not already mounted try to mount the device
	args := []string{devPath, mountPoint}
	_, rc, err := util.ExecCommandOutput(mountCommand, args)
	if err != nil {
		if rc == mountUUIDErr {
			util.LogDebug.Print("rc=" + strconv.Itoa(mountUUIDErr) + " trying again with no uuid option")
			_, _, err = util.ExecCommandOutput(mountCommand, []string{"-o", "nouuid", devPath, mountPoint})
		}
	}
	if err != nil {
		return nil, err
	}

	// verify the mount worked
	err = verifyMount(devPath, mountPoint)
	device := &model.Device{
		AltFullPathName: mountedDevice,
	}
	mount := &model.Mount{
		Mountpoint: mountPoint,
		Device:     device}
	return mount, err
}

func checkIfMountExists(devPath, mountPoint string) error {
	is, _, err := util.FileExists(devPath)
	if err != nil || is == false {
		return fmt.Errorf("Device Path %s doesn't exist", devPath)
	}
	is, _, err = util.FileExists(mountPoint)
	if err != nil || is == false {
		return fmt.Errorf("Mountpoint %s doesn't exist", mountPoint)
	}
	return err
}

func verifyMount(devPath, mountPoint string) error {
	mountedDevice, err := GetDeviceFromMountPoint(mountPoint)
	if err != nil {
		return err
	}
	if mountedDevice != "" {
		util.LogDebug.Printf("%s is mounted at %s", devPath, mountPoint)
	}
	return err
}
