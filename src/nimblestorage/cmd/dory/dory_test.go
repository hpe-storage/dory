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
	"testing"
)

var basicTests = []struct {
	name                         string
	override                     bool
	dockerVolumePluginSocketPath string
	stripK8sFromOptions          bool
	logFilePath                  string
	debug                        bool
	createVolumes                bool
	enable16                     bool
	factorForConversion          int
	listOfStorageResourceOptions []string
	supportsCapabilities         bool
}{
	{"test/good", true, "/run/docker/plugins/nimble.sock", true, "/var/log/dory.log", false, true, false, 1073741824, []string{"size", "sizeInGiB"}, true},
	{"test/flipped", true, "nimble", false, "some path", true, false, true, 14, []string{"size", "sizeInGiB", "w", "x", "y", "z"}, false},
	{"test/broken", false, "/run/docker/plugins/nimble.sock", true, "/var/log/dory.log", false, true, false, 1073741824, []string{"size", "sizeInGiB"}, true},
	{"test/errors", true, "21", true, "true", false, true, false, 1073741824, []string{"size", "sizeInGiB"}, true},
}

// nolint: gocyclo
func TestConfigFiles(t *testing.T) {
	for _, tc := range basicTests {
		t.Run(tc.name, func(t *testing.T) {
			//reset for each test
			dockerVolumePluginSocketPath = "/run/docker/plugins/nimble.sock"
			stripK8sFromOptions = true
			logFilePath = "/var/log/dory.log"
			debug = false
			createVolumes = true
			enable16 = false
			factorForConversion = 1073741824
			listOfStorageResourceOptions = []string{"size", "sizeInGiB"}
			supportsCapabilities = true

			override := initialize(tc.name, true)
			if override != tc.override {
				t.Error(
					"For", "override",
					"expected", tc.override,
					"got:", override,
				)
			}
			if dockerVolumePluginSocketPath != tc.dockerVolumePluginSocketPath {
				t.Error(
					"For", "dockerVolumePluginSocketPath",
					"expected", tc.dockerVolumePluginSocketPath,
					"got:", dockerVolumePluginSocketPath,
				)
			}
			if stripK8sFromOptions != tc.stripK8sFromOptions {
				t.Error(
					"For", "stripK8sFromOptions",
					"expected", tc.stripK8sFromOptions,
					"got:", stripK8sFromOptions,
				)
			}
			if logFilePath != tc.logFilePath {
				t.Error(
					"For", "logFilePath",
					"expected", tc.logFilePath,
					"got:", logFilePath,
				)
			}
			if debug != tc.debug {
				t.Error(
					"For", "debug",
					"expected", tc.debug,
					"got:", debug,
				)
			}
			if createVolumes != tc.createVolumes {
				t.Error(
					"For", "createVolumes",
					"expected", tc.createVolumes,
					"got:", createVolumes,
				)
			}
			if enable16 != tc.enable16 {
				t.Error(
					"For", "enable16",
					"expected", tc.enable16,
					"got:", enable16,
				)
			}
			if factorForConversion != tc.factorForConversion {
				t.Error(
					"For", "factorForConversion",
					"expected", tc.factorForConversion,
					"got:", factorForConversion,
				)
			}
			if len(listOfStorageResourceOptions) != len(tc.listOfStorageResourceOptions) {
				t.Error(
					"For", "listOfStorageResourceOptions",
					"expected", tc.listOfStorageResourceOptions,
					"got:", listOfStorageResourceOptions,
				)
			}
			if supportsCapabilities != tc.supportsCapabilities {
				t.Error(
					"For", "supportsCapabilities",
					"expected", tc.supportsCapabilities,
					"got:", supportsCapabilities,
				)
			}
		})
	}
}
