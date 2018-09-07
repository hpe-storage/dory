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

package flexvol

import (
	"encoding/json"
	"fmt"
	"github.com/hpe-storage/dory/common/docker/dockervol"
	"github.com/hpe-storage/dory/common/linux"
	"github.com/hpe-storage/dory/common/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"
	"strings"
)

const (
	// InitCommand  - Initializes the driver.
	InitCommand = "init"
	// AttachCommand - Attach the volume specified by the given spec.
	AttachCommand = "attach"
	//DetachCommand - Detach the volume from the kubelet.
	DetachCommand = "detach"
	//MountCommand - Mount device mounts the device to a global path which individual pods can then bind mount.
	MountCommand = "mount"
	//UnmountCommand - Unmounts the filesystem for the device.
	UnmountCommand = "unmount"
	//GetVolumeNameCommand - Get the name of the volume.
	GetVolumeNameCommand = "getvolumename"
	//SuccessStatus indicates success
	SuccessStatus = "Success"
	//FailureStatus indicates failure
	FailureStatus = "Failure"
	//NotSupportedStatus indicates not supported
	NotSupportedStatus = "Not supported"
	//FailureJSON is a pre-marshalled response used in the case of a marshalling error
	FailureJSON = "{\"status\":\"Failure\",\"message\":\"Unknown error.\"}"
	//mountPathRegex describes the uuid and flexvolume name in the path
	//examples:
	// /var/lib/origin/openshift.local.volumes/pods/88917cdb-514d-11e7-93fb-5254005e615a/volumes/hpe~nimble/test2
	// /var/lib/kubelet/pods/fb36bec9-51f7-11e7-8eb8-005056968cbc/volumes/hpe~nimble/test
	mountPathRegex = "/var/lib/.*/pods/(?P<uuid>[\\w\\d-]*)/volumes/"
	//docker volume status key
	devicePathKey = "devicePath"
	maxTries      = 3
)

var (
	//createVolumes indicate whether the driver should create missing volumes
	createVolumes = true
	pluginID = ""

	execPath string

	dvp *dockervol.DockerVolumePlugin
)

// Response containers the required information for each invocation
type Response struct {
	//"status": "<Success/Failure/Not Supported>",
	Status string `json:"status"`
	//"message": "<Reason for success/failure>",
	Message string `json:"message,omitempty"`
	//"device": "<Path to the device attached. This field is valid only for attach calls>"
	Device string `json:"device,omitempty"`
	//"volumeName:" "undocumented"
	VolumeName string `json:"volumeName,omitempty"`
	//"attached": <True/False (Return true if volume is attached on the node. Valid only for isattached call-out)>
	Attached bool `json:"attached,omitempty"`
	//Capabilities reported on Driver init
	DriverCapabilities map[string]bool `json:"capabilities,omitempty"`
}

//AttachRequest is used to create a volume if one with this name doesn't exist
type AttachRequest struct {
	Name           string
	PvOrVolumeName string `json:"kubernetes.io/pvOrVolumeName,omitempty"`
	FsType         string `json:"kubernetes.io/fsType,omitempty"`
	ReadWrite      string `json:"kubernetes.io/readwrite,omitempty"`
}

func (ar *AttachRequest) getBestName() string {
	if ar.Name != "" {
		return ar.Name
	}
	return ar.PvOrVolumeName
}

// Config controls the docker behavior
func Config(ePath string, options *dockervol.Options) (err error) {
	dvp, err = dockervol.NewDockerVolumePlugin(options)
	createVolumes = options.CreateVolumes
	pluginID = options.ManagedPluginID
	execPath = ePath
	return err
}

// BuildJSONResponse marshals a message into the FlexVolume JSON Response.
// If error is not nil, the default Failure message is returned.
func BuildJSONResponse(response *Response) string {
	if len(response.Status) < 1 {
		response.Status = NotSupportedStatus
	}

	jmess, err := json.Marshal(response)
	if err != nil {
		return FailureJSON
	}
	return string(jmess)
}

// ErrorResponse creates a Response with Status and Message set.
func ErrorResponse(err error) *Response {
	response := &Response{
		Status: FailureStatus,
	}
	response.Message = err.Error()
	return response
}

