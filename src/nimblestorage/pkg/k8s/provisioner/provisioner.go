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
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	storage_v1 "k8s.io/client-go/pkg/apis/storage/v1"
	"k8s.io/client-go/tools/cache"
	"nimblestorage/pkg/chain"
	"nimblestorage/pkg/docker/dockervol"
	"nimblestorage/pkg/jconfig"
	"nimblestorage/pkg/util"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	dockerVolumeName   = "docker-volume-name"
	flexVolumeBasePath = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/"
	k8sProvisionedBy   = "pv.kubernetes.io/provisioned-by"
)

var (
	// resyncPeriod describes how often to get a full resync (0=never)
	resyncPeriod   = 15 * time.Minute
	maxWaitForBind = 5 * time.Minute
)

// Provisioner provides dynamic pvs based on pvcs and storage classes.
type Provisioner struct {
	kubeClient *kubernetes.Clientset
	// serverVersion is the k8s server version
	serverVersion *version.Info
	// classStore provides access to StorageClasses on the cluster
	classStore              cache.Store
	claim2chan              map[string]chan *api_v1.PersistentVolumeClaim
	claim2chanLock          *sync.Mutex
	affectDockerVols        bool
	namePrefix              string
	dockerVolNameAnnotation string
}

// addClaimChan adds a chan to the map index by claim id
func (p *Provisioner) addClaimChan(claimID string, c chan *api_v1.PersistentVolumeClaim) {
	p.claim2chanLock.Lock()
	defer p.claim2chanLock.Unlock()
	p.claim2chan[claimID] = c
}

// getClaimChan gets a chan from the map index by claim id
func (p *Provisioner) getClaimChan(claimID string) chan *api_v1.PersistentVolumeClaim {
	p.claim2chanLock.Lock()
	defer p.claim2chanLock.Unlock()
	return p.claim2chan[claimID]
}

// removeClaimChan closes (if open) chan and removes it from the map
func (p *Provisioner) removeClaimChan(claimID string) {
	p.claim2chanLock.Lock()
	defer p.claim2chanLock.Unlock()
	if p.claim2chan[claimID] == nil {
		return
	}
	select {
	case <-p.claim2chan[claimID]:
	default:
		close(p.claim2chan[claimID])
	}
	delete(p.claim2chan, claimID)
}

//NewProvisioner provides a Provisioner for a k8s cluster
func NewProvisioner(clientSet *kubernetes.Clientset, provisionerName string, affectDockerVols bool) *Provisioner {
	return &Provisioner{
		kubeClient:              clientSet,
		claim2chan:              make(map[string]chan *api_v1.PersistentVolumeClaim, 10),
		claim2chanLock:          &sync.Mutex{},
		affectDockerVols:        affectDockerVols,
		namePrefix:              provisionerName + "/",
		dockerVolNameAnnotation: provisionerName + "/" + dockerVolumeName,
	}
}

// Start the provision workflow.  Note that Start will block until there are storage classes found.
func (p *Provisioner) Start(stop chan struct{}) {
	var err error
	// get the server version
	p.serverVersion, err = p.kubeClient.Discovery().ServerVersion()
	if err != nil {
		util.LogError.Printf("Unable to get server version.  %s", err.Error())
	}

	// Get the StorageClass store and start it's reflector
	var classReflector *cache.Reflector
	p.classStore, classReflector = p.newClassReflector(p.kubeClient)
	go classReflector.RunUntil(stop)

	// Get and start the Persistent Volume Claim Controller
	claimInformer := p.newClaimController()
	go claimInformer.Run(stop)

	volInformer := p.newVolumeController()
	go volInformer.Run(stop)

	// Wait for our reflector to load (or for someone to add a Storage Class)
	p.waitForClasses()

	util.LogDebug.Printf("provisioner (prefix=%s) has been started and is watching a server with version %s.", p.namePrefix, p.serverVersion)

}

