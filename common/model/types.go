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

package model

//Device describes a storage device
type Device struct {
	Pathname        string   `json:"path_name,omitempty"`
	SerialNumber    string   `json:"serial_number,omitempty"`
	Major           string   `json:"major,omitempty"`
	Minor           string   `json:"minor,omitempty"`
	AltFullPathName string   `json:"alt_full_path_name,omitempty"`
	MpathName       string   `json:"mpath_device_name,omitempty"`
	Size            int64    `json:"size,omitempty"` // size in MiB
	Slaves          []string `json:"slaves,omitempty"`
	Hcils           []string `json:"-"` // export it if needed
}

//Mount describes a filesystem mount
type Mount struct {
	ID         uint64  `json:"id,omitempty"`
	Mountpoint string  `json:"mount_point,omitempty"`
	Device     *Device `json:"device,omitempty"`
}
