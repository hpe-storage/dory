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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"strings"
)

func (p *Provisioner) listAllClaims(options meta_v1.ListOptions) (runtime.Object, error) {
	return p.kubeClient.Core().PersistentVolumeClaims(meta_v1.NamespaceAll).List(options)
}

func (p *Provisioner) watchAllClaims(options meta_v1.ListOptions) (watch.Interface, error) {
	return p.kubeClient.Core().PersistentVolumeClaims(meta_v1.NamespaceAll).Watch(options)
}

//NewClaimController provides a controller that watches for PersistentVolumeClaims and takes action on them
func (p *Provisioner) newClaimController() (cache.Store, cache.Controller) {
	claimListWatch := &cache.ListWatch{
		ListFunc:  p.listAllClaims,
		WatchFunc: p.watchAllClaims,
	}

	return cache.NewInformer(
		claimListWatch,
		&api_v1.PersistentVolumeClaim{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    p.addedClaim,
			UpdateFunc: p.updatedClaim,
		},
	)
}

func (p *Provisioner) addedClaim(t interface{}) {
	claim, err := getPersistentVolumeClaim(t)
	if err != nil {
		util.LogError.Printf("Failed to get persistent volume claim from %v, %s", t, err.Error())
		return
	}
	go p.processAddedClaim(claim)
}

func (p *Provisioner) processAddedClaim(claim *api_v1.PersistentVolumeClaim) {
	// is this a state we can do anything about
	if claim.Status.Phase != api_v1.ClaimPending {
		util.LogInfo.Printf("pvc %s was not in pending phase.  current phase=%s - skipping", claim.Name, claim.Status.Phase)
		return
	}

	// is this a class we support
	className := getClaimClassName(claim)
	class, err := p.getClass(className)
	if err != nil {
		util.LogError.Printf("error getting class named %s for pvc %s. err=%v", className, claim.Name, err)
		return
	}
	if !strings.HasPrefix(class.Provisioner, p.namePrefix) {
		util.LogInfo.Printf("class named %s in pvc %s did not refer to a supported provisioner (name must begin with %s).  current provisioner=%s - skipping", className, claim.Name, p.namePrefix, class.Provisioner)
		return
	}

	util.LogInfo.Printf("processAddedClaim: provisioner:%s pvc:%s  class:%s", class.Provisioner, claim.Name, className)
	p.addMessageChan(fmt.Sprintf("%s", claim.UID), nil)
	p.provisionVolume(claim, class)
}

func (p *Provisioner) updatedClaim(oldT interface{}, newT interface{}) {
	claim, err := getPersistentVolumeClaim(newT)
	if err != nil {
		util.LogError.Printf("Oops - %s\n", err.Error())
		return
	}
	util.LogDebug.Printf("updatedClaim: pvc %s current phase=%s", claim.Name, claim.Status.Phase)
	go p.sendUpdate(claim)
}

func getClaimClassName(claim *api_v1.PersistentVolumeClaim) (name string) {
	name, beta := claim.Annotations[api_v1.BetaStorageClassAnnotation]

	//if no longer in beta
	if !beta && claim.Spec.StorageClassName != nil {
		name = *claim.Spec.StorageClassName
	}
	return name
}

func getClaimMatchLabels(claim *api_v1.PersistentVolumeClaim) map[string]string {
	if claim.Spec.Selector == nil || claim.Spec.Selector.MatchLabels == nil {
		return map[string]string{}
	}
	return claim.Spec.Selector.MatchLabels
}

func getPersistentVolumeClaim(t interface{}) (*api_v1.PersistentVolumeClaim, error) {
	switch t := t.(type) {
	default:
		return nil, fmt.Errorf("unexpected type %T for %v", t, t)
	case *api_v1.PersistentVolumeClaim:
		return t, nil
	case api_v1.PersistentVolumeClaim:
		return &t, nil
	}
}

func (p *Provisioner) getClaimFromPVCName(nameSpace, claimName string) (*api_v1.PersistentVolumeClaim, error) {
	util.LogDebug.Printf("getClaimFromPVCNames called with %s/%s", nameSpace, claimName)
	if len(p.claimsStore.List()) < 1 {
		return nil, fmt.Errorf("requested pvc %s/%s was not found", nameSpace, claimName)
	}
	claimInterface, found, err := p.claimsStore.GetByKey(nameSpace + "/" + claimName)
	if err != nil {
		util.LogError.Printf("Error to retrieve pvc %s/%s : %s", nameSpace, claimName, err.Error())
		return nil, fmt.Errorf("Error to retrieve pvc %s/%s : %s", nameSpace, claimName, err.Error())
	}
	if !found {
		util.LogError.Printf("requested pvc %s/%s was not found", nameSpace, claimName)
		return nil, fmt.Errorf("requested pvc %s/%s was not found", nameSpace, claimName)
	}
	var claim *api_v1.PersistentVolumeClaim
	claim, err = getPersistentVolumeClaim(claimInterface)
	if err != nil {
		util.LogError.Printf("requested pvc %s/%s was not found : %s", nameSpace, claimName, err.Error())
		return nil, fmt.Errorf("requested pvc %s/%s was not found : %s", nameSpace, claimName, err.Error())
	}
	util.LogDebug.Printf("claim found namespace :%s name: %s", claim.Namespace, claim.Name)

	return claim, nil
}

func (p *Provisioner) getClaimOverrideOptions(claim *api_v1.PersistentVolumeClaim, overrides []string, optionsMap map[string]interface{}) map[string]interface{} {
	util.LogDebug.Printf("handling getClaimOverrideOptions for %s", p.namePrefix)
	provisionerName := p.namePrefix
	for _, override := range overrides {
		util.LogDebug.Printf("handling override :%v", override)
		for key, annotation := range claim.Annotations {
			util.LogDebug.Printf("handling annotation key :%v val :%v", key, annotation)
			if strings.HasPrefix(strings.ToLower(key), provisionerName+strings.ToLower(override)) {
				util.LogDebug.Printf("annotation key :%v val :%v matched override :%v", key, annotation, override)
				if valOpt, ok := optionsMap[override]; ok {
					util.LogInfo.Printf("key %v exist with val %v, overriding with pvc annotation %v", override, valOpt, annotation)
				}
				util.LogDebug.Printf("adding key %v val :%v to docker opts", override, annotation)
				optionsMap[override] = annotation
			}
		}
	}
	return optionsMap
}

func (p *Provisioner) getClaimNameSpace(claim *api_v1.PersistentVolumeClaim) string {
	nameSpace := "default"
	if claim != nil && claim.Namespace != "" {
		nameSpace = claim.Namespace
	}
	util.LogDebug.Printf("namespace of the claim %s is : %s", claim.Name, nameSpace)
	return nameSpace
}
