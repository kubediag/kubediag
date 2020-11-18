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

package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

var (
	prometheusAlertReceivedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prometheus_alert_received_count",
			Help: "Counter of prometheus alerts received by alertmanager",
		},
	)
	alertmanagerAbnormalGenerationSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "alertmanager_abnormal_generation_success_count",
			Help: "Counter of successful abnormal generations by alertmanager",
		},
	)
	alertmanagerAbnormalGenerationErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "alertmanager_abnormal_generation_error_count",
			Help: "Counter of erroneous abnormal generations by alertmanager",
		},
	)
)

// Alertmanager can handle valid post alerts requests.
type Alertmanager interface {
	// Handler handles http requests.
	Handler(http.ResponseWriter, *http.Request)
}

// alertmanager manages prometheus alerts received by kube diagnoser.
type alertmanager struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// repeatInterval specifies how long to wait before sending a notification again if it has already
	// been sent successfully for an alert.
	repeatInterval time.Duration
	// firingAlertSet contains all alerts fired by alertmanager.
	firingAlertSet map[uint64]time.Time
	// sourceManagerCh is a channel for queuing Abnormals to be processed by source manager.
	sourceManagerCh chan diagnosisv1.Abnormal
	// alertmanagerEnabled indicates whether alertmanager is enabled.
	alertmanagerEnabled bool
}

// NewAlertmanager creates a new Alertmanager.
func NewAlertmanager(
	ctx context.Context,
	logger logr.Logger,
	repeatInterval time.Duration,
	sourceManagerCh chan diagnosisv1.Abnormal,
	alertmanagerEnabled bool,
) Alertmanager {
	metrics.Registry.MustRegister(
		prometheusAlertReceivedCount,
		alertmanagerAbnormalGenerationSuccessCount,
		alertmanagerAbnormalGenerationErrorCount,
	)

	firingAlertSet := make(map[uint64]time.Time)

	return &alertmanager{
		Context:             ctx,
		Logger:              logger,
		repeatInterval:      repeatInterval,
		firingAlertSet:      firingAlertSet,
		sourceManagerCh:     sourceManagerCh,
		alertmanagerEnabled: alertmanagerEnabled,
	}
}

// Handler handles http requests for sending prometheus alerts.
func (am *alertmanager) Handler(w http.ResponseWriter, r *http.Request) {
	if !am.alertmanagerEnabled {
		http.Error(w, fmt.Sprintf("alertmanager is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		prometheusAlertReceivedCount.Inc()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			alertmanagerAbnormalGenerationErrorCount.Inc()
			am.Error(err, "unable to read request body")
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var alerts []*types.Alert
		err = json.Unmarshal(body, &alerts)
		if err != nil {
			alertmanagerAbnormalGenerationErrorCount.Inc()
			am.Error(err, "failed to unmarshal request body")
			http.Error(w, fmt.Sprintf("failed to unmarshal request body: %v", err), http.StatusInternalServerError)
			return
		}

		for _, alert := range alerts {
			// Skip if the alert is resolved.
			if alert.Resolved() {
				continue
			}

			// Skip alerts if the repeat interval has not been passed.
			fingerprint := alert.Fingerprint()
			now := time.Now()
			lastFiring, ok := am.firingAlertSet[uint64(fingerprint)]
			if ok && lastFiring.After(now.Add(-am.repeatInterval)) {
				continue
			}

			// Create abnormal according to the prometheus alert.
			name := fmt.Sprintf("%s.%s.%s", util.PrometheusAlertGeneratedAbnormalPrefix, strings.ToLower(alert.Name()), alert.Fingerprint().String()[:7])
			namespace := util.DefautlNamespace
			prometheusAlert := diagnosisv1.PrometheusAlert{
				Labels:      alert.Labels,
				Annotations: alert.Annotations,
				StartsAt: metav1.Time{
					Time: alert.StartsAt,
				},
				EndsAt: metav1.Time{
					Time: alert.EndsAt,
				},
				GeneratorURL: alert.GeneratorURL,
			}
			abnormal := diagnosisv1.Abnormal{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: diagnosisv1.AbnormalSpec{
					Source:          diagnosisv1.PrometheusAlertSource,
					PrometheusAlert: &prometheusAlert,
				},
			}

			// Add abnormal to the queue processed by source manager.
			err := util.QueueAbnormal(am, am.sourceManagerCh, abnormal)
			if err != nil {
				alertmanagerAbnormalGenerationErrorCount.Inc()
				am.Error(err, "failed to send abnormal to source manager queue", "abnormal", client.ObjectKey{
					Name:      abnormal.Name,
					Namespace: abnormal.Namespace,
				})
			}

			// Update alert fired time if the abnormal is created successfully.
			am.firingAlertSet[uint64(fingerprint)] = now
		}

		// Increment counter of successful abnormal generations by alertmanager.
		alertmanagerAbnormalGenerationSuccessCount.Inc()

		w.Write([]byte("OK"))
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
