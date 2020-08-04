/*
Copyright 2020 The Kube Diagnoser Authors.

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

package sourcemanager

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// SourceManager manages abnormals in the system.
type SourceManager interface {
	Run(<-chan struct{})
	SyncAbnormal(diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
}

// sourceManagerImpl implements SourceManager interface.
type sourceManagerImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Client knows how to perform CRUD operations on Kubernetes objects.
	Client client.Client
	// Log represents the ability to log messages.
	Log logr.Logger
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme *runtime.Scheme
	// Cache knows how to load Kubernetes objects.
	Cache cache.Cache
	// NodeName specifies the node name.
	NodeName string

	// Channel for queuing Abnormals to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Abnormal
	// Channel for queuing Abnormals to be processed by information manager.
	informationManagerCh chan diagnosisv1.Abnormal
}

// NewSourceManager creates a new SourceManager.
func NewSourceManager(
	ctx context.Context,
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	sourceManagerCh chan diagnosisv1.Abnormal,
	informationManagerCh chan diagnosisv1.Abnormal,
) SourceManager {
	return &sourceManagerImpl{
		Context:              ctx,
		Client:               cli,
		Log:                  log,
		Scheme:               scheme,
		Cache:                cache,
		NodeName:             nodeName,
		sourceManagerCh:      sourceManagerCh,
		informationManagerCh: informationManagerCh,
	}
}

// Run runs the source manager.
func (sm *sourceManagerImpl) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !sm.Cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in source manager channel.
		case abnormal := <-sm.sourceManagerCh:
			if util.IsAbnormalNodeNameMatched(abnormal, sm.NodeName) {
				abnormal, err := sm.SyncAbnormal(abnormal)
				if err != nil {
					sm.Log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				sm.Log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop source manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncAbnormal syncs abnormals.
func (sm *sourceManagerImpl) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	sm.Log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal, err := sm.sendAbnormalToInformationManager(abnormal)
	if err != nil {
		sm.addAbnormalToSourceManagerQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// sendAbnormalToInformationManager sends Abnormal to information manager.
func (sm *sourceManagerImpl) sendAbnormalToInformationManager(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	sm.Log.Info("sending Abnormal to information manager", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.StartTime = metav1.Now()
	abnormal.Status.Phase = diagnosisv1.InformationCollecting
	if err := sm.Client.Status().Update(sm.Context, &abnormal); err != nil {
		sm.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToSourceManagerQueue adds Abnormal to the queue processed by source manager.
func (sm *sourceManagerImpl) addAbnormalToSourceManagerQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(sm.Context, sm.informationManagerCh, abnormal)
	if err != nil {
		sm.Log.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToSourceManagerQueueWithTimer adds Abnormal to the queue processed by source manager with a timer.
func (sm *sourceManagerImpl) addAbnormalToSourceManagerQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(sm.Context, 30*time.Second, sm.informationManagerCh, abnormal)
	if err != nil {
		sm.Log.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
