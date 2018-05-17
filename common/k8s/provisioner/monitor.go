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
	"github.com/hpe-storage/dory/common/util"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"reflect"
	"time"
)

type monitorBind struct {
	origClaim *api_v1.PersistentVolumeClaim
	pChain    *chain.Chain
	p         *Provisioner
	vol       *api_v1.PersistentVolume
}

func (m *monitorBind) Name() string {
	return reflect.TypeOf(m).Name()
}

func (m *monitorBind) Run() (name interface{}, err error) {
	messChan := m.p.getMessageChan(fmt.Sprintf("%s", m.origClaim.UID))

	m.vol, _ = getPersistentVolume(m.pChain.GetRunnerOutput("createPersistentVolume"))
	if m.vol == nil {
		return nil, fmt.Errorf("unable to get volume for pvc %s waiting for bind status (UID=%s)", m.origClaim.Name, m.origClaim.GetUID())
	}

	// add the pv id to the map referencing this channel
	m.p.addMessageChan(fmt.Sprintf("%s", m.vol.UID), messChan)

	for name == nil && err == nil {
		name, err = m.route(messChan)
	}
	return name, err
}

func (m *monitorBind) Rollback() (err error) {
	return nil
}

func (m *monitorBind) route(channel chan *updateMessage) (interface{}, error) {
	select {
	case message := <-channel:
		if message.pvc != nil {
			name, err := m.processClaimMessage(message)
			return name, err
		}
		if message.pv != nil {
			name, err := m.processVolMessage(message)
			return name, err
		}
	case <-time.After(maxWaitForBind):
		return m.processTimeout()
	}
	return nil, nil
}

func (m *monitorBind) processClaimMessage(message *updateMessage) (name interface{}, err error) {
	claim := message.pvc
	util.LogDebug.Printf("pvc %s updated (UID=%s).  Status is now %s", claim.Name, claim.GetUID(), claim.Status.Phase)
	if claim.Status.Phase == api_v1.ClaimBound {
		if m.vol.Name != claim.Spec.VolumeName {
			info := fmt.Sprintf("pvc %s was satisfied by %s, the pv provisioned was %s", claim.Name, claim.Spec.VolumeName, m.vol.Name)
			m.p.eventRecorder.Event(claim, api_v1.EventTypeWarning, "MonitorBind", info)
			// roll back here or our volume will be left hanging
			return nil, fmt.Errorf("%s", info)
		}
		util.LogDebug.Printf("pvc %s was satisfied by pv %s", claim.Name, claim.Spec.VolumeName)
		return claim.Spec.VolumeName, nil
	} else if claim.Status.Phase == api_v1.ClaimLost {
		info := fmt.Sprintf("pvc %s was lost, reverting volume create (UID=%s)", claim.Name, claim.UID)
		util.LogError.Print(info)
		m.p.eventRecorder.Event(m.origClaim, api_v1.EventTypeWarning, "MonitorBind", info)
		// roll back here since the claim was lost
		return nil, fmt.Errorf("pvc %s was lost, reverting volume create (UID=%s)", claim.Name, claim.UID)
	}
	return nil, nil
}

func (m *monitorBind) processVolMessage(message *updateMessage) (name interface{}, err error) {
	volume := message.pv
	util.LogDebug.Printf("pv %s updated (UID=%s).  Status is now %s", volume.Name, volume.UID, volume.Status.Phase)
	switch volume.Status.Phase {
	case api_v1.VolumeBound:
		if m.origClaim.UID != volume.Spec.ClaimRef.UID {
			info := fmt.Sprintf("pv %s satisfied pvc %s (%s), expecting %s", volume.Name, volume.Spec.ClaimRef.Name, volume.Spec.ClaimRef.UID, m.origClaim.Name)
			m.p.eventRecorder.Event(volume, api_v1.EventTypeWarning, "MonitorBind", info)
			util.LogError.Printf(info)
			//don't roll back here since the volume we created is bound to something
			return volume.Name, nil
		}
		util.LogInfo.Printf("pv %s satisfied pvc %s (%s)", volume.Name, volume.Spec.ClaimRef.Name, volume.Spec.ClaimRef.UID)
		return volume.Name, nil

	case api_v1.VolumeReleased:
		info := fmt.Sprintf("pv %s has been released, claimref was %s (waiting for %s)", volume.Name, volume.Spec.ClaimRef.UID, m.origClaim.UID)
		util.LogInfo.Printf(info)
		// don't roll back here since the volume will be deleted by the normal workflow
		return volume.Name, nil
	}
	return nil, nil
}

func (m *monitorBind) processTimeout() (interface{}, error) {
	info := fmt.Sprintf("pvc %s timed out waiting for bind status, reverting volume create (UID=%s)", m.origClaim.Name, m.origClaim.UID)
	util.LogError.Print(info)
	m.p.eventRecorder.Event(m.origClaim, api_v1.EventTypeWarning, "MonitorBind", info)
	return nil, fmt.Errorf("pvc %s (%s) not bound after timeout", m.origClaim.Name, m.origClaim.GetUID())
}
