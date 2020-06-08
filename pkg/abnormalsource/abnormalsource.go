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

package abnormalsource

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
)

// AbnormalSource manages abnormals in the system.
type AbnormalSource interface {
	Run() error
	ListAbnormals(ctx context.Context, log logr.Logger) ([]diagnosisv1.Abnormal, error)
	SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
}

// abnormalSourceImpl implements AbnormalSource interface.
type abnormalSourceImpl struct {
	// Client knows how to perform CRUD operations on Kubernetes objects.
	client.Client
	// Log represents the ability to log messages.
	Log logr.Logger
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme *runtime.Scheme
	// Cache knows how to load Kubernetes objects.
	Cache cache.Cache

	// Channel for queuing Abnormals to be processed by abnormal source.
	abnormalSourceCh chan diagnosisv1.Abnormal
	// Channel for queuing Abnormals to be processed by information manager.
	informationManagerCh chan diagnosisv1.Abnormal
	// Channel for notifying stop signal.
	stopCh chan struct{}
}

// NewAbnormalSource creates a new AbnormalSource.
func NewAbnormalSource(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	abnormalSourceCh chan diagnosisv1.Abnormal,
	informationManagerCh chan diagnosisv1.Abnormal,
	stopCh chan struct{},
) AbnormalSource {
	return &abnormalSourceImpl{
		Client:               cli,
		Log:                  log,
		Scheme:               scheme,
		Cache:                cache,
		abnormalSourceCh:     abnormalSourceCh,
		informationManagerCh: informationManagerCh,
		stopCh:               stopCh,
	}
}

// Run runs the abnormal source.
func (as *abnormalSourceImpl) Run() error {
	ctx := context.Background()
	log := as.Log.WithValues("component", "abnormalsource")

	// Wait for all caches to sync before processing.
	if !as.Cache.WaitForCacheSync(as.stopCh) {
		return fmt.Errorf("falied to sync cache")
	}

	// List abnormals on start.
	abnormals, err := as.ListAbnormals(ctx, log)
	if err != nil {
		log.Error(err, "failed to list Abnormals")
		return err
	}

	// Sync all abnormals on start.
	for _, abnormal := range abnormals {
		abnormal, err := as.SyncAbnormal(ctx, log, abnormal)
		if err != nil {
			log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
		}

		log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}

	// Process abnormals queuing in abnormal source channel.
	for abnormal := range as.abnormalSourceCh {
		abnormal, err := as.SyncAbnormal(ctx, log, abnormal)
		if err != nil {
			log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
		}

		log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}

	return nil
}

// ListAbnormals lists Abnormals from cache.
func (as *abnormalSourceImpl) ListAbnormals(ctx context.Context, log logr.Logger) ([]diagnosisv1.Abnormal, error) {
	log.Info("listing Abnormals")

	var abnormalList diagnosisv1.AbnormalList
	if err := as.Cache.List(ctx, &abnormalList); err != nil {
		return nil, err
	}

	return abnormalList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (as *abnormalSourceImpl) SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

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
		abnormal, err := as.SendAbnormalToInformationManager(ctx, log, abnormal)
		if err != nil {
			as.abnormalSourceCh <- abnormal
			return abnormal, err
		}

		return abnormal, nil
	}

	return abnormal, nil
}

// SendAbnormalToInformationManager sends Abnormal to information manager.
func (as *abnormalSourceImpl) SendAbnormalToInformationManager(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("sending Abnormal to information manager", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.StartTime = metav1.Now()
	abnormal.Status.Phase = diagnosisv1.InformationCollecting
	if err := as.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	as.informationManagerCh <- abnormal
	return abnormal, nil
}
