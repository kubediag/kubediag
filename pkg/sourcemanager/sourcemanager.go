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
	Run() error
	SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
}

// sourceManagerImpl implements SourceManager interface.
type sourceManagerImpl struct {
	// Client knows how to perform CRUD operations on Kubernetes objects.
	client.Client
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
	// Channel for notifying stop signal.
	stopCh chan struct{}
}

// NewSourceManager creates a new SourceManager.
func NewSourceManager(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	sourceManagerCh chan diagnosisv1.Abnormal,
	informationManagerCh chan diagnosisv1.Abnormal,
	stopCh chan struct{},
) SourceManager {
	return &sourceManagerImpl{
		Client:               cli,
		Log:                  log,
		Scheme:               scheme,
		Cache:                cache,
		NodeName:             nodeName,
		sourceManagerCh:      sourceManagerCh,
		informationManagerCh: informationManagerCh,
		stopCh:               stopCh,
	}
}

// Run runs the source manager.
func (sm *sourceManagerImpl) Run() error {
	ctx := context.Background()
	log := sm.Log.WithValues("component", "sourcemanager")

	// Wait for all caches to sync before processing.
	if !sm.Cache.WaitForCacheSync(sm.stopCh) {
		return fmt.Errorf("falied to sync cache")
	}

	// Process abnormals queuing in source manager channel.
	for abnormal := range sm.sourceManagerCh {
		if util.IsAbnormalNodeNameMatched(abnormal, sm.NodeName) {
			abnormal, err := sm.SyncAbnormal(ctx, log, abnormal)
			if err != nil {
				log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
			}

			log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
		}
	}

	return nil
}

// SyncAbnormal syncs abnormals.
func (sm *sourceManagerImpl) SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal, err := sm.sendAbnormalToInformationManager(ctx, log, abnormal)
	if err != nil {
		sm.addAbnormalToSourceManagerQueue(ctx, log, abnormal)
		return abnormal, err
	}

	return abnormal, nil
}

// sendAbnormalToInformationManager sends Abnormal to information manager.
func (sm *sourceManagerImpl) sendAbnormalToInformationManager(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("sending Abnormal to information manager", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.StartTime = metav1.Now()
	abnormal.Status.Phase = diagnosisv1.InformationCollecting
	if err := sm.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToSourceManagerQueue adds Abnormal to the queue processed by source manager.
func (sm *sourceManagerImpl) addAbnormalToSourceManagerQueue(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(ctx, sm.informationManagerCh, abnormal)
	if err != nil {
		log.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToSourceManagerQueueWithTimer adds Abnormal to the queue processed by source manager with a timer.
func (sm *sourceManagerImpl) addAbnormalToSourceManagerQueueWithTimer(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(ctx, 30*time.Second, sm.informationManagerCh, abnormal)
	if err != nil {
		log.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
