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
	GetAbnormal(ctx context.Context, log logr.Logger, namespace string, name string) (diagnosisv1.Abnormal, error)
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

// GetAbnormal gets an Abnormal from apiserver.
func (sm *sourceManagerImpl) GetAbnormal(ctx context.Context, log logr.Logger, namespace string, name string) (diagnosisv1.Abnormal, error) {
	var abnormal diagnosisv1.Abnormal
	if err := sm.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &abnormal); err != nil {
		return diagnosisv1.Abnormal{}, err
	}

	return abnormal, nil
}

// SyncAbnormal syncs abnormals.
func (sm *sourceManagerImpl) SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal, err := sm.GetAbnormal(ctx, log, abnormal.Namespace, abnormal.Name)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			sm.addAbnormalToSourceManagerQueue(abnormal)
			return abnormal, err
		}

		return abnormal, nil
	}

	switch abnormal.Status.Phase {
	case diagnosisv1.InformationCollecting:
		log.Info("ignoring Abnormal in phase InformationCollecting", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	case diagnosisv1.AbnormalDiagnosing:
		log.Info("ignoring Abnormal in phase Diagnosing", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	case diagnosisv1.AbnormalRecovering:
		log.Info("ignoring Abnormal in phase Recovering", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	case diagnosisv1.AbnormalSucceeded:
		log.Info("ignoring Abnormal in phase Succeeded", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	case diagnosisv1.AbnormalFailed:
		log.Info("ignoring Abnormal in phase Failed", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	case diagnosisv1.AbnormalUnknown:
		log.Info("ignoring Abnormal in phase Unknown", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	default:
		abnormal, err := sm.sendAbnormalToInformationManager(ctx, log, abnormal)
		if err != nil {
			sm.addAbnormalToSourceManagerQueue(abnormal)
			return abnormal, err
		}

		return abnormal, nil
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

	sm.addAbnormalToInformationManagerQueue(abnormal)
	return abnormal, nil
}

// addAbnormalToSourceManagerQueue adds Abnormal to the queue processed by source manager.
func (sm *sourceManagerImpl) addAbnormalToSourceManagerQueue(abnormal diagnosisv1.Abnormal) {
	sm.sourceManagerCh <- abnormal
}

// addAbnormalToInformationManagerQueue adds Abnormal to the queue processed by information manager.
func (sm *sourceManagerImpl) addAbnormalToInformationManagerQueue(abnormal diagnosisv1.Abnormal) {
	sm.informationManagerCh <- abnormal
}
