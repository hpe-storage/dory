package provisioner

import (
	"github.com/hpe-storage/dory/common/docker/dockervol"
	resource_v1 "k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	storage_v1 "k8s.io/client-go/pkg/apis/storage/v1"
	rest "k8s.io/client-go/rest"
	"testing"
)

type testDockerOptions struct {
	listOfStorageResourceOptions []string
	factorForConversion          int
	claimSize                    int
}

func getStorageClassParams() map[string]string {
	m := make(map[string]string)
	m["size"] = "123"
	m["description"] = "dynamic"
	return m
}

func getTestDockerOptions() map[string]testDockerOptions {
	m := make(map[string]testDockerOptions)
	m["invalidClaim"] = testDockerOptions{
		[]string{"size", "sizeInGiB"},
		0,
		-1,
	}
	m["validClaim"] = testDockerOptions{
		[]string{"size", "sizeInGiB"},
		1073741824,
		16,
	}

	m["invalidStorageResources"] = testDockerOptions{
		[]string{"invalidSize", "invalidSizeinGiB"},
		12345,
		16,
	}
	return m

}

func TestDockerOptionInvalidClaim(t *testing.T) {
	invalidOption := getTestDockerOptions()["invalidClaim"]
	outputOption := getDockerOptions(getStorageClassParams(), invalidOption.claimSize, invalidOption.listOfStorageResourceOptions)
	if outputOption["size"] != getStorageClassParams()["size"] {
		t.Error("size should not be set for invalid claimsize")
	}
}

func TestDockerOptionValidClaim(t *testing.T) {
	validOption := getTestDockerOptions()["validClaim"]
	outputOption := getDockerOptions(getStorageClassParams(), validOption.claimSize, validOption.listOfStorageResourceOptions)
	if outputOption["size"] == getStorageClassParams()["size"] {
		t.Error("size should be set for for valid claimsize")
	}

}
func TestInvalidStorageResources(t *testing.T) {
	invalidOption := getTestDockerOptions()["invalidStorageResources"]
	outputOption := getDockerOptions(getStorageClassParams(), invalidOption.claimSize, invalidOption.listOfStorageResourceOptions)
	if outputOption["size"] == getStorageClassParams()["size"] {
		t.Error("size should set for for invalid listOfStorageResourceOptions but valid claim size")
	}
}
func getTestPVC() *api_v1.PersistentVolumeClaim {
	testClaim := new(api_v1.PersistentVolumeClaim)
	testClaim.Annotations = make(map[string]string)
	testValue := resource_v1.NewQuantity(123456789012345, "BinarySI")
	requests := make(api_v1.ResourceList)
	requests["storage"] = *testValue
	testClaim.Spec.Resources.Requests = requests
	testClaim.ObjectMeta.Name = "pvc-test"
	testClaim.Namespace = "default"
	testClaim.SelfLink = "/api/v1/namespaces/default/persistentvolumeclaims/pvc-test"
	testClaim.UID = "29dd7cc4-c319-11e7-83a2-005056860ac5"
	selector := new(meta_v1.LabelSelector)
	selector.MatchLabels = map[string]string{"foo": "bar"}
	testClaim.Spec.Selector = selector
	storageClass := "foo"
	testClaim.Spec.StorageClassName = &storageClass
	return testClaim
}
func TestNonZeroGetClaimSize(t *testing.T) {
	claim := getTestPVC()
	dockerClient := &dockervol.DockerVolumePlugin{
		ListOfStorageResourceOptions: []string{"size", "sizeInGiB"},
		FactorForConversion:          1073741824}

	val := getClaimSizeForFactor(claim, dockerClient, 0)
	if val == 0 {
		t.Error("Claim size should be non zero value")
	}
}
func TestZeroClaimSize(t *testing.T) {
	claim := getTestPVC()
	dockerClient := &dockervol.DockerVolumePlugin{
		FactorForConversion:          0,
		ListOfStorageResourceOptions: []string{"size", "sizeInGiB"}}
	val := getClaimSizeForFactor(claim, dockerClient, 0)
	if val != 0 {
		t.Error("Claim size should be zero for invalid FactorForConversion")
	}

	dockerClient = &dockervol.DockerVolumePlugin{
		FactorForConversion:          1073741824,
		ListOfStorageResourceOptions: nil}
	val = getClaimSizeForFactor(claim, dockerClient, 0)
	if val != 0 {
		t.Error("Claim size should be zero for invalid Storage Resource")
	}
}

func getTestStorageClass() *storage_v1.StorageClass {
	testClass := new(storage_v1.StorageClass)
	testClass.ObjectMeta.Name = "pvc"
	testClass.Namespace = "/apis/storage.k8s.io/v1/storageclasses/foo"
	testClass.UID = "76fe311a-b511-11e7-8025-005056860ac5"
	testClass.ResourceVersion = "78963"
	testClass.Generation = 0
	testClass.Provisioner = "dory"
	testClass.Parameters = make(map[string]string)
	return testClass
}

func getTestProvisioner() *Provisioner {
	config := new(rest.Config)
	kubeClient, _ := kubernetes.NewForConfig(config)
	p := NewProvisioner(kubeClient, "dory", true, true)
	return p
}

func TestGetPersistentVolume(t *testing.T) {
	p := getTestProvisioner()
	pv, _ := p.newPersistentVolume("pv-test", getStorageClassParams(), getTestPVC(), getTestStorageClass())
	vol, _ := getPersistentVolume(pv)
	if vol == nil {
		t.Error("unable to retrieve volume from pv interface")
	}
}

func TestGetPersistentVolumeClaim(t *testing.T) {
	claim, _ := getPersistentVolumeClaim(getTestPVC())
	if claim == nil {
		t.Error("unable to retrieve claim from pvc interface")
	}
}

func TestGetClaimMatchLabels(t *testing.T) {
	matchLabels := getClaimMatchLabels(getTestPVC())
	if matchLabels["foo"] != "bar" {
		t.Error("unable to retrieve match labels from pvc")
	}
}

func TestGetClaimClassName(t *testing.T) {
	name := getClaimClassName(getTestPVC())
	if name == "" {
		t.Error("claim name should not be empty for claim", getTestPVC())
	}
}

func TestGetStorageClass(t *testing.T) {
	class, _ := getStorageClass(getTestStorageClass())
	if class == nil {
		t.Error("unable to retrieve storage class")
	}
}

func TestGetBestVolName(t *testing.T) {
	p := getTestProvisioner()
	volName := p.getBestVolName(getTestPVC(), getTestStorageClass())
	if volName == "" {
		t.Error("volname should not be empty")
	}
}

func TestEmptyVolumeCreate(t *testing.T) {
	options := &dockervol.Options{StripK8sFromOptions: true}
	dvp, _ := dockervol.NewDockerVolumePlugin(options)
	optionsMap := make(map[string]interface{})
	optionsMap["description"] = "empty volume"
	_, err := dvp.Create("", optionsMap)
	if err == nil {
		t.Error("expected error on empty volume name")
	}
}
