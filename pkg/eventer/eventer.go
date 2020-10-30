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

package eventer

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// Eventer generates abnormals from kubernetes events.
type Eventer interface {
	// Run runs the Eventer.
	Run(<-chan struct{})
}

// eventer manages kubernetes events.
type eventer struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// eventChainCh is a channel for queuing Events to be processed by eventer.
	eventChainCh chan corev1.Event
	// sourceManagerCh is a channel for queuing Abnormals to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Abnormal
}

// NewEventer creates a new Eventer.
func NewEventer(
	ctx context.Context,
	logger logr.Logger,
	eventChainCh chan corev1.Event,
	sourceManagerCh chan diagnosisv1.Abnormal,
) Eventer {
	return &eventer{
		Context:         ctx,
		Logger:          logger,
		eventChainCh:    eventChainCh,
		sourceManagerCh: sourceManagerCh,
	}
}

// Run runs the eventer.
func (ev *eventer) Run(stopCh <-chan struct{}) {
	for {
		select {
		// Process events queuing in event channel.
		case event := <-ev.eventChainCh:
			// Create abnormal according to the kubernetes event.
			name := fmt.Sprintf("%s.%s.%s", util.KubernetesEventGeneratedAbnormalPrefix, event.Namespace, event.Name)
			namespace := util.DefautlNamespace
			abnormal := diagnosisv1.Abnormal{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: diagnosisv1.AbnormalSpec{
					Source:          diagnosisv1.KubernetesEventSource,
					KubernetesEvent: &event,
				},
			}

			// Add abnormal to the queue processed by source manager.
			err := util.QueueAbnormal(ev, ev.sourceManagerCh, abnormal)
			if err != nil {
				ev.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
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
