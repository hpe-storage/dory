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
	storage_v1 "k8s.io/api/storage/v1"
	storage_v1beta1 "k8s.io/api/storage/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"strings"
)

func (p *Provisioner) listAllClasses(options meta_v1.ListOptions) (runtime.Object, error) {
	return p.kubeClient.StorageV1().StorageClasses().List(options)
}
func (p *Provisioner) watchAllClasses(options meta_v1.ListOptions) (watch.Interface, error) {
	return p.kubeClient.StorageV1().StorageClasses().Watch(options)
}

func (p *Provisioner) listBetaAllClasses(options meta_v1.ListOptions) (runtime.Object, error) {
	return p.kubeClient.StorageV1beta1().StorageClasses().List(options)
}
func (p *Provisioner) watchBetaAllClasses(options meta_v1.ListOptions) (watch.Interface, error) {
	return p.kubeClient.StorageV1beta1().StorageClasses().Watch(options)
}

//NewClassReflector provides a controller that watches for PersistentVolumeClasss and takes action on them
func (p *Provisioner) newClassReflector(kubeClient *kubernetes.Clientset) (cache.Store, *cache.Reflector) {
	classStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)
	var classReflector *cache.Reflector
	// In 1.6 and above classes are out of beta
	classListWatch := &cache.ListWatch{
		ListFunc:  p.listAllClasses,
		WatchFunc: p.watchAllClasses,
	}
	classReflector = cache.NewReflector(classListWatch, &storage_v1.StorageClass{}, classStore, 0)

	// if we're dealing with 1.5, classes are still in beta
	if p.serverVersion.Major == "1" && p.serverVersion.Minor == "5" {
		classListWatch = &cache.ListWatch{
			ListFunc:  p.listBetaAllClasses,
			WatchFunc: p.watchBetaAllClasses,
		}
		classReflector = cache.NewReflector(classListWatch, &storage_v1beta1.StorageClass{}, classStore, 0)
	}

	return classStore, classReflector
}

func (p *Provisioner) getClass(className string) (*storage_v1.StorageClass, error) {
	classObj, found, err := p.classStore.GetByKey(className)
	if err != nil {
		util.LogError.Printf("error getting class named %s. err=%v", className, err)
		return nil, err
	}
	if !found {
		util.LogError.Printf("unable to find a class named %s", className)
		return nil, fmt.Errorf("unable to find a class named %s", className)
	}
	return getStorageClass(classObj)
}

func (p *Provisioner) getClassOverrides(optionsMap map[string]string) []string {
	var overrides []string
	if val, ok := optionsMap[allowOverrides]; ok {
		util.LogDebug.Printf("allowOverrides %s", val)
		for k, v := range strings.Split(val, ",") {
			// remove leading and trailing spaces from value before Trim (needed to support multiline overrides e.g ", ")
			v = strings.TrimSpace(v)
			if len(v) > 0 && v != "" {
				util.LogDebug.Printf("processing iter :%v value :%v", k, v)
				overrides = append(overrides, v)
			}
		}
	}
	util.LogDebug.Printf("resulting overrides :%#v  dockerOpts :%#v", overrides, optionsMap)
	return overrides
}

func getStorageClass(t interface{}) (*storage_v1.StorageClass, error) {
	switch t := t.(type) {
	default:
		return nil, fmt.Errorf("unexpected type %T for %v", t, t)
	case *storage_v1.StorageClass:
		return t, nil
	case *storage_v1beta1.StorageClass:
		return &storage_v1.StorageClass{
			TypeMeta:    t.TypeMeta,
			ObjectMeta:  t.ObjectMeta,
			Provisioner: t.Provisioner,
			Parameters:  t.Parameters,
		}, nil
	case storage_v1beta1.StorageClass:
		return &storage_v1.StorageClass{
			TypeMeta:    t.TypeMeta,
			ObjectMeta:  t.ObjectMeta,
			Provisioner: t.Provisioner,
			Parameters:  t.Parameters,
		}, nil
	case storage_v1.StorageClass:
		return &t, nil
	}
}