func (p *Provisioner) deleteVolume(pv *api_v1.PersistentVolume, rmPV bool) {
	deleteChain := chain.NewChain(3, 3*time.Second)
	provisioner := pv.Annotations[k8sProvisionedBy]

	// if the pv was just deleted, make sure we clean up the docker volume
	if p.affectDockerVols {
		dockerClient, err := p.newDockerClient(provisioner)
		if err != nil {
			//TODO send event
			return
		}
		vol := p.getDockerVolume(dockerClient, pv.Name)
		if vol != nil && vol.Name == pv.Name {
			util.LogDebug.Printf("Docker volume with name %s found.  Delete using %s.", pv.Name, provisioner)
			deleteChain.AppendRunner(&deleteDockerVol{
				name:   pv.Name,
				client: dockerClient,
			})
		}
	}

	if rmPV {
		deleteChain.AppendRunner(&deletePersistentVolume{
			kubeClient: p.kubeClient,
			vol:        pv,
		})
	}

	err := deleteChain.Execute()

	if err != nil {
		//TODO send event
		util.LogError.Printf("error deleting %v. err=%v", pv, err)
	}

}

func (p *Provisioner) provisionVolume(claim *api_v1.PersistentVolumeClaim, updates chan *api_v1.PersistentVolumeClaim, class *storage_v1.StorageClass) {
	defer p.removeClaimChan(fmt.Sprintf("%s", claim.GetUID()))

	// find a name...
	volName := claim.Spec.VolumeName
	if volName == "" {
		volName = claim.GetGenerateName()
		if volName == "" {
			volName = claim.Annotations[p.dockerVolNameAnnotation]
			if volName == "" {
				volName = fmt.Sprintf("%s-%s", class.Name, claim.UID)
			}
		}
	}

	pv, err := p.newPersistentVolume(volName, claim, class)
	if err != nil {
		util.LogError.Printf("error building pv from %v and %v. err=%v", claim, class, err)
		return
	}

	provisionChain := chain.NewChain(3, 3*time.Second)

	if p.affectDockerVols {
		var dockerClient *dockervol.DockerVolumePlugin
		dockerClient, err = p.newDockerClient(class.Provisioner)
		if err != nil {
			//TODO send event
			return
		}
		vol := p.getDockerVolume(dockerClient, volName)
		if vol != nil && volName == vol.Name {
			util.LogError.Printf("error provisioning pv from %v and %v. err=Docker volume with this name was found %v.", claim, class, vol)
			return
		}

		provisionChain.AppendRunner(&createDockerVol{
			requestedName: pv.Name,
			options:       getDockerOptions(class.Parameters),
			client:        dockerClient,
		})
	}

	provisionChain.AppendRunner(&createPersistentVolume{
		kubeClient: p.kubeClient,
		vol:        pv,
	})

	provisionChain.AppendRunner(&monitorBind{
		updateChan: updates,
		origClaim:  claim,
		pChain:     provisionChain,
	})

	err = provisionChain.Execute()

	if err != nil {
		//TODO send event
		util.LogError.Printf("error provisioning pv from %v and %v. err=%v", claim, class, err)
	}

}

func (p *Provisioner) newDockerClient(provisionerName string) (*dockervol.DockerVolumePlugin, error) {
	driverName := strings.Split(provisionerName, "/")
	if len(driverName) < 2 {
		util.LogInfo.Printf("Unable to parse provisioner name %s.", provisionerName)
		return nil, fmt.Errorf("unable to parse provisioner name %s", provisionerName)
	}
	configPathName := fmt.Sprintf("%s%s/%s.json", flexVolumeBasePath, strings.Replace(provisionerName, "/", "~", 1), driverName[1])
	util.LogDebug.Printf("looking for %s", configPathName)

	socketFile := ""
	strip := true

	c, err := jconfig.NewConfig(configPathName)
	if err != nil {
		util.LogInfo.Printf("Unable to process config at %s, %v.  Using defaults.", configPathName, err)
	} else {
		socketFile = c.GetString("dockerVolumePluginSocketPath")
		if _, err = os.Stat(socketFile); os.IsNotExist(err) {
			util.LogError.Printf("Unable to open socket file at %s, it does not exist.", socketFile)
		}

		b, err := c.GetBool("stripK8sFromOptions")
		if err == nil {
			strip = b
		}
	}

	return dockervol.NewDockerVolumePlugin(socketFile, strip), nil
}

