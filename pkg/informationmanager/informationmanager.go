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

package informationmanager

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// InformationManager manages information collectors in the system.
type InformationManager interface {
	Run(<-chan struct{})
	ListInformationCollectors(context.Context, logr.Logger) ([]diagnosisv1.InformationCollector, error)
	SyncAbnormal(context.Context, logr.Logger, diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
}

// informationManagerImpl implements InformationManager interface.
type informationManagerImpl struct {
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

	// Transport for sending http requests to information collectors.
	transport *http.Transport
	// Channel for queuing Abnormals to be processed by information manager.
	informationManagerCh chan diagnosisv1.Abnormal
	// Channel for queuing Abnormals to be processed by diagnoser chain.
	diagnoserChainCh chan diagnosisv1.Abnormal
}

// NewInformationManager creates a new InformationManager.
func NewInformationManager(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	informationManagerCh chan diagnosisv1.Abnormal,
	diagnoserChainCh chan diagnosisv1.Abnormal,
) InformationManager {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &informationManagerImpl{
		Client:               cli,
		Log:                  log,
		Scheme:               scheme,
		Cache:                cache,
		NodeName:             nodeName,
		transport:            transport,
		informationManagerCh: informationManagerCh,
		diagnoserChainCh:     diagnoserChainCh,
	}
}

// Run runs the information manager.
func (im *informationManagerImpl) Run(stopCh <-chan struct{}) {
	ctx := context.Background()
	log := im.Log.WithValues("component", "informationmanager")

	// Wait for all caches to sync before processing.
	if !im.Cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in information manager channel.
		case abnormal := <-im.informationManagerCh:
			if util.IsAbnormalNodeNameMatched(abnormal, im.NodeName) {
				abnormal, err := im.SyncAbnormal(ctx, log, abnormal)
				if err != nil {
					log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}
		// Stop information manager on stop signal.
		case <-stopCh:
			return
		}
	}
}

// ListInformationCollectors lists InformationCollectors from cache.
func (im *informationManagerImpl) ListInformationCollectors(ctx context.Context, log logr.Logger) ([]diagnosisv1.InformationCollector, error) {
	log.Info("listing InformationCollectors")

	var informationCollectorList diagnosisv1.InformationCollectorList
	if err := im.Cache.List(ctx, &informationCollectorList); err != nil {
		return nil, err
	}

	return informationCollectorList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (im *informationManagerImpl) SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	_, condition := util.GetAbnormalCondition(&abnormal.Status, diagnosisv1.InformationCollected)
	if condition != nil {
		log.Info("ignoring Abnormal in phase InformationCollecting with condition InformationCollected", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	} else {
		informationCollectors, err := im.ListInformationCollectors(ctx, log)
		if err != nil {
			log.Error(err, "failed to list InformationCollectors")
			im.addAbnormalToInformationManagerQueue(ctx, log, abnormal)
			return abnormal, err
		}

		abnormal, err := im.runInformationCollection(ctx, log, informationCollectors, abnormal)
		if err != nil {
			log.Error(err, "failed to run collection")
			im.addAbnormalToInformationManagerQueue(ctx, log, abnormal)
			return abnormal, err
		}
	}

	return abnormal, nil
}

// runInformationCollection collects information from information collectors.
func (im *informationManagerImpl) runInformationCollection(ctx context.Context, log logr.Logger, informationCollectors []diagnosisv1.InformationCollector, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Skip collection if SkipInformationCollection is true.
	if abnormal.Spec.SkipInformationCollection {
		log.Info("skipping collection", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		abnormal, err := im.sendAbnormalToDiagnoserChain(ctx, log, abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, collector := range informationCollectors {
		// Execute only matched information collectors if AssignedInformationCollectors is not empty.
		matched := false
		if len(abnormal.Spec.AssignedInformationCollectors) == 0 {
			matched = true
		} else {
			for _, assignedCollector := range abnormal.Spec.AssignedInformationCollectors {
				if collector.Name == assignedCollector.Name && collector.Namespace == assignedCollector.Namespace {
					log.Info("assigned collector matched", "collector", client.ObjectKey{
						Name:      collector.Name,
						Namespace: collector.Namespace,
					}, "abnormal", client.ObjectKey{
						Name:      abnormal.Name,
						Namespace: abnormal.Namespace,
					})
					matched = true
					break
				}
			}
		}

		if !matched {
			continue
		}

		log.Info("running collection", "collector", client.ObjectKey{
			Name:      collector.Name,
			Namespace: collector.Namespace,
		}, "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		scheme := strings.ToLower(string(collector.Spec.Scheme))
		host := collector.Spec.IP
		port := collector.Spec.Port
		path := collector.Spec.Path
		url := util.FormatURL(scheme, host, port, path)
		timeout := time.Duration(collector.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: im.transport,
		}

		// Send http request to the information collector with payload of abnormal.
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, log)
		if err != nil {
			log.Error(err, "failed to do http request to collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		// Validate an abnormal after processed by an information collector.
		err = util.ValidateAbnormalResult(result, abnormal)
		if err != nil {
			log.Error(err, "invalid result from collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		abnormal.Status = result.Status
		abnormal, err := im.sendAbnormalToDiagnoserChain(ctx, log, abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	abnormal, err := im.setAbnormalFailed(ctx, log, abnormal)
	if err != nil {
		return abnormal, err
	}

	return abnormal, nil
}

// sendAbnormalToDiagnoserChain sends Abnormal to diagnoser chain.
func (im *informationManagerImpl) sendAbnormalToDiagnoserChain(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("sending Abnormal to diagnoser chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalDiagnosing
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionTrue,
	})
	if err := im.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (im *informationManagerImpl) setAbnormalFailed(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionFalse,
	})
	if err := im.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToInformationManagerQueue adds Abnormal to the queue processed by information manager.
func (im *informationManagerImpl) addAbnormalToInformationManagerQueue(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(ctx, im.informationManagerCh, abnormal)
	if err != nil {
		log.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToInformationManagerQueueWithTimer adds Abnormal to the queue processed by information manager with a timer.
func (im *informationManagerImpl) addAbnormalToInformationManagerQueueWithTimer(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(ctx, 30*time.Second, im.informationManagerCh, abnormal)
	if err != nil {
		log.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
