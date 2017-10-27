package provisioner

import (
	resource_v1 "k8s.io/apimachinery/pkg/api/resource"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"nimblestorage/pkg/docker/dockervol"
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
	if outputOption["size"] != getStorageClassParams()["size"] {
		t.Error("size should be not set for for invalid listOfStorageResourceOptions")
	}
}
func getTestPVC() *api_v1.PersistentVolumeClaim {
	testClaim := new(api_v1.PersistentVolumeClaim)
	testValue := resource_v1.NewQuantity(123456789012345, "BinarySI")
	requests := make(api_v1.ResourceList)
	requests["storage"] = *testValue
	testClaim.Spec.Resources.Requests = requests
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
