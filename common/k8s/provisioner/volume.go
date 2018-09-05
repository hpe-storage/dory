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

package provisioner

import (
	"fmt"
	"github.com/hpe-storage/dory/common/util"
	api_v1 "k8s.io/api/core/v1"
	storage_v1 "k8s.io/api/storage/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/reference"
	"strings"
)

func (p *Provisioner) listAllVolumes(options meta_v1.ListOptions) (runtime.Object, error) {
	return p.kubeClient.Core().PersistentVolumes().List(options)
}

func (p *Provisioner) watchAllVolumes(options meta_v1.ListOptions) (watch.Interface, error) {
	return p.kubeClient.Core().PersistentVolumes().Watch(options)
}

//NewVolumeController provides a controller that watches for PersistentVolumes and takes action on them
func (p *Provisioner) newVolumeController() cache.Controller {
	volListWatch := &cache.ListWatch{
		ListFunc:  p.listAllVolumes,
		WatchFunc: p.watchAllVolumes,
	}

	_, volInformer := cache.NewInformer(
		volListWatch,
		&api_v1.PersistentVolume{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    p.addedVolume,
			UpdateFunc: p.updatedVolume,
			DeleteFunc: p.deletedVolume,
		},
	)
	return volInformer
}

func (p *Provisioner) addedVolume(t interface{}) {
	vol, err := getPersistentVolume(t)
	if err != nil {
		util.LogError.Printf("unable to process pv add - %v,  %s", t, err.Error())
	}
	go p.processVolEvent("added", vol, true)
}

func (p *Provisioner) updatedVolume(oldT interface{}, newT interface{}) {
	vol, err := getPersistentVolume(newT)
	if err != nil {
		util.LogError.Printf("unable to process pv update - %v,  %s", newT, err.Error())
	}

	go p.processVolEvent("updatedVol", vol, true)
}

func (p *Provisioner) deletedVolume(t interface{}) {
	vol, err := getPersistentVolume(t)
	if err != nil {
		util.LogError.Printf("unable to process pv delete - %v,  %s", t, err.Error())
	}
	go p.processVolEvent("deletedVol", vol, false)
}

// We map updated and deleted events here incase we were not running when the pv state changed to Released.  If rmPV is true, we try to remove the pv object from the cluster.  If its false, we don't.
func (p *Provisioner) processVolEvent(event string, vol *api_v1.PersistentVolume, rmPV bool) {
	//notify the monitor
	go p.sendUpdate(vol)

	if vol.Status.Phase != api_v1.VolumeReleased || vol.Spec.PersistentVolumeReclaimPolicy != api_v1.PersistentVolumeReclaimDelete {
		util.LogInfo.Printf("%s event: pv:%s phase:%v (reclaim policy:%v) - skipping", event, vol.Name, vol.Status.Phase, vol.Spec.PersistentVolumeReclaimPolicy)
		return
	}
	if _, found := vol.Annotations[k8sProvisionedBy]; !found {
		util.LogInfo.Printf("%s event: pv:%s phase:%v (reclaim policy:%v) - missing annotation skipping", event, vol.Name, vol.Status.Phase, vol.Spec.PersistentVolumeReclaimPolicy)
		return
	}

	if !strings.HasPrefix(vol.Annotations[k8sProvisionedBy], p.namePrefix) {
		util.LogInfo.Printf("%s event: pv:%s phase:%v (reclaim policy:%v) provisioner:%v - unknown provisioner skipping", event, vol.Name, vol.Status.Phase, vol.Spec.PersistentVolumeReclaimPolicy, vol.Annotations[k8sProvisionedBy])
		return
	}

	util.LogDebug.Printf("%s event: cleaning up pv:%s phase:%v", event, vol.Name, vol.Status.Phase)
	p.deleteVolume(vol, rmPV)
}

