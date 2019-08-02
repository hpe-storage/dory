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
	"github.com/hpe-storage/dory/common/chain"
	"github.com/hpe-storage/dory/common/docker/dockervol"
	"github.com/hpe-storage/dory/common/jconfig"
	"github.com/hpe-storage/dory/common/util"
	uuid "github.com/satori/go.uuid"
	"k8s.io/api/core/v1"
	api_v1 "k8s.io/api/core/v1"
	storage_v1 "k8s.io/api/storage/v1"
	resource_v1 "k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	dockerVolumeName   = "docker-volume-name"
	flexVolumeBasePath = "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/"
	k8sProvisionedBy   = "pv.kubernetes.io/provisioned-by"
	chainTimeout       = 2 * time.Minute
	chainRetries       = 2
	//TODO allow this to be set per docker volume driver
	maxCreates = 4
	//TODO allow this to be set per docker volume driver
	maxDeletes                 = 10
	defaultSocketFile          = "/run/docker/plugins/nimble.sock"
	defaultfactorForConversion = 1073741824
	defaultStripValue          = true
	maxWaitForClaims           = 60
	allowOverrides             = "allowOverrides"
	cloneOf                    = "cloneOf"
	cloneOfPVC                 = "cloneOfPVC"
	manager                    = "manager"
	managerName                = "k8s"
	id2chanMapSize             = 1024
	deleteRetrySleep           = 5 * time.Second
)

var (
	// resyncPeriod describes how often to get a full resync (0=never)
	resyncPeriod = 5 * time.Minute
	// maxWaitForBind refers to a single execution of the retry loop
	maxWaitForBind = 30 * time.Second
	// statusLoggingWait is only used when debug is true
	statusLoggingWait                   = 5 * time.Second
	defaultListOfStorageResourceOptions = []string{"size", "sizeInGiB"}
	defaultDockerOptions                = map[string]interface{}{"mountConflictDelay": 30, manager: managerName}
)

// Provisioner provides dynamic pvs based on pvcs and storage classes.
type Provisioner struct {
	kubeClient *kubernetes.Clientset
	// serverVersion is the k8s server version
	serverVersion *version.Info
	// classStore provides access to StorageClasses on the cluster
	classStore              cache.Store
	claimsStore             cache.Store
	id2chan                 map[string]chan *updateMessage
	id2chanLock             *sync.Mutex
	affectDockerVols        bool
	namePrefix              string
	dockerVolNameAnnotation string
	eventRecorder           record.EventRecorder
	provisionCommandChains  uint32
	deleteCommandChains     uint32
	parkedCommands          uint32
	debug                   bool
}

type updateMessage struct {
	pv  *api_v1.PersistentVolume
	pvc *api_v1.PersistentVolumeClaim
}

// addMessageChan adds a chan to the map index by id.  If channel is nil, a new chan is allocated and added
func (p *Provisioner) addMessageChan(id string, channel chan *updateMessage) {
	p.id2chanLock.Lock()
	defer p.id2chanLock.Unlock()

	if _, found := p.id2chan[id]; found {
		return
	}
	if channel != nil {
		util.LogDebug.Printf("addMessageChan: adding %s", id)
		p.id2chan[id] = channel
	} else {
		util.LogDebug.Printf("addMessageChan: creating %s", id)
		p.id2chan[id] = make(chan *updateMessage, 1024)
	}
}

// getMessageChan gets a chan from the map index by claim or vol id to be passed to the consumer.
// Do not use this pointer to send data as the channel might be closed right after the
// pointer is returned.  Instead use sendUpdate(...).
func (p *Provisioner) getMessageChan(id string) chan *updateMessage {
	p.id2chanLock.Lock()
	defer p.id2chanLock.Unlock()

	return p.id2chan[id]
}