//Get a volume (create if necessary) This was added to k8s 1.6
func Get(jsonRequest string) (string, error) {
	util.LogInfo.Printf("get called with (%s)\n", jsonRequest)
	req := &AttachRequest{}
	err := json.Unmarshal([]byte(jsonRequest), req)
	if err != nil {
		return "", err
	}
	name, err := getOrCreate(req.getBestName(), jsonRequest)
	if err != nil {
		return "", err
	}
	response := &Response{
		Status:     SuccessStatus,
		VolumeName: name,
	}
	return BuildJSONResponse(response), nil
}

//Attach doesn't attach a volume.  It simply creates a volume if necessary.  It then returns "Not Supported".
//This worked well in 1.5 in that it broke the create and mount into 2 timeout windows, but
//this has changed in 1.6.
func Attach(jsonRequest string) (string, error) {
	util.LogDebug.Printf("attach called with %s\n", jsonRequest)
	req := &AttachRequest{}
	err := json.Unmarshal([]byte(jsonRequest), req)
	if err != nil {
		return "", err
	}

	_, err = getOrCreate(req.getBestName(), jsonRequest)
	if err != nil {
		return "", err
	}

	return BuildJSONResponse(&Response{Status: NotSupportedStatus, Message: "Not supported."}), nil
}

func getOrCreate(name, jsonRequest string) (string, error) {
	util.LogDebug.Printf("getOrCreate called with %s and %s\n", name, jsonRequest)
	volume, err := dvp.Get(name)
	if err != nil || volume.Volume.Name != name {
		if !createVolumes {
			return "", fmt.Errorf("configured to NOT create volumes")
		}

		util.LogInfo.Printf("volume %s was not found(err=%v), creating a new volume using %v", name, err, jsonRequest)
		var options map[string]interface{}
		err := json.Unmarshal([]byte(jsonRequest), &options)
		if err != nil {
			util.LogError.Printf("unable to unmarshal options for %v - %s", jsonRequest, err.Error())
			return "", err
		}
		newName, err := dvp.Create(name, options)
		util.LogDebug.Printf("getOrCreate returning %v for %s", newName, name)
		if err != nil {
			return "", err
		}
		return newName, nil
	}

	return volume.Volume.Name, nil
}

//Mount a volume
func Mount(args []string) (string, error) {
	util.LogDebug.Printf("mount called with %v\n", args)
	err := ensureArg("mount", args, 2)
	if err != nil {
		return "", err
	}

	req := &AttachRequest{}
	//json seems to be in the second or third argument
	jsonRequest, err := findJSON(args, req)
	if err != nil {
		return "", err
	}

	dockerVolName := req.getBestName()
	_, err = getOrCreate(dockerVolName, jsonRequest)
	if err != nil {
		return "", err
	}

	mountID, err := getMountID(args[0])
	if err != nil {
		return "", err
	}

	path, err := dvp.Mount(dockerVolName, mountID)
	if err != nil {
		return "", err
	}

	//Mkdir
	err = os.MkdirAll(args[0], 0755)
	if err != nil {
		return "", err
	}

	err = doMount(args[0], path, dockerVolName, mountID)
	if err != nil {
		pathForManagedPlugin := "/var/lib/docker/plugins/" + pluginID + "/rootfs"+ path
		util.LogDebug.Printf("pathForManagedPlugin: %s", pathForManagedPlugin)
		err = linux.BindMount(pathForManagedPlugin, args[0], false)

		if err != nil {
			return "", err
		}
		path = pathForManagedPlugin
	}

	// Set selinux context if configured
	// References:
	//    https://github.com/kubernetes/kubernetes/issues/20813
	//    https://github.com/openshift/origin/issues/741
	//    https://github.com/projectatomic/atomic-site/blob/master/source/blog/2015-06-15-using-volumes-with-docker-can-cause-problems-with-selinux.html.md
	err = linux.Chcon("svirt_sandbox_file_t", path)
	if err != nil {
		return "", err
	}

	return BuildJSONResponse(&Response{Status: SuccessStatus}), nil
}

