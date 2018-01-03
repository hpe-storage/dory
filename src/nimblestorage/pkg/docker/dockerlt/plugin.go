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

// Plugin describes a Docker v2 plugin
type Plugin struct {
	ID      string       `json:"Id,omitempty"`
	Name    string       `json:"Name,omitempty"`
	Enabled bool         `json:"Enabled,omitempty"`
	Config  PluginConfig `json:"Config,omitempty"`
}

// PluginConfig describes the config for the plugin
type PluginConfig struct {
	Interface PluginInterface `json:"Interface,omitempty"`
}

// PluginInterface describes the interface used by docker to communicate with this plugin
type PluginInterface struct {
	Socket string `json:"Socket,omitempty"`
}
