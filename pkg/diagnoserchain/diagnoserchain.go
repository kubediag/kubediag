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

package diagnoserchain

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"reflect"
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

// DiagnoserChain manages diagnoser in the system.
type DiagnoserChain interface {
	Run() error
	GetAbnormal(ctx context.Context, log logr.Logger, namespace string, name string) (diagnosisv1.Abnormal, error)
	ListDiagnosers(ctx context.Context, log logr.Logger) ([]diagnosisv1.Diagnoser, error)
	SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error)
}

// diagnoserChainImpl implements DiagnoserChain interface.
type diagnoserChainImpl struct {
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
	// Channel for queuing Abnormals to be processed by diagnoser chain.
	diagnoserChainCh chan diagnosisv1.Abnormal
	// Channel for queuing Abnormals to be processed by recoverer chain.
	recovererChainCh chan diagnosisv1.Abnormal
	// Channel for notifying stop signal.
	stopCh chan struct{}
}

// NewDiagnoserChain creates a new DiagnoserChain.
func NewDiagnoserChain(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	diagnoserChainCh chan diagnosisv1.Abnormal,
	recovererChainCh chan diagnosisv1.Abnormal,
	stopCh chan struct{},
) DiagnoserChain {
	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &diagnoserChainImpl{
		Client:           cli,
		Log:              log,
		Scheme:           scheme,
		Cache:            cache,
		NodeName:         nodeName,
		transport:        transport,
		diagnoserChainCh: diagnoserChainCh,
		recovererChainCh: recovererChainCh,
		stopCh:           stopCh,
	}
}

