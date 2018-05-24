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
	"github.com/hpe-storage/dory/common/util"
	"strings"
)

const (
	procMounts    = "/proc/mounts"
	mountCommand  = "mount"
	umountCommand = "umount"
)

func unmount(mountPoint string) error {
	// try to unmount
	args := []string{mountPoint}
	_, _, err := util.ExecCommandOutput(umountCommand, args)
	if err != nil {
		return err
	}
	return nil
}

//BindMount mounts a path to a mountPoint.  The rbind flag controls recursive binding
func BindMount(path, mountPoint string, rbind bool) error {
	util.LogDebug.Printf("BindMount called with %s %s %v", path, mountPoint, rbind)
	flag := "--bind"
	if rbind {
		flag = "--rbind"
	}

	args := []string{flag, path, mountPoint}
	out, rc, err := util.ExecCommandOutput(mountCommand, args)
	if err != nil {
		util.LogError.Printf("BindMount failed with %d.  It was called with %s %s %v.  Output=%v.", rc, path, mountPoint, rbind, out)
		return err
	}

	return nil
}

//BindUnmount unmounts a bind mount.
func BindUnmount(mountPoint string) error {
	util.LogDebug.Printf("BindUnmount called with %s", mountPoint)
	err := unmount(mountPoint)
	if err != nil {
		return err
	}

	return nil
}

//GetDeviceFromMountPoint returns the device path from /proc/mounts
// for the mountpoint provided.  For example /dev/mapper/mpathd might be
// returned for /mnt.
func GetDeviceFromMountPoint(mountPoint string) (string, error) {
	util.LogDebug.Print("getDeviceFromMountPoint called with ", mountPoint)
	return getMountsEntry(mountPoint, false)
}

//GetMountPointFromDevice returns the FIRST mountpoint listed in
// /proc/mounts matching the device.  Note that /proc/mounts lists
// device paths using the device mapper format.  For example: /dev/mapper/mpathd
func GetMountPointFromDevice(devPath string) (string, error) {
	util.LogDebug.Print("getMountPointFromDevice called with ", devPath)
	return getMountsEntry(devPath, true)
}

func getMountsEntry(path string, dev bool) (string, error) {
	util.LogDebug.Printf("getMountsEntry called with path:%v isDev:%v", path, dev)
	mountLines, err := util.FileGetStrings(procMounts)
	if err != nil {
		return "", err
	}

	var searchIndex, returnIndex int
	if dev {
		returnIndex = 1
	} else {
		searchIndex = 1
	}

	for _, line := range mountLines {
		entry := strings.Fields(line)
		util.LogDebug.Print("mounts entry :", entry)
		if len(entry) > 2 {
			if entry[searchIndex] == path {
				util.LogDebug.Printf("%s was found with %s", path, entry[returnIndex])
				return entry[returnIndex], nil
			}
		}
	}
	return "", nil
}
