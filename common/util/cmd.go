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

package util

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

// ExecCommandOutput returns stdout and stderr in a single string, the return code, and error.
// If the return code is not zero, error will not be nil.
// Stdout and Stderr are dumped to the log at the debug level.
// Return code of 999 indicates an error starting the command.
func ExecCommandOutput(cmd string, args []string) (string, int, error) {
	LogDebug.Print("ExecCommandOutput called with ", cmd, args)
	c := exec.Command(cmd, args...)
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b

	if err := c.Start(); err != nil {
		return "", 999, err
	}

	//TODO we could set a timeout here if needed

	err := c.Wait()
	out := string(b.Bytes())

	for _, line := range strings.Split(out, "\n") {
		LogDebug.Print("out :", line)
	}

	if err != nil {
		//check the rc of the exec
		if badnews, ok := err.(*exec.ExitError); ok {
			if status, ok := badnews.Sys().(syscall.WaitStatus); ok {
				return out, status.ExitStatus(), fmt.Errorf("rc=%d", status.ExitStatus())
			}
		} else {
			return out, 888, fmt.Errorf("unknown error")
		}
	}

	return out, 0, nil
}

// FindStringSubmatchMap : find and build  the map of named groups
func FindStringSubmatchMap(s string, r *regexp.Regexp) map[string]string {
	captures := make(map[string]string)
	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}
	for i, name := range r.SubexpNames() {
		if i != 0 {
			captures[name] = match[i]
		}
	}
	return captures
}