// Unmount a volume
func Unmount(args []string) (string, error) {
	util.LogDebug.Printf("Unmount called with %v", args)
	mountID, err := getMountID(args[0])
	if err != nil {
		return "", err
	}

	devPath, err := linux.GetDeviceFromMountPoint(args[0])
	if err != nil {
		return "", err
	}

	util.LogDebug.Printf("Umount of \"%s\" from %s", args[0], devPath)
	err = linux.BindUnmount(args[0])
	if err != nil {
		return "", err
	}

	dockerPath, metadata, err := getDockerPathAndMetadata(args[0], devPath)
	if err != nil {
		return "", err
	}

	dockerVolumeName, err := retryGetVolumeNameFromMountPath(args[0], dockerPath)
	if err != nil {
		return "", err
	}

	util.LogDebug.Printf("docker unmount of %s %s", dockerVolumeName, mountID)
	err = dvp.Unmount(dockerVolumeName, mountID)
	if err != nil {
		return "", err
	}

	if metadata != "" {
		dockerVolumeName, err = getVolumeNameFromMountPath(args[0], dockerPath)
		if err != nil {
			// an error means that we didn't find the volume mounted
			// this means we can clean up the breadcrumbs
			util.LogDebug.Printf("Unmount: removing metadata=%s", metadata)
			os.Remove(metadata)
		}
		util.LogDebug.Printf("Unmount: dockerVolumeName=%s still has an active mount at %s.", dockerVolumeName, dockerPath)
	}

	return BuildJSONResponse(&Response{Status: SuccessStatus}), nil
}

// retry getVolumeNameFromMountPath for maxTries
func retryGetVolumeNameFromMountPath(k8sPath, dockerPath string) (string, error) {
	util.LogDebug.Printf("retryGetVolumeNameFromMountPath called with %s %s", k8sPath, dockerPath)
	try := 0
	for {
		util.LogDebug.Printf("getVolumeNameFromMountPath called with %s %s try:%d", k8sPath, dockerPath, try+1)
		dockerVolumeName, err := getVolumeNameFromMountPath(k8sPath, dockerPath)
		if err != nil {
			if try < maxTries {
				try++
				time.Sleep(time.Duration(try) * time.Second)
				continue
			}
			return "", err
		}
		util.LogDebug.Printf("dockerVolumeName %s found at k8sPath :%s", dockerVolumeName, k8sPath)
		return dockerVolumeName, nil
	}
}

func getMountID(path string) (string, error) {
	util.LogDebug.Printf("getMountID called with %v\n", path)
	r := regexp.MustCompile(mountPathRegex)
	groups := r.FindStringSubmatch(path)
	if len(groups) < 2 {
		return "", fmt.Errorf("unable to split %s", path)
	}
	util.LogDebug.Printf("getMountID returning \"%s\"", groups[1])
	return groups[1], nil

}

func getVolumeNameFromMountPath(k8sPath, dockerPath string) (string, error) {
	util.LogDebug.Printf("getVolumeNameFromMountPath called with %s and %s", k8sPath, dockerPath)
	name := filepath.Base(dockerPath)
	dockerVolume, err := dvp.Get(name)
	if err != nil || dockerVolume.Volume.Name != name {
		// The docker plugin might not use the docker volume name in the path.
		// Therefore we need to look through the know volumes to find out who
		// is mounted at that path.
		volumes, err2 := dvp.List()
		if err2 != nil {
			util.LogError.Printf("Unable to get list of volumes. - %s, get error was %s", err2, err)
			return "", err
		}
		for _, vol := range volumes.Volumes {
			util.LogDebug.Printf("dockerPath %s, volume mountpoint %s", dockerPath, vol.Mountpoint)
			if (vol.Mountpoint == dockerPath || (findStringAfterLastSlash(vol.Mountpoint) == findStringAfterLastSlash(dockerPath))){
				util.LogDebug.Printf(" returning docker volume name %s", vol.Name)
				return vol.Name, nil
			}
		}
		return "", fmt.Errorf("unable to find docker volume for %s.  No docker volume claimed to be mounted at %s", k8sPath, dockerPath)
	}
	if dockerVolume.Volume.Mountpoint == "" {
		return "", fmt.Errorf("found a docker volume but its MountPoint was \"\"")
	}
	return dockerVolume.Volume.Name, nil
}

func findStringAfterLastSlash(s string) string {

	//s := "/var/lib/docker/plugins/a238188db964f8139af8d502a9b134b1f9522ccc27936ae5512b2f1b662f0aa5/rootfs/opt/hpe/data/hpedocker-dm-uuid-mpath-360002ac0000000000101331f00019d52"

	flds := strings.Split(s, "/")
	arrayLength := len(flds)
	fmt.Printf(" Length = %d, last substring %s" ,arrayLength, flds[arrayLength -1])
	return flds[arrayLength - 1]
}
func findJSON(args []string, req *AttachRequest) (string, error) {
	var err error
	for i := 1; i < len(args); i++ {
		util.LogDebug.Printf("findJSON(%d) about to unmarshal %v", i, args[i])
		err = json.Unmarshal([]byte(args[i]), req)
		if err == nil {
			return args[i], nil
		}
	}
	return "", err
}