// sendUpdate sends an claim or volume update to the consumer.  A big lock (entire map)
// is used for now.
func (p *Provisioner) sendUpdate(t interface{}) {
	var id string
	var mess *updateMessage

	claim, _ := getPersistentVolumeClaim(t)
	if claim != nil {
		util.LogDebug.Printf("sendUpdate: pvc:%s (%s) phase:%s", claim.Name, claim.UID, claim.Status.Phase)
		id = fmt.Sprintf("%s", claim.UID)
		mess = &updateMessage{pvc: claim}
	} else {
		vol, _ := getPersistentVolume(t)
		if vol != nil {
			util.LogDebug.Printf("sendUpdate: pv:%s (%s) phase:%s", vol.Name, vol.UID, vol.Status.Phase)
			id = fmt.Sprintf("%s", vol.UID)
			mess = &updateMessage{pv: vol}
		}
	}

	// hold the big lock just to send
	p.id2chanLock.Lock()
	defer p.id2chanLock.Unlock()

	messChan := p.id2chan[id]
	if messChan == nil {
		util.LogDebug.Printf("send: skipping %s, not in map", id)
		return
	}
	messChan <- mess
}

// removeMessageChan closes (if open) chan and removes it from the map
func (p *Provisioner) removeMessageChan(claimID string, volID string) {
	util.LogDebug.Printf("removeMessageChan called with claimID %s volID %s", claimID, volID)
	p.id2chanLock.Lock()
	defer p.id2chanLock.Unlock()

	messChan := p.id2chan[claimID]
	if messChan != nil {
		delete(p.id2chan, claimID)
	}
	if byVolID, found := p.id2chan[volID]; found {
		delete(p.id2chan, volID)
		if messChan == nil {
			messChan = byVolID
		}
	}
	if messChan == nil {
		return
	}

	select {
	case <-messChan:
	default:
		close(messChan)
	}
}

//NewProvisioner provides a Provisioner for a k8s cluster
func NewProvisioner(clientSet *kubernetes.Clientset, provisionerName string, affectDockerVols bool, debug bool) *Provisioner {
	id := uuid.NewV4()
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&core_v1.EventSinkImpl{Interface: clientSet.Core().Events(v1.NamespaceAll)})
	util.LogDebug.Printf("provisioner (prefix=%s) is being created with instance id %s and id2chan capacity %d.", provisionerName, id.String(), id2chanMapSize)
	return &Provisioner{
		kubeClient:              clientSet,
		id2chan:                 make(map[string]chan *updateMessage, id2chanMapSize), //make a id to chan (updatemessage) map with a capacity of id2chanMapSize
		id2chanLock:             &sync.Mutex{},
		affectDockerVols:        affectDockerVols,
		namePrefix:              provisionerName + "/",
		dockerVolNameAnnotation: provisionerName + "/" + dockerVolumeName,
		eventRecorder:           broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("%s-%s", provisionerName, id.String())}),
		debug:                   debug,
	}
}

