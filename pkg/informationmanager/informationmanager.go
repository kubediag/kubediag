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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// InformationManager manages information collectors in the system.
type InformationManager interface {
	Run(<-chan struct{})
	ListInformationCollectors() ([]diagnosisv1.InformationCollector, error)
	SyncAbnormal(diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
	Handler(http.ResponseWriter, *http.Request)
}

// informationManagerImpl implements InformationManager interface.
type informationManagerImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Client knows how to perform CRUD operations on Kubernetes objects.
	Client client.Client
	// Log represents the ability to log messages.
	Log logr.Logger
	// EventRecorder knows how to record events on behalf of an EventSource.
	EventRecorder record.EventRecorder
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
	ctx context.Context,
	cli client.Client,
	log logr.Logger,
	eventRecorder record.EventRecorder,
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
		Context:              ctx,
		Client:               cli,
		Log:                  log,
		EventRecorder:        eventRecorder,
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
	// Wait for all caches to sync before processing.
	if !im.Cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process abnormals queuing in information manager channel.
		case abnormal := <-im.informationManagerCh:
			if util.IsAbnormalNodeNameMatched(abnormal, im.NodeName) {
				abnormal, err := im.SyncAbnormal(abnormal)
				if err != nil {
					im.Log.Error(err, "failed to sync Abnormal", "abnormal", abnormal)
				}

				im.Log.Info("syncing Abnormal successfully", "abnormal", client.ObjectKey{
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
func (im *informationManagerImpl) ListInformationCollectors() ([]diagnosisv1.InformationCollector, error) {
	im.Log.Info("listing InformationCollectors")

	var informationCollectorList diagnosisv1.InformationCollectorList
	if err := im.Cache.List(im.Context, &informationCollectorList); err != nil {
		return nil, err
	}

	return informationCollectorList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (im *informationManagerImpl) SyncAbnormal(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	_, condition := util.GetAbnormalCondition(&abnormal.Status, diagnosisv1.InformationCollected)
	if condition != nil {
		im.Log.Info("ignoring Abnormal in phase InformationCollecting with condition InformationCollected", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	} else {
		informationCollectors, err := im.ListInformationCollectors()
		if err != nil {
			im.Log.Error(err, "failed to list InformationCollectors")
			im.addAbnormalToInformationManagerQueue(abnormal)
			return abnormal, err
		}

		abnormal, err := im.runInformationCollection(informationCollectors, abnormal)
		if err != nil {
			im.Log.Error(err, "failed to run collection")
			im.addAbnormalToInformationManagerQueue(abnormal)
			return abnormal, err
		}
	}

	return abnormal, nil
}

// Handler handles http requests and response with information collectors.
func (im *informationManagerImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		informationCollectors, err := im.ListInformationCollectors()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list information collectors: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(informationCollectors)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal information collectors: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// runInformationCollection collects information from information collectors.
func (im *informationManagerImpl) runInformationCollection(informationCollectors []diagnosisv1.InformationCollector, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	// Skip collection if SkipInformationCollection is true.
	if abnormal.Spec.SkipInformationCollection {
		im.Log.Info("skipping collection", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})

		im.EventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "SkippingCollection", "Skipping collection")

		abnormal, err := im.sendAbnormalToDiagnoserChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	informationCollected := false
	for _, collector := range informationCollectors {
		// Execute only matched information collectors if AssignedInformationCollectors is not empty.
		matched := false
		if len(abnormal.Spec.AssignedInformationCollectors) == 0 {
			matched = true
		} else {
			for _, assignedCollector := range abnormal.Spec.AssignedInformationCollectors {
				if collector.Name == assignedCollector.Name && collector.Namespace == assignedCollector.Namespace {
					im.Log.Info("assigned collector matched", "collector", client.ObjectKey{
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

		im.Log.Info("running collection", "collector", client.ObjectKey{
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
		result, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, im.Log)
		if err != nil {
			im.Log.Error(err, "failed to do http request to collector", "collector", client.ObjectKey{
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
			im.Log.Error(err, "invalid result from collector", "collector", client.ObjectKey{
				Name:      collector.Name,
				Namespace: collector.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		informationCollected = true
		abnormal.Status = result.Status

		im.EventRecorder.Eventf(&abnormal, corev1.EventTypeNormal, "InformationCollected", "Information collected by %s/%s", collector.Namespace, collector.Name)
	}

	// All assigned information collectors will be executed. The Abnormal will be sent to diagnoser chain
	// if any information is collected successfully.
	if informationCollected {
		abnormal, err := im.sendAbnormalToDiagnoserChain(abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	abnormal, err := im.setAbnormalFailed(abnormal)
	if err != nil {
		return abnormal, err
	}

	im.EventRecorder.Eventf(&abnormal, corev1.EventTypeWarning, "FailedCollect", "Unable to collect information for abnormal %s(%s)", abnormal.Name, abnormal.UID)

	return abnormal, nil
}

// sendAbnormalToDiagnoserChain sends Abnormal to diagnoser chain.
func (im *informationManagerImpl) sendAbnormalToDiagnoserChain(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Log.Info("sending Abnormal to diagnoser chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalDiagnosing
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionTrue,
	})
	if err := im.Client.Status().Update(im.Context, &abnormal); err != nil {
		im.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (im *informationManagerImpl) setAbnormalFailed(abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	im.Log.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.InformationCollected,
		Status: corev1.ConditionFalse,
	})
	if err := im.Client.Status().Update(im.Context, &abnormal); err != nil {
		im.Log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToInformationManagerQueue adds Abnormal to the queue processed by information manager.
func (im *informationManagerImpl) addAbnormalToInformationManagerQueue(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormal(im.Context, im.informationManagerCh, abnormal)
	if err != nil {
		im.Log.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}

// addAbnormalToInformationManagerQueueWithTimer adds Abnormal to the queue processed by information manager with a timer.
func (im *informationManagerImpl) addAbnormalToInformationManagerQueueWithTimer(abnormal diagnosisv1.Abnormal) {
	err := util.QueueAbnormalWithTimer(im.Context, 30*time.Second, im.informationManagerCh, abnormal)
	if err != nil {
		im.Log.Error(err, "failed to send abnormal to information manager queue", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
	}
}
