package provisioner

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"nimblestorage/pkg/util"
	"os"
	"testing"
)

func TestForFun(t *testing.T) {
	util.OpenLog(true)
	kubeConfig := "/Users/eforgette/eforgette-u16k816i.conf"
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		util.LogError.Printf("Error getting config from %s - %s\n", kubeConfig, err.Error())
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		util.LogError.Printf("Error getting client - %s\n", err.Error())
		os.Exit(1)
	}

	stop := make(chan struct{})
	p := NewProvisioner(kubeClient, "dory", false)
	p.Start(stop)

	<-stop
}
