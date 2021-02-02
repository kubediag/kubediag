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
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

var (
	eventReceivedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "event_received_count",
			Help: "Counter of events received by eventer",
		},
	)
	eventerDiagnosisGenerationSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventer_diagnosis_generation_success_count",
			Help: "Counter of successful diagnosis generations by eventer",
		},
	)
	eventerDiagnosisGenerationErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventer_diagnosis_generation_error_count",
			Help: "Counter of erroneous diagnosis generations by eventer",
		},
	)
)

// Eventer generates diagnoses from kubernetes events.
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
	// sourceManagerCh is a channel for queuing Diagnoses to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Diagnosis
	// eventerEnabled indicates whether eventer is enabled.
	eventerEnabled bool
}

// NewEventer creates a new Eventer.
func NewEventer(
	ctx context.Context,
	logger logr.Logger,
	eventChainCh chan corev1.Event,
	sourceManagerCh chan diagnosisv1.Diagnosis,
	eventerEnabled bool,
) Eventer {
	metrics.Registry.MustRegister(
		eventReceivedCount,
		eventerDiagnosisGenerationSuccessCount,
		eventerDiagnosisGenerationErrorCount,
	)

	return &eventer{
		Context:         ctx,
		Logger:          logger,
		eventChainCh:    eventChainCh,
		sourceManagerCh: sourceManagerCh,
		eventerEnabled:  eventerEnabled,
	}
}

// Run runs the eventer.
func (ev *eventer) Run(stopCh <-chan struct{}) {
	if !ev.eventerEnabled {
		return
	}

	for {
		select {
		// Process events queuing in event channel.
		case event := <-ev.eventChainCh:
			eventReceivedCount.Inc()

			// Create diagnosis according to the kubernetes event.
			name := fmt.Sprintf("%s.%s.%s", util.KubernetesEventGeneratedDiagnosisPrefix, event.Namespace, event.Name)
			namespace := util.DefautlNamespace
			diagnosis := diagnosisv1.Diagnosis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: diagnosisv1.DiagnosisSpec{
					Source:          diagnosisv1.KubernetesEventSource,
					KubernetesEvent: &event,
				},
			}

			// Add diagnosis to the queue processed by source manager.
			err := util.QueueDiagnosis(ev, ev.sourceManagerCh, diagnosis)
			if err != nil {
				eventerDiagnosisGenerationErrorCount.Inc()
				ev.Error(err, "failed to send diagnosis to source manager queue", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			}

			// Increment counter of successful diagnosis generations by eventer.
			eventerDiagnosisGenerationSuccessCount.Inc()
		// Stop source manager on stop signal.
		case <-stopCh:
			return
		}
	}
}