func getPersistentVolume(t interface{}) (*api_v1.PersistentVolume, error) {
	switch t := t.(type) {
	default:
		return nil, fmt.Errorf("unexpected type %T for %v", t, t)
	case *api_v1.PersistentVolume:
		return t, nil
	case api_v1.PersistentVolume:
		return &t, nil
	}
}
func (p *Provisioner) getVolumeNameFromClaimName(nameSpace, claimName string) (string, error) {
	// get the pv corresponding to this pvc and substitute with pv (docker volume name)
	util.LogDebug.Printf("handling %s with pvcName %s", cloneOfPVC, claimName)
	claim, err := p.getClaimFromPVCName(nameSpace, claimName)
	if err != nil {
		return "", err
	}
	if claim == nil || claim.Spec.VolumeName == "" {
		return "", fmt.Errorf("no volume found for claim %s", claimName)
	}
	return claim.Spec.VolumeName, nil
}

func (p *Provisioner) getDockerOptions(params map[string]string, class *storage_v1.StorageClass, claimSizeinGiB int, listOfOptions []string, nameSpace string) (map[string]interface{}, error) {
	dockOpts := make(map[string]interface{}, len(params))
	foundSizeKey := false
	for key, value := range params {
		if key == cloneOfPVC {
			pvName, err := p.getVolumeNameFromClaimName(nameSpace, value)
			if err != nil {
				util.LogError.Printf("Error to retrieve pvc %s/%s : %s return existing options", nameSpace, value, err.Error())
				p.eventRecorder.Event(class, api_v1.EventTypeWarning, "ProvisionStorage",
					fmt.Sprintf("Error to retrieve pvc %s/%s : %s", nameSpace, value, err.Error()))
				return nil, err
			}
			util.LogDebug.Printf("setting key : cloneOf value : %v", pvName)
			dockOpts["cloneOf"] = pvName
			continue
		}
		dockOpts[key] = value
		util.LogDebug.Printf("storageclass option key:%v value:%v", key, value)
		if claimSizeinGiB > 0 && contains(listOfOptions, key) {
			foundSizeKey = true
			for _, option := range listOfOptions {
				if key == option {
					util.LogInfo.Printf("storageclass option matched storage resource option:%s ,overriding the value to %d", key, claimSizeinGiB)
					dockOpts[key] = claimSizeinGiB
					break
				}
			}
		}
	}
	if claimSizeinGiB > 0 && !foundSizeKey {
		util.LogDebug.Print("storage class does not contain size key, overriding to claim size")
		dockOpts["size"] = claimSizeinGiB
	}
	return dockOpts, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (p *Provisioner) newPersistentVolume(pvName string, params map[string]string, claim *api_v1.PersistentVolumeClaim, class *storage_v1.StorageClass) (*api_v1.PersistentVolume, error) {
	claimRef, err := reference.GetReference(scheme.Scheme, claim)
	if err != nil {
		util.LogError.Printf("unable to get reference for claim %v. %s", claim, err)
		return nil, err
	}

	claimName := getClaimClassName(claim)
	if class.Parameters == nil {
		class.Parameters = make(map[string]string)
	}
	class.Parameters["name"] = pvName

	if class.ReclaimPolicy == nil {
		class.ReclaimPolicy = new(api_v1.PersistentVolumeReclaimPolicy)
		*class.ReclaimPolicy = api_v1.PersistentVolumeReclaimDelete
	}

	pv := &api_v1.PersistentVolume{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      pvName,
			Namespace: claim.Namespace,
			Labels:    getClaimMatchLabels(claim),
			Annotations: map[string]string{
				"volume.beta.kubernetes.io/storage-class": claimName,
				k8sProvisionedBy:                          class.Provisioner,
				p.dockerVolNameAnnotation:                 pvName,
			},
		},
		Spec: api_v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *class.ReclaimPolicy,
			AccessModes:                   claim.Spec.AccessModes,
			ClaimRef:                      claimRef,
			StorageClassName:              claimName,
			Capacity: api_v1.ResourceList{
				api_v1.ResourceName(api_v1.ResourceStorage): claim.Spec.Resources.Requests[api_v1.ResourceName(api_v1.ResourceStorage)],
			},
			PersistentVolumeSource: api_v1.PersistentVolumeSource{
				FlexVolume: &api_v1.FlexVolumeSource{
					Driver:  class.Provisioner,
					Options: params,
				},
			},
		},
	}
	return pv, nil
}
