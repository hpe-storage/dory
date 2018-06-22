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

package main

import (
	"fmt"
	"github.com/hpe-storage/dory/common/docker/dockervol"
	"github.com/hpe-storage/dory/common/jconfig"
	flexvol "github.com/hpe-storage/dory/common/k8s/flexvol"
	"github.com/hpe-storage/dory/common/util"
	"os"
	"path/filepath"
)

const (
	cmdConfigChk = "config"
	//override options
	optDockerVolumePluginSocketPath = "dockerVolumePluginSocketPath"
	optStripK8sFromOptions          = "stripK8sFromOptions"
	optLogFilePath                  = "logFilePath"
	optDebug                        = "logDebug"
	optCreateVolumes                = "createVolumes"
	optEnable16                     = "enable1.6"
	optFactorForConversion          = "factorForConversion"
	optListOfStorageResourceOptions = "listOfStorageResourceOptions"
	optSupportsCapabilities         = "supportsCapabilities"
)

var (
	// Version contains the current version added by the build process
	Version = "dev"
	// Commit contains the commit id added by the build process
	Commit = "unknown"

	dockerVolumePluginSocketPath = "/run/docker/plugins/nimble.sock"
	stripK8sFromOptions          = true
	logFilePath                  = "/var/log/dory.log"
	debug                        = false
	createVolumes                = true
	enable16                     = false
	factorForConversion          = 1073741824
	listOfStorageResourceOptions = []string{"size", "sizeInGiB"}
	supportsCapabilities         = true
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Not enough args")
		return
	}

	driverCommand := os.Args[1]
	justCheckConfig := false
	if driverCommand == cmdConfigChk {
		justCheckConfig = true
	}

	overridden := initialize(os.Args[0], justCheckConfig)
	if justCheckConfig {
		return
	}

	util.OpenLogFile(logFilePath, 10, 4, 90, debug)
	defer util.CloseLogFile()
	pid := os.Getpid()
	util.LogInfo.Printf("[%d] entry  : Driver=%s Version=%s-%s Socket=%s Overridden=%t", pid, filepath.Base(os.Args[0]), Version, Commit, dockerVolumePluginSocketPath, overridden)

	util.LogInfo.Printf("[%d] request: %s %v", pid, driverCommand, os.Args[2:])
	dockervolOptions := &dockervol.Options{
		SocketPath:                   dockerVolumePluginSocketPath,
		StripK8sFromOptions:          stripK8sFromOptions,
		CreateVolumes:                createVolumes,
		ListOfStorageResourceOptions: listOfStorageResourceOptions,
		FactorForConversion:          factorForConversion,
		SupportsCapabilities:         supportsCapabilities,
	}
	err := flexvol.Config(os.Args[0], dockervolOptions)
	var mess string
	if err != nil {
		mess = flexvol.BuildJSONResponse(&flexvol.Response{
			Status:  flexvol.FailureStatus,
			Message: fmt.Sprintf("Unable to communicate with docker volume plugin - %s", err.Error())})
	} else {
		mess = flexvol.Handle(driverCommand, enable16, os.Args[2:])
	}
	util.LogInfo.Printf("[%d] reply  : %s %v: %v", pid, driverCommand, os.Args[2:], mess)

	fmt.Println(mess)
}

func initialize(name string, report bool) bool {
	override := false

	// don't log anything in initialize because we haven't open a log file yet.
	filePath := fmt.Sprintf("%s%s", name, ".json")
	c, err := jconfig.NewConfig(filePath)
	if err != nil {
		if report {
			fmt.Printf("Error processing %s - %s\n", filePath, err.Error())
		}
		return false
	}

	s, err := c.GetStringWithError(optLogFilePath)
	if err == nil && s != "" {
		override = true
		logFilePath = s
	} else {
		configOptCheck(report, optLogFilePath, err)
	}

	s, err = c.GetStringWithError(optDockerVolumePluginSocketPath)
	if err == nil && s != "" {
		override = true
		dockerVolumePluginSocketPath = s
	} else {
		configOptCheck(report, optDockerVolumePluginSocketPath, err)
	}

	b, err := c.GetBool(optDebug)
	if err == nil {
		override = true
		debug = b
	} else {
		configOptCheck(report, optDebug, err)
	}

	b, err = c.GetBool(optSupportsCapabilities)
	if err == nil {
		override = true
		supportsCapabilities = b
	} else {
		configOptCheck(report, optSupportsCapabilities, err)
	}

	overrideFlexVol := initializeFlexVolOptions(c, report)
	if overrideFlexVol {
		override = true
	}

	return override
}

func initializeFlexVolOptions(c *jconfig.Config, report bool) bool {
	override := false

	b, err := c.GetBool(optStripK8sFromOptions)
	if err == nil {
		override = true
		stripK8sFromOptions = b
	} else {
		configOptCheck(report, optStripK8sFromOptions, err)
	}

	b, err = c.GetBool(optCreateVolumes)
	if err == nil {
		override = true
		createVolumes = b
	} else {
		configOptCheck(report, optCreateVolumes, err)
	}

	ss, err := c.GetStringSliceWithError(optListOfStorageResourceOptions)
	if ss != nil {
		override = true
		listOfStorageResourceOptions = ss
	} else {
		configOptCheck(report, optListOfStorageResourceOptions, err)
	}

	i, err := c.GetInt64SliceWithError(optFactorForConversion)
	if err == nil {
		override = true
		factorForConversion = int(i)
	} else {
		configOptCheck(report, optFactorForConversion, err)
	}

	e16, err := c.GetBool(optEnable16)
	if err == nil {
		override = true
		enable16 = e16
	} else {
		configOptCheck(report, optEnable16, err)
	}
	configOptDump(report)

	return override
}

func configOptCheck(report bool, optName string, err error) {
	if report {
		fmt.Printf("Error processing option '%s' - %s\n", optName, err.Error())
	}
}

func configOptDump(report bool) {
	if !report {
		return
	}
	fmt.Printf("\nDriver=%s Version=%s-%s\nCurrent Config:\n", filepath.Base(os.Args[0]), Version, Commit)
	fmt.Printf("%30s = %s\n", optDockerVolumePluginSocketPath, dockerVolumePluginSocketPath)
	fmt.Printf("%30s = %t\n", optStripK8sFromOptions, stripK8sFromOptions)
	fmt.Printf("%30s = %s\n", optLogFilePath, logFilePath)
	fmt.Printf("%30s = %t\n", optDebug, debug)
	fmt.Printf("%30s = %t\n", optCreateVolumes, createVolumes)
	fmt.Printf("%30s = %t\n", optEnable16, enable16)
	fmt.Printf("%30s = %d\n", optFactorForConversion, factorForConversion)
	fmt.Printf("%30s = %v\n", optListOfStorageResourceOptions, listOfStorageResourceOptions)
	fmt.Printf("%30s = %t\n", optSupportsCapabilities, supportsCapabilities)

}
