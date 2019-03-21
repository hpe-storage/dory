/*
(c) Copyright 2018 Hewlett Packard Enterprise Development LP

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

package dockerlt

import (
	"github.com/hpe-storage/dory/common/connectivity"
	"github.com/hpe-storage/dory/common/util"
	"time"
)

const (
	defaultSocketPath         = "/var/run/docker.sock"
	dockerClientSocketTimeout = time.Duration(300) * time.Second
)

// DockerClient is a light weight docker client
type DockerClient struct {
	client *connectivity.Client
}

type errorResponse struct {
	Message string `json:"message,omitempty"`
}

// NewDockerClient provides a light weight docker client connection
func NewDockerClient(socketPath string) *DockerClient {
	if socketPath == "" {
		socketPath = defaultSocketPath
	}
	return &DockerClient{connectivity.NewSocketClientWithTimeout(socketPath, dockerClientSocketTimeout)}
}

// PluginsGet does a GET against /plugins
func (dc *DockerClient) PluginsGet() ([]Plugin, error) {
	plugins := make([]Plugin, 0)
	apiError := &errorResponse{}

	err := dc.client.DoJSON(&connectivity.Request{
		Action:        "GET",
		Path:          "/plugins",
		Payload:       nil,
		Response:      &plugins,
		ResponseError: apiError})

	if err != nil {
		util.LogInfo.Printf("unable to list docker plugins - %s (%s)", err.Error(), apiError.Message)
		return nil, err
	}

	util.LogDebug.Printf("returning %#v", plugins)
	return plugins, nil
}