// update the existing volume's metadata for the claims
func (p *Provisioner) updateDockerVolumeMetadata(store cache.Store) {
	util.LogDebug.Print("updateDockerVolumeMetadata started")
	optionsMap := map[string]interface{}{manager: managerName}

	i := 0
	for len(store.List()) < 1 {
		if i > maxWaitForClaims {
			util.LogInfo.Printf("No Claims found after waiting for %d seconds. Ignoring update", maxWaitForClaims)
			return
		}
		time.Sleep(time.Second)
		i++
	}

	for _, pvc := range store.List() {
		claim, err := getPersistentVolumeClaim(pvc)
		if err != nil {
			util.LogDebug.Printf("unable to retrieve the claim from %v", pvc)
			continue
		}

		if claim.Status.Phase != api_v1.ClaimBound {
			util.LogDebug.Printf("claim %s was not bound - skipping", claim.Name)
			continue
		}

		className := getClaimClassName(claim)
		util.LogDebug.Printf("found classname %s for claim %s.", className, claim.Name)
		class, err := p.getClass(className)
		if err != nil {
			util.LogError.Printf("unable to retrieve the class object for claim %v", claim)
			continue
		}

		if !strings.HasPrefix(class.Provisioner, p.namePrefix) {
			util.LogInfo.Printf("updateDockerVolumeMetadata: class named %s in pvc %s did not refer to a supported provisioner (name must begin with %s).  current provisioner=%s - skipping", className, claim.Name, p.namePrefix, class.Provisioner)
			continue
		}

		err = p.updateVolume(claim, class.Provisioner, optionsMap)
		if err != nil {
			// we don't want to beat on the docker plugin if it doesn't support update
			// so we simply move on to the next volume if we hit an error
			util.LogError.Printf("unable to update volume %v Err: %v", claim.Spec.VolumeName, err.Error())
			continue
		}
	}

	util.LogDebug.Print("updateDockerVolumeMetadata ended")
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
	go classReflector.Run(stop)

	// Get and start the Persistent Volume Claim Controller
	var claimInformer cache.Controller
	p.claimsStore, claimInformer = p.newClaimController()
	go claimInformer.Run(stop)

	go p.updateDockerVolumeMetadata(p.claimsStore)

	volInformer := p.newVolumeController()
	go volInformer.Run(stop)

	if p.debug {
		go p.statusLogger()
	}

	// Wait for our reflector to load (or for someone to add a Storage Class)
	p.waitForClasses()

	util.LogDebug.Printf("provisioner (prefix=%s) has been started and is watching a server with version %s.", p.namePrefix, p.serverVersion)

}

func (p *Provisioner) statusLogger() {
	for {
		time.Sleep(statusLoggingWait)
		_, err := p.kubeClient.Discovery().ServerVersion()
		if err != nil {
			util.LogError.Printf("statusLogger: provision chains=%d, delete chains=%d, parked chains=%d, ids tracked=%d, connection error=%s", atomic.LoadUint32(&p.provisionCommandChains), atomic.LoadUint32(&p.deleteCommandChains), atomic.LoadUint32(&p.parkedCommands), len(p.id2chan), err.Error())
			return
		}
		util.LogInfo.Printf("statusLogger: provision chains=%d, delete chains=%d, parked chains=%d, ids tracked=%d, connection=valid", atomic.LoadUint32(&p.provisionCommandChains), atomic.LoadUint32(&p.deleteCommandChains), atomic.LoadUint32(&p.parkedCommands), len(p.id2chan))
	}
}

