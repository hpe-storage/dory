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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"nimblestorage/pkg/k8s/provisioner"
	"nimblestorage/pkg/util"
	"os"
)

func main() {
	if len(os.Args) < 1 {
		fmt.Println("Please specify the full path (including filename) to admin config file.")
		return
	}

	kubeConfig := os.Args[1]
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		fmt.Printf("Error getting config from %s - %s\n", kubeConfig, err.Error())
		os.Exit(1)
	}

	provisionerName := "dory"
	if len(os.Args) > 2 {
		provisionerName = os.Args[2]
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error getting client - %s\n", err.Error())
		os.Exit(1)
	}
	util.OpenLog(true)

	stop := make(chan struct{})
	p := provisioner.NewProvisioner(kubeClient, provisionerName, true)
	p.Start(stop)
	<-stop

}
