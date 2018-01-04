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
	"nimblestorage/pkg/docker/dockervol"
	"nimblestorage/pkg/jconfig"
	flexvol "nimblestorage/pkg/k8s/flexvol"
	"nimblestorage/pkg/util"
	"os"
	"path/filepath"
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
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Not enough args")
		return
	}

	overridden := initialize()
	util.OpenLogFile(logFilePath, 10, 4, 90, debug)
	defer util.CloseLogFile()
	pid := os.Getpid()
	util.LogInfo.Printf("[%d] entry  : Driver=%s Version=%s-%s Socket=%s Overridden=%t", pid, filepath.Base(os.Args[0]), Version, Commit, dockerVolumePluginSocketPath, overridden)

	driverCommand := os.Args[1]
	util.LogInfo.Printf("[%d] request: %s %v", pid, driverCommand, os.Args[2:])
	dockervolOptions := &dockervol.Options{
		SocketPath:                   dockerVolumePluginSocketPath,
		StripK8sFromOptions:          stripK8sFromOptions,
		CreateVolumes:                createVolumes,
		ListOfStorageResourceOptions: listOfStorageResourceOptions,
		FactorForConversion:          factorForConversion,
	}
	err := flexvol.Config(dockervolOptions)
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

func initialize() bool {
	override := false

	// don't log anything in initialize because we haven't open a log file yet.
	c, err := jconfig.NewConfig(fmt.Sprintf("%s%s", os.Args[0], ".json"))
	if err != nil {
		return false
	}
	s, err := c.GetStringWithError("logFilePath")
	if err == nil && s != "" {
		override = true
		logFilePath = s
	}
	s, err = c.GetStringWithError("dockerVolumePluginSocketPath")
	if err == nil && s != "" {
		override = true
		dockerVolumePluginSocketPath = s
	}
	b, err := c.GetBool("logDebug")
	if err == nil {
		override = true
		debug = b
	}
	b, err = c.GetBool("stripK8sFromOptions")
	if err == nil {
		override = true
		stripK8sFromOptions = b
	}
	b, err = c.GetBool("createVolumes")
	if err == nil {
		override = true
		createVolumes = b
	}

	overrideFlexVol := initializeFlexVolOptions(c)
	if overrideFlexVol {
		override = true
	}

	return override
}

func initializeFlexVolOptions(c *jconfig.Config) bool {
	override := false
	ss := c.GetStringSlice("listOfStorageResourceOptions")
	if ss != nil {
		override = true
		listOfStorageResourceOptions = ss
	}
	i := c.GetInt64("factorForConversion")
	if i != 0 {
		override = true
		factorForConversion = int(i)
	}

	e16, err := c.GetBool("enable1.6")
	if err == nil {
		override = true
		enable16 = e16
	}

	return override
}
