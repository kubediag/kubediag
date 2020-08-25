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
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// sourceManager manages abnormal sources in the system.
type sourceManager struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// eventRecorder knows how to record events on behalf of an EventSource.
	eventRecorder record.EventRecorder
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// nodeName specifies the node name.
	nodeName string
	// sourceManagerCh is a channel for queuing Abnormals to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Abnormal
	// informationManagerCh is a channel for queuing Abnormals to be processed by information manager.
	informationManagerCh chan diagnosisv1.Abnormal
}

// NewSourceManager creates a new sourceManager.
func NewSourceManager(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	sourceManagerCh chan diagnosisv1.Abnormal,
	informationManagerCh chan diagnosisv1.Abnormal,
) types.AbnormalManager {
	return &sourceManager{
		Context:              ctx,
		Logger:               logger,
		client:               cli,
		eventRecorder:        eventRecorder,
		scheme:               scheme,
		cache:                cache,
		nodeName:             nodeName,
		sourceManagerCh:      sourceManagerCh,
		informationManagerCh: informationManagerCh,
	}
}

// Run runs the source manager.
func (sm *sourceManager) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !sm.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in source manager channel.
		case abnormal := <-sm.sourceManagerCh:
			if util.IsAbnormalNodeNameMatched(abnormal, sm.nodeName) {
				abnormal, err := sm.SyncAbnormal(abnormal)
				if err != nil {
					sm.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				sm.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
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
func (sm *sourceManager) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	sm.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	sm.eventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "Accepted", "Accepted abnormal")

	abnormal, err := sm.sendAbnormalToInformationManager(abnormal)
	if err != nil {
		sm.addAbnormalToSourceManagerQueue(abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// Handler handles http requests.
func (sm *sourceManager) Handler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
}

// sendAbnormalToInformationManager sends Abnormal to information manager.
func (sm *sourceManager) sendAbnormalToInformationManager(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	sm.Info("sending Abnormal to information manager", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.StartTime = metav1.Now()
	abnormal.Status.Phase = diagnosisv1.InformationCollecting
	if err := sm.client.Status().Update(sm, &abnormal); err != nil {
		sm.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToSourceManagerQueue adds Abnormal to the queue processed by source manager.
func (sm *sourceManager) addAbnormalToSourceManagerQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(sm, sm.informationManagerCh, abnormal)
	if err != nil {
		sm.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToSourceManagerQueueWithTimer adds Abnormal to the queue processed by source manager with a timer.
func (sm *sourceManager) addAbnormalToSourceManagerQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(sm, 30*time.Second, sm.informationManagerCh, abnormal)
	if err != nil {
		sm.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
