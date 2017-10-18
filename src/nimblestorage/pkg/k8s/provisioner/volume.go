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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/api/v1/ref"
	storage_v1 "k8s.io/client-go/pkg/apis/storage/v1"
	"k8s.io/client-go/tools/cache"
	"nimblestorage/pkg/util"
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

func getDockerOptions(params map[string]string) map[string]interface{} {
	dockOpts := make(map[string]interface{}, len(params))
	for key, value := range params {
		dockOpts[key] = value
	}
	return dockOpts
}

func (p *Provisioner) newPersistentVolume(pvName string, claim *api_v1.PersistentVolumeClaim, class *storage_v1.StorageClass) (*api_v1.PersistentVolume, error) {
	claimRef, err := ref.GetReference(scheme.Scheme, claim)
	if err != nil {
		util.LogError.Printf("unable to get reference for claim %v. %s", claim, err)
		return nil, err
	}

	claimName := getClaimClassName(claim)
	class.Parameters["name"] = pvName

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
			PersistentVolumeReclaimPolicy: api_v1.PersistentVolumeReclaimDelete,
			AccessModes:                   claim.Spec.AccessModes,
			ClaimRef:                      claimRef,
			StorageClassName:              claimName,
			Capacity: api_v1.ResourceList{
				api_v1.ResourceName(api_v1.ResourceStorage): claim.Spec.Resources.Requests[api_v1.ResourceName(api_v1.ResourceStorage)],
			},
			PersistentVolumeSource: api_v1.PersistentVolumeSource{
				FlexVolume: &api_v1.FlexVolumeSource{
					Driver:  class.Provisioner,
					Options: class.Parameters,
				},
			},
		},
	}

	return pv, nil
}
