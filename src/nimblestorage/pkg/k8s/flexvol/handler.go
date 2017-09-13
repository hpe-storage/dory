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
	"fmt"
	"nimblestorage/pkg/util"
)

// Handle the conversion of flexvol commands and args to docker volume
func Handle(driverCommand string, args []string) string {
	if driverCommand == InitCommand {
		return BuildJSONResponse(&Response{Status: SuccessStatus})
	}

	err := ensureArg(driverCommand, args, 1)
	if err != nil {
		return BuildJSONResponse(ErrorResponse(err))
	}

	switch driverCommand {
	case AttachCommand:
		return attachVolume(args[0])
	case MountCommand:
		return mountVolume(args)
	case UnmountCommand:
		return unmountVolume(args)
	default:
		util.LogError.Printf("Unsupported command (%s) was called with %s\n", driverCommand, args)
	}
	return BuildJSONResponse(&Response{Status: NotSupportedStatus, Message: "Not supported."})
}

func attachVolume(json string) string {
	mess, err := Attach(json)
	if err != nil {
		return BuildJSONResponse(ErrorResponse(err))
	}
	return mess
}

func mountVolume(args []string) string {
	mess, err := Mount(args)
	if err != nil {
		return BuildJSONResponse(ErrorResponse(err))
	}
	return mess
}

func unmountVolume(args []string) string {
	mess, err := Unmount(args)
	if err != nil {
		return BuildJSONResponse(ErrorResponse(err))
	}
	return mess
}

func ensureArg(driverCommand string, args []string, number int) error {
	if len(args) < number {
		return fmt.Errorf("Not enough arguments for %s", driverCommand)
	}
	return nil
}