// block until there are some classes defined in the cluster
func (p *Provisioner) waitForClasses() {
	i := 0
	for len(p.classStore.List()) < 1 {
		if i > 29 {
			util.LogInfo.Printf("No StorageClass found.  Unable to make progress.")
			i = 0
		}
		time.Sleep(time.Second)
		i++
	}
}

func (p *Provisioner) getDockerVolume(dockerClient *dockervol.DockerVolumePlugin, volName string) *dockervol.DockerVolume {
	vol, err := dockerClient.Get(volName)
	if err != nil {
		return nil
	}
	return &vol.Volume
}

type createDockerVol struct {
	requestedName string
	returnedName  string
	options       map[string]interface{}
	client        *dockervol.DockerVolumePlugin
}

func (c createDockerVol) Name() string {
	return reflect.TypeOf(c).Name()
}

func (c *createDockerVol) Run() (name interface{}, err error) {
	c.returnedName, err = c.client.Create(c.requestedName, c.options)
	name = c.returnedName
	return name, err
}

func (c *createDockerVol) Rollback() (err error) {
	if c.returnedName != "" {
		return c.client.Delete(c.returnedName)
	}
	return nil
}

type deleteDockerVol struct {
	name   string
	client *dockervol.DockerVolumePlugin
}

func (c deleteDockerVol) Name() string {
	return reflect.TypeOf(c).Name()
}

func (c *deleteDockerVol) Run() (name interface{}, err error) {
	return nil, c.client.Delete(c.name)
}

func (c *deleteDockerVol) Rollback() (err error) {
	//no op
	return nil
}

type createPersistentVolume struct {
	kubeClient *kubernetes.Clientset
	vol        *api_v1.PersistentVolume
}

func (c createPersistentVolume) Name() string {
	return reflect.TypeOf(c).Name()
}

func (c *createPersistentVolume) Run() (name interface{}, err error) {
	return c.kubeClient.Core().PersistentVolumes().Create(c.vol)
}

func (c *createPersistentVolume) Rollback() (err error) {
	return c.kubeClient.Core().PersistentVolumes().Delete(c.vol.Name, &meta_v1.DeleteOptions{})
}

type deletePersistentVolume struct {
	kubeClient *kubernetes.Clientset
	vol        *api_v1.PersistentVolume
}

func (d deletePersistentVolume) Name() string {
	return reflect.TypeOf(d).Name()
}

func (d *deletePersistentVolume) Run() (name interface{}, err error) {
	err = d.kubeClient.Core().PersistentVolumes().Delete(d.vol.Name, &meta_v1.DeleteOptions{})
	return nil, err
}

func (d *deletePersistentVolume) Rollback() (err error) {
	//no op
	return nil
}

type monitorBind struct {
	updateChan chan *api_v1.PersistentVolumeClaim
	origClaim  *api_v1.PersistentVolumeClaim
	pChain     *chain.Chain
}

func (m *monitorBind) Name() string {
	return reflect.TypeOf(m).Name()
}

func (m *monitorBind) Run() (name interface{}, err error) {
	for true {
		select {
		case claim := <-m.updateChan:
			util.LogDebug.Printf("pvc %s updated (UID=%s).  Status is now %s", claim.Name, claim.GetUID(), claim.Status.Phase)
			if claim.Status.Phase == api_v1.ClaimBound {
				util.LogDebug.Printf("pvc %s is bound to pv %s", claim.Name, claim.Spec.VolumeName)
				vol, err := getPersistentVolume(m.pChain.GetRunnerOutput("createPersistentVolume"))
				if err != nil {
					//TODO sent event
					return nil, err
				}
				if vol.Name != claim.Spec.VolumeName {
					//TODO sent event
					return nil, fmt.Errorf("pvc %s was satisfied by %s, the pv provisioned was %s", claim.Name, claim.Spec.VolumeName, vol.Name)
				}
				return claim.Spec.VolumeName, nil
			}
		case <-time.After(maxWaitForBind):
			util.LogError.Printf("pvc %s timed out waiting for bind status (UID=%s)", m.origClaim.Name, m.origClaim.GetUID())
			return nil, fmt.Errorf("pvc %s (%s) not bound after timeout", m.origClaim.Name, m.origClaim.GetUID())
		}
	}
	return nil, nil
}

func (m *monitorBind) Rollback() (err error) {
	return nil
}