// return the dockerPath and a path to the metadata file if present
func getDockerPathAndMetadata(flexvolPath, devPath string) (string, string, error) {
	dockerPath, err := linux.GetMountPointFromDevice(devPath)
	if err != nil {
		return "", "", err
	}
	util.LogDebug.Printf("getDockerPathAndMetadata: devPath=%s was mounted on dockerPath=%s", devPath, dockerPath)

	metadata := ""
	if dockerPath == "" {
		// if we didn't get a docker path its because we're running
		// in a different namespace (likely rkt)
		util.LogInfo.Printf("getDockerPathAndMetadata: didn't find a docker path for devPath=%s and flexvolPath=%s", devPath, flexvolPath)

		metadata, err = getMountMetadataPath(flexvolPath)
		if err != nil {
			util.LogError.Printf("getDockerPathAndMetadata: unable to read metadata=%s for devPath=%s and flexvolPath=%s", metadata, devPath, flexvolPath)
			return "", "", err
		}

		var fileData []byte
		fileData, err = ioutil.ReadFile(metadata)
		if err != nil {
			util.LogError.Printf("getDockerPathAndMetadata: unable to read file content from metadata=%s for devPath=%s and flexvolPath=%s", metadata, dockerPath, flexvolPath)
			return "", "", err
		}
		dockerPath = string(fileData)
		util.LogDebug.Printf("getDockerPathAndMetadata: found dockerPath=%s for devPath=%s and flexvolPath=%s", dockerPath, devPath, flexvolPath)
	}

	return dockerPath, metadata, nil
}

func doMount(flexvolPath, dockerPath, dockerName, mountID string) error {
	devPath, err := linux.GetDeviceFromMountPoint(dockerPath)
	if err != nil {
		return err
	}

	if devPath == "" {
		//we're probably running in a different namespace
		//so we need to pull the device path from the
		//docker volume driver
		util.LogInfo.Printf("doMount: devPath was empty for flexvolPath=% volume=%s", flexvolPath, dockerName)

		//get the volume info
		var volRes *dockervol.GetResponse
		volRes, err = dvp.Get(dockerName)
		if err != nil {
			return err
		}

		devPath, found := volRes.Volume.Status[devicePathKey].(string)
		if !found || devPath == "" {
			util.LogError.Printf("Unable to get device for flexvolPath=%s from docker volume=%+v (path=%s)", flexvolPath, volRes, dockerPath)
			return fmt.Errorf("Unable to get device for flexvolPath=%s from docker volume=%s", flexvolPath, dockerPath)
		}
		util.LogDebug.Printf("doMount: found devPath=%s for volume=%s", devPath, dockerName)

		//mount devicePath onto flexvolPath
		_, err = linux.MountDeviceWithFileSystem(devPath, flexvolPath)
		if err != nil {
			return err
		}
		util.LogDebug.Printf("doMount: mounted devPath=%s at flexvolPath=%s", devPath, flexvolPath)

		//create a hidden file in the flexvolume path that maps flexvolume mount to the docker volume (breadcrumb)
		var metadata string
		metadata, err = getMountMetadataPath(flexvolPath)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(metadata, []byte(dockerPath), 0600)
		if err != nil {
			return err
		}
		util.LogDebug.Printf("doMount: stored dockerPath=%s at metadata=%s", dockerPath, metadata)

	} else {
		//bind mount the docker path to the flexvol path
		err = linux.BindMount(dockerPath, flexvolPath, false)
		util.LogDebug.Printf("doMount: bind mounted dockerPath=%s at flexvolPath=%s", dockerPath, flexvolPath)
	}

	return err
}

func getMountMetadataPath(flexvolPath string) (string, error) {
	_, flexvolFilename := filepath.Split(flexvolPath)
	if flexvolFilename == "" {
		return "", fmt.Errorf("unable to get filename from %s", flexvolPath)
	}
	execPathDir, _ := filepath.Split(execPath)
	if flexvolFilename == "" {
		return "", fmt.Errorf("unable to get dir from %s", execPath)
	}
	return fmt.Sprintf("%s.%s", execPathDir, flexvolFilename), nil
}