func (p *Provisioner) deleteVolume(pv *api_v1.PersistentVolume, rmPV bool) {
	provisioner := pv.Annotations[k8sProvisionedBy]
	util.LogDebug.Printf("in deleteVolume: cleaning up pv:%s Status:%v with deleteChain %d parkedCommands %d with affectDockerVols %v", pv.Name, pv.Status, atomic.LoadUint32(&p.deleteCommandChains), atomic.LoadUint32(&p.parkedCommands), p.affectDockerVols)
	// slow down a delete storm
	limit(&p.deleteCommandChains, &p.parkedCommands, maxDeletes)

	atomic.AddUint32(&p.deleteCommandChains, 1)
	defer atomic.AddUint32(&p.deleteCommandChains, ^uint32(0))
	deleteChain := chain.NewChain(chainRetries, deleteRetrySleep)

	// if the pv was just deleted, make sure we clean up the docker volume
	if p.affectDockerVols {
		dockerClient, _, err := p.newDockerVolumePluginClient(provisioner)
		if err != nil {
			info := fmt.Sprintf("failed to get docker client for %s while trying to delete pv %s: %v", provisioner, pv.Name, err)
			util.LogError.Print(info)
			p.eventRecorder.Event(pv, api_v1.EventTypeWarning, "DeleteVolumeGetClient", info)
			return
		}
		vol := p.getDockerVolume(dockerClient, pv.Name)
		if vol != nil && vol.Name == pv.Name {
			p.eventRecorder.Event(pv, api_v1.EventTypeNormal, "DeleteVolume", fmt.Sprintf("cleaning up volume named %s", pv.Name))
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
		p.eventRecorder.Event(pv, api_v1.EventTypeWarning, "DeleteVolume",
			fmt.Sprintf("Failed to delete volume for pv %s: %v", pv.Name, err))
	}

}

func (p *Provisioner) updateVolume(claim *api_v1.PersistentVolumeClaim, provisioner string, updateMap map[string]interface{}) error {
	util.LogDebug.Printf("updateVolume called with claim:%s, provisioner:%s and options:%v", claim.Name, provisioner, updateMap)

	// get the volume name for update
	volName := claim.Spec.VolumeName

	var dockerClient *dockervol.DockerVolumePlugin
	dockerClient, _, err := p.newDockerVolumePluginClient(provisioner)
	if err != nil {
		return err
	}

	vol := p.getDockerVolume(dockerClient, volName)
	if (vol == nil) || (vol != nil && volName != vol.Name) {
		return fmt.Errorf("error updating pv from claim: %v and provisioner :%s. err=Docker volume %v with name %s was not found ", claim, provisioner, vol, volName)
	}

	if val, ok := vol.Status[manager]; ok && val != "" {
		util.LogDebug.Printf("claim:%s has manager set to value %v - skipping", claim.Name, val)
		return nil
	}

	util.LogDebug.Printf("invoking VolumeDriver.Update with name :%s updateMap :%v", vol.Name, updateMap)
	_, err = dockerClient.Update(vol.Name, updateMap)
	if err != nil {
		return err
	}
	return nil
}

//nolint: gocyclo
// this is a complex function that we should break down a bit
func (p *Provisioner) provisionVolume(claim *api_v1.PersistentVolumeClaim, class *storage_v1.StorageClass) {

	// this can fire multiple times without issue, so we defer this even though we don't have a volume yet
	id := fmt.Sprintf("%s", claim.UID)
	defer p.removeMessageChan(id, "")

	// find a name...
	volName := p.getBestVolName(claim, class)
	//namespace of the claim
	nameSpace := p.getClaimNameSpace(claim)

	// create a copy of the storage class options for NLT-1172
	params := make(map[string]string)
	for key, value := range class.Parameters {
		params[key] = value
	}
	// add name to options
	params["name"] = volName

	pv, err := p.newPersistentVolume(volName, params, claim, class)
	if err != nil {
		util.LogError.Printf("error building pv from %v %v and %v. err=%v", claim, params, class, err)
		return
	}
	util.LogDebug.Printf("pv to be created %v", pv)

	// slow down a create storm
	limit(&p.provisionCommandChains, &p.parkedCommands, maxCreates)

	provisionChain := chain.NewChain(chainRetries, chainTimeout)
	atomic.AddUint32(&p.provisionCommandChains, 1)
	defer atomic.AddUint32(&p.provisionCommandChains, ^uint32(0))

	var dockerClient *dockervol.DockerVolumePlugin
	var dockerOptions map[string]interface{}
	dockerClient, dockerOptions, err = p.newDockerVolumePluginClient(class.Provisioner)
	if err != nil {
		util.LogError.Printf("unable to get docker client for class %v while trying to provision pvc named %s (%s): %s", class, claim.Name, id, err)
		p.eventRecorder.Event(class, api_v1.EventTypeWarning, "ProvisionVolumeGetClient",
			fmt.Sprintf("failed to get docker volume client for class %s while trying to provision claim %s (%s): %s", class.Name, claim.Name, id, err))
		return
	}
	vol := p.getDockerVolume(dockerClient, volName)
	if vol != nil && volName == vol.Name {
		util.LogError.Printf("error provisioning pv from %v and %v. err=Docker volume with this name was found %v.", claim, class, vol)
		return
	}

	sizeForDockerVolumeinGib := getClaimSizeForFactor(claim, dockerClient, 0)

	// handling storage class overrides
	overrides := p.getClassOverrides(params)
	optionsMap, err := p.getDockerOptions(params, class, sizeForDockerVolumeinGib, dockerClient.ListOfStorageResourceOptions, nameSpace)
	if err != nil {
		util.LogError.Printf("error getting Docker option from %v %v and %v. err=%v", claim, params, class, err)
		return
	}

	// get updated options map for docker after handling overrides and annotations
	optionsMap, err = p.getClaimOverrideOptions(claim, overrides, optionsMap)
	if err != nil {
		p.eventRecorder.Event(class, api_v1.EventTypeWarning, "ProvisionStorage", err.Error())
		util.LogError.Printf("error handling annotations. err=%v", err)
		return
	}

	util.LogDebug.Printf("updated optionsMap with overrides %#v", optionsMap)

	// set default docker options if not already set
	p.setDefaultDockerOptions(optionsMap, params, dockerOptions, dockerClient)
	if p.affectDockerVols {
		provisionChain.AppendRunner(&createDockerVol{
			requestedName: pv.Name,
			options:       optionsMap,
			client:        dockerClient,
		})
	}

	provisionChain.AppendRunner(&createPersistentVolume{
		kubeClient: p.kubeClient,
		vol:        pv,
	})

	provisionChain.AppendRunner(&monitorBind{
		origClaim: claim,
		pChain:    provisionChain,
		p:         p,
	})

	p.eventRecorder.Event(class, api_v1.EventTypeNormal, "ProvisionStorage", fmt.Sprintf("%s provisioning storage for pvc %s (%s) using class %s", class.Provisioner, claim.Name, id, class.Name))
	err = provisionChain.Execute()

	if err != nil {
		p.eventRecorder.Event(class, api_v1.EventTypeWarning, "ProvisionStorage",
			fmt.Sprintf("failed to create volume for claim %s with class %s: %s", claim.Name, class.Name, err))
	}

	// if we created a volume, remove its uuid from the message map
	pvol, _ := getPersistentVolume(provisionChain.GetRunnerOutput("createPersistentVolume"))
	if pvol != nil {
		p.removeMessageChan(fmt.Sprintf("%s", claim.UID), fmt.Sprintf("%s", pvol.UID))
	}

}

func (p *Provisioner) setDefaultDockerOptions(optionsMap map[string]interface{}, params map[string]string, dockerOptions map[string]interface{}, dockerClient *dockervol.DockerVolumePlugin) {
	for k, v := range dockerOptions {
		util.LogDebug.Printf("processing %s:%v", k, v)
		_, ok := params[k]
		if ok == false {
			util.LogInfo.Printf("setting the docker option %s:%v", k, v)
			val := reflect.ValueOf(v)
			optionsMap[k] = val.Interface()
		}
	}
	util.LogDebug.Printf("optionsMap %v", optionsMap)
}

func limit(watched, parked *uint32, max uint32) {
	if atomic.LoadUint32(watched) >= max {
		atomic.AddUint32(parked, 1)
		for atomic.LoadUint32(watched) >= max {
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		}
		atomic.AddUint32(parked, ^uint32(0))
	}
}

func getClaimSizeForFactor(claim *api_v1.PersistentVolumeClaim, dockerClient *dockervol.DockerVolumePlugin, sizeForDockerVolumeinGib int) int {
	requestParams := claim.Spec.Resources.Requests
	for key, val := range requestParams {
		if key == "storage" {
			if val.Format == resource_v1.BinarySI || val.Format == resource_v1.DecimalSI {
				sizeInBytes, isInt := val.AsInt64()
				if isInt && sizeInBytes > 0 {
					if dockerClient.ListOfStorageResourceOptions != nil &&
						dockerClient.FactorForConversion != 0 {
						sizeForDockerVolumeinGib = int(sizeInBytes) / dockerClient.FactorForConversion
						util.LogDebug.Printf("claimSize=%d for size=%d bytes and factorForConversion=%d", sizeForDockerVolumeinGib, sizeInBytes, dockerClient.FactorForConversion)
						return sizeForDockerVolumeinGib
					}
				}
			}
		}
	}
	return sizeForDockerVolumeinGib
}

func (p *Provisioner) newDockerVolumePluginClient(provisionerName string) (*dockervol.DockerVolumePlugin, map[string]interface{}, error) {
	driverName := strings.Split(provisionerName, "/")
	if len(driverName) < 2 {
		util.LogInfo.Printf("Unable to parse provisioner name %s.", provisionerName)
		return nil, nil, fmt.Errorf("unable to parse provisioner name %s", provisionerName)
	}
	configPathName := fmt.Sprintf("%s%s/%s.json", flexVolumeBasePath, strings.Replace(provisionerName, "/", "~", 1), driverName[1])
	util.LogDebug.Printf("looking for %s", configPathName)
	var (
		socketFile                   = defaultSocketFile
		strip                        = defaultStripValue
		listOfStorageResourceOptions = defaultListOfStorageResourceOptions
		factorForConversion          = defaultfactorForConversion
		dockerOpts                   = defaultDockerOptions
	)
	c, err := jconfig.NewConfig(configPathName)
	if err != nil {
		util.LogInfo.Printf("Unable to process config at %s, %v.  Using defaults.", configPathName, err)
	} else {
		socketFile, err = c.GetStringWithError("dockerVolumePluginSocketPath")
		if err != nil {
			socketFile = defaultSocketFile
		}
		b, err := c.GetBool("stripK8sFromOptions")
		if err == nil {
			strip = b
		}
		ss, err := c.GetStringSliceWithError("listOfStorageResourceOptions")
		if err != nil {
			listOfStorageResourceOptions = ss
		}
		i := c.GetInt64("factorForConversion")
		if i != 0 {
			factorForConversion = int(i)
		}
		defaultOpts, err := c.GetMapSlice("defaultOptions")
		if err == nil {
			util.LogDebug.Printf("parsing defaultOptions %v", defaultOpts)
			optMap := make(map[string]interface{})

			for _, values := range defaultOpts {
				for k, v := range values {
					optMap[k] = v
					util.LogDebug.Printf("key %v value %v", k, optMap[k])
				}
			}
			dockerOpts = optMap
			util.LogDebug.Printf("dockerOptions %v", dockerOpts)
		}
	}
	options := &dockervol.Options{
		SocketPath:                   socketFile,
		StripK8sFromOptions:          strip,
		ListOfStorageResourceOptions: listOfStorageResourceOptions,
		FactorForConversion:          factorForConversion,
	}
	client, er := dockervol.NewDockerVolumePlugin(options)
	return client, dockerOpts, er
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

func (p *Provisioner) getBestVolName(claim *api_v1.PersistentVolumeClaim, class *storage_v1.StorageClass) string {
	if claim.Annotations[p.dockerVolNameAnnotation] != "" {
		return fmt.Sprintf("%s-%s", claim.Namespace, claim.Annotations[p.dockerVolNameAnnotation])
	}
	if claim.GetGenerateName() != "" {
		return fmt.Sprintf("%s-%s", claim.Namespace, claim.GetGenerateName())
	}
	return fmt.Sprintf("%s-%s", class.Name, claim.UID)
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
	if err != nil {
		util.LogError.Printf("failed to create docker volume, error = %s", err.Error())
		return nil, err
	}
	util.LogInfo.Printf("created docker volume named %s", c.returnedName)

	name = c.returnedName
	return name, err
}

func (c *createDockerVol) Rollback() (err error) {
	if c.returnedName != "" {
		err = c.client.Delete(c.returnedName, managerName)
		if err != nil {
			err = c.client.Delete(c.returnedName, "")
		}
	}
	return err
}

type deleteDockerVol struct {
	name   string
	client *dockervol.DockerVolumePlugin
}

func (c deleteDockerVol) Name() string {
	return reflect.TypeOf(c).Name()
}

func (c *deleteDockerVol) Run() (name interface{}, err error) {
	err = c.client.Delete(c.name, managerName)
	if err != nil {
		err = c.client.Delete(c.name, "")
	}
	return nil, err
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