// Run runs the diagnoser chain.
func (dc *diagnoserChainImpl) Run() error {
	ctx := context.Background()
	log := dc.Log.WithValues("component", "diagnoserchain")

	// Wait for all caches to sync before processing.
	if !dc.Cache.WaitForCacheSync(dc.stopCh) {
		return fmt.Errorf("falied to sync cache")
	}

	// Process abnormals queuing in diagnoser chain channel.
	for abnormal := range dc.diagnoserChainCh {
		if util.IsAbnormalNodeNameMatched(abnormal, dc.NodeName) {
			abnormal, err := dc.SyncAbnormal(ctx, log, abnormal)
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
func (dc *diagnoserChainImpl) GetAbnormal(ctx context.Context, log logr.Logger, namespace string, name string) (diagnosisv1.Abnormal, error) {
	var abnormal diagnosisv1.Abnormal
	if err := dc.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &abnormal); err != nil {
		return diagnosisv1.Abnormal{}, err
	}

	return abnormal, nil
}

// ListDiagnosers lists Diagnosers from cache.
func (dc *diagnoserChainImpl) ListDiagnosers(ctx context.Context, log logr.Logger) ([]diagnosisv1.Diagnoser, error) {
	log.Info("listing Diagnosers")

	var diagnoserList diagnosisv1.DiagnoserList
	if err := dc.Cache.List(ctx, &diagnoserList); err != nil {
		return nil, err
	}

	return diagnoserList.Items, nil
}

// SyncAbnormal syncs abnormals.
func (dc *diagnoserChainImpl) SyncAbnormal(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("starting to sync Abnormal", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal, err := dc.GetAbnormal(ctx, log, abnormal.Namespace, abnormal.Name)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			dc.addAbnormalToDiagnoserChainQueue(abnormal)
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
		diagnosers, err := dc.ListDiagnosers(ctx, log)
		if err != nil {
			log.Error(err, "failed to list Diagnosers")
			dc.addAbnormalToDiagnoserChainQueue(abnormal)
			return abnormal, err
		}

		abnormal, err := dc.runDiagnosis(ctx, log, diagnosers, abnormal)
		if err != nil {
			log.Error(err, "failed to run diagnosis")
			dc.addAbnormalToDiagnoserChainQueue(abnormal)
			return abnormal, err
		}

		return abnormal, nil
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
	}

	return abnormal, nil
}

// runDiagnosis diagnoses an abnormal with diagnosers.
func (dc *diagnoserChainImpl) runDiagnosis(ctx context.Context, log logr.Logger, diagnosers []diagnosisv1.Diagnoser, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	deepCopy := *abnormal.DeepCopy()

	// Skip diagnosis if SkipDiagnosis is true.
	if abnormal.Spec.SkipDiagnosis {
		log.Info("skipping diagnosis", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		})
		abnormal, err := dc.sendAbnormalToRecovererChain(ctx, log, abnormal)
		if err != nil {
			return abnormal, err
		}

		return abnormal, nil
	}

	for _, diagnoser := range diagnosers {
		// Execute only matched diagnosers if AssignedDiagnosers is not empty.
		matched := false
		if len(abnormal.Spec.AssignedDiagnosers) == 0 {
			matched = true
		} else {
			for _, assignedDiagnoser := range abnormal.Spec.AssignedDiagnosers {
				if diagnoser.Name == assignedDiagnoser.Name && diagnoser.Namespace == assignedDiagnoser.Namespace {
					log.Info("assigned diagnoser matched", "diagnoser", client.ObjectKey{
						Name:      diagnoser.Name,
						Namespace: diagnoser.Namespace,
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

		scheme := strings.ToLower(string(diagnoser.Spec.Scheme))
		host := diagnoser.Spec.IP
		port := diagnoser.Spec.Port
		path := diagnoser.Spec.Path
		url := util.FormatURL(scheme, host, port, path)
		timeout := time.Duration(diagnoser.Spec.TimeoutSeconds) * time.Second

		cli := &http.Client{
			Timeout:   timeout,
			Transport: dc.transport,
		}

		// Send http request to the diagnosers with payload of abnormal.
		result, retry, err := util.DoHTTPRequestWithAbnormal(abnormal, url, *cli, log)
		if err != nil {
			log.Error(err, "failed to do http request to diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		// Validate an abnormal after processed by a diagnoser.
		err = util.ValidateAbnormalResult(result, abnormal)
		if err != nil {
			log.Error(err, "invalid result from diagnoser", "diagnoser", client.ObjectKey{
				Name:      diagnoser.Name,
				Namespace: diagnoser.Namespace,
			}, "abnormal", client.ObjectKey{
				Name:      abnormal.Name,
				Namespace: abnormal.Namespace,
			})
			continue
		}

		abnormal.Status = result.Status
		abnormal.Status.Diagnoser = diagnosisv1.NamespacedName{
			Name:      diagnoser.Name,
			Namespace: diagnoser.Namespace,
		}
		if retry {
			if reflect.DeepEqual(deepCopy, abnormal) {
				log.Info("skip updating abnormal for not being modified by diagnoser", "diagnoser", client.ObjectKey{
					Name:      diagnoser.Name,
					Namespace: diagnoser.Namespace,
				}, "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			} else {
				if err := dc.Status().Update(ctx, &abnormal); err != nil {
					log.Error(err, "unable to update Abnormal")
					return abnormal, err
				}
			}
			go util.QueueAbnormalWithTimer(ctx, abnormal, dc.addAbnormalToDiagnoserChainQueue)
		} else {
			abnormal, err := dc.sendAbnormalToRecovererChain(ctx, log, abnormal)
			if err != nil {
				return abnormal, err
			}
		}

		return abnormal, nil
	}

	abnormal, err := dc.setAbnormalFailed(ctx, log, abnormal)
	if err != nil {
		return abnormal, err
	}

	return abnormal, nil
}

// sendAbnormalToRecovererChain sends Abnormal to recoverer chain.
func (dc *diagnoserChainImpl) sendAbnormalToRecovererChain(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("sending Abnormal to recoverer chain", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalRecovering
	abnormal.Status.Identifiable = true
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionTrue,
	})
	if err := dc.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	dc.addAbnormalToRecovererChainQueue(abnormal)
	return abnormal, nil
}

// setAbnormalFailed sets abnormal phase to Failed.
func (dc *diagnoserChainImpl) setAbnormalFailed(ctx context.Context, log logr.Logger, abnormal diagnosisv1.Abnormal) (diagnosisv1.Abnormal, error) {
	log.Info("setting Abnormal phase to failed", "abnormal", client.ObjectKey{
		Name:      abnormal.Name,
		Namespace: abnormal.Namespace,
	})

	abnormal.Status.Phase = diagnosisv1.AbnormalFailed
	abnormal.Status.Identifiable = false
	util.UpdateAbnormalCondition(&abnormal.Status, &diagnosisv1.AbnormalCondition{
		Type:   diagnosisv1.AbnormalIdentified,
		Status: corev1.ConditionFalse,
	})
	if err := dc.Status().Update(ctx, &abnormal); err != nil {
		log.Error(err, "unable to update Abnormal")
		return abnormal, err
	}

	return abnormal, nil
}

// addAbnormalToDiagnoserChainQueue adds Abnormal to the queue processed by diagnoser chain.
func (dc *diagnoserChainImpl) addAbnormalToDiagnoserChainQueue(abnormal diagnosisv1.Abnormal) {
	dc.diagnoserChainCh <- abnormal
}

// addAbnormalToRecovererChainQueue adds Abnormal to the queue processed by recoverer chain.
func (dc *diagnoserChainImpl) addAbnormalToRecovererChainQueue(abnormal diagnosisv1.Abnormal) {
	dc.recovererChainCh <- abnormal
}
