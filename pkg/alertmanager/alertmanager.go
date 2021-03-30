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
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

var (
	prometheusAlertReceivedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "prometheus_alert_received_count",
			Help: "Counter of prometheus alerts received by alertmanager",
		},
	)
	alertmanagerDiagnosisGenerationSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "alertmanager_diagnosis_generation_success_count",
			Help: "Counter of successful diagnosis generations by alertmanager",
		},
	)
	alertmanagerDiagnosisGenerationErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "alertmanager_diagnosis_generation_error_count",
			Help: "Counter of erroneous diagnosis generations by alertmanager",
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

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// nodeName specifies the node name.
	nodeName string
	// repeatInterval specifies how long to wait before sending a notification again if it has already
	// been sent successfully for an alert.
	repeatInterval time.Duration
	// firingAlertSet contains all alerts fired by alertmanager.
	firingAlertSet map[uint64]time.Time
	// alertmanagerEnabled indicates whether alertmanager is enabled.
	alertmanagerEnabled bool
}

// NewAlertmanager creates a new Alertmanager.
func NewAlertmanager(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	cache cache.Cache,
	nodeName string,
	repeatInterval time.Duration,
	alertmanagerEnabled bool,
) Alertmanager {
	metrics.Registry.MustRegister(
		prometheusAlertReceivedCount,
		alertmanagerDiagnosisGenerationSuccessCount,
		alertmanagerDiagnosisGenerationErrorCount,
	)

	firingAlertSet := make(map[uint64]time.Time)

	return &alertmanager{
		Context:             ctx,
		Logger:              logger,
		client:              cli,
		cache:               cache,
		nodeName:            nodeName,
		repeatInterval:      repeatInterval,
		firingAlertSet:      firingAlertSet,
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
			alertmanagerDiagnosisGenerationErrorCount.Inc()
			am.Error(err, "unable to read request body")
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var alerts []*types.Alert
		err = json.Unmarshal(body, &alerts)
		if err != nil {
			alertmanagerDiagnosisGenerationErrorCount.Inc()
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

			triggers, err := am.listTriggers()
			if err != nil {
				am.Error(err, "failed to list Triggers")
				return
			}

			diagnosis, err := am.createDiagnosisFromPrometheusAlert(triggers, alert)
			if err != nil {
				return
			}

			am.Info("creating Diagnosis from prometheus alert successfully", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})

			// Update alert fired time if the diagnosis is created successfully.
			am.firingAlertSet[uint64(fingerprint)] = now
		}

		// Increment counter of successful diagnosis generations by alertmanager.
		alertmanagerDiagnosisGenerationSuccessCount.Inc()

		w.Write([]byte("OK"))
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// listTriggers lists Triggers from cache.
func (am *alertmanager) listTriggers() ([]diagnosisv1.Trigger, error) {
	var triggersList diagnosisv1.TriggerList
	if err := am.cache.List(am, &triggersList); err != nil {
		return nil, err
	}

	return triggersList.Items, nil
}

// createDiagnosisFromPrometheusAlert creates an Diagnosis from prometheus alert and triggers.
func (am *alertmanager) createDiagnosisFromPrometheusAlert(triggers []diagnosisv1.Trigger, alert *types.Alert) (diagnosisv1.Diagnosis, error) {
	for _, trigger := range triggers {
		sourceTemplate := trigger.Spec.SourceTemplate
		if sourceTemplate.PrometheusAlertTemplate != nil {
			// Set all fields of the diagnosis according to trigger if the prometheus alert contains
			// all match of the regular expression pattern defined in prometheus alert template.
			matched, err := matchPrometheusAlert(*sourceTemplate.PrometheusAlertTemplate, alert)
			if err != nil {
				am.Error(err, "failed to compare trigger template and prometheus alert")
				continue
			}

			if matched {
				am.Info("creating Diagnosis from prometheus alert", "alert", alert.String())

				// Create diagnosis according to the prometheus alert.
				name := fmt.Sprintf("%s.%s.%s", util.PrometheusAlertGeneratedDiagnosisPrefix, strings.ToLower(alert.Name()), alert.Fingerprint().String()[:7])
				namespace := util.DefautlNamespace
				annotations := make(map[string]string)
				annotations[util.PrometheusAlertAnnotation] = string(alert.String())
				diagnosis := diagnosisv1.Diagnosis{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Namespace:   namespace,
						Annotations: annotations,
					},
					Spec: diagnosisv1.DiagnosisSpec{
						OperationSet: trigger.Spec.OperationSet,
						NodeName:     string(alert.Labels[sourceTemplate.PrometheusAlertTemplate.NodeNameReferenceLabel]),
					},
				}

				if err := am.client.Create(am, &diagnosis); err != nil {
					if !apierrors.IsAlreadyExists(err) {
						am.Error(err, "unable to create Diagnosis")
						return diagnosis, err
					}
				}

				return diagnosis, nil
			}
		}
	}

	return diagnosisv1.Diagnosis{}, nil
}

// matchPrometheusAlert reports whether the diagnosis contains all match of the regular expression pattern
// defined in prometheus alert template.
func matchPrometheusAlert(prometheusAlertTemplate diagnosisv1.PrometheusAlertTemplate, alert *types.Alert) (bool, error) {
	re, err := regexp.Compile(prometheusAlertTemplate.Regexp.AlertName)
	if err != nil {
		return false, err
	}
	if !re.MatchString(string(alert.Labels[model.AlertNameLabel])) {
		return false, nil
	}

	// Template label key must be identical to the prometheus alert label key.
	// Template label value should be a regular expression.
	for templateKey, templateValue := range prometheusAlertTemplate.Regexp.Labels {
		value, ok := alert.Labels[templateKey]
		if !ok {
			return false, nil
		}

		re, err := regexp.Compile(string(templateValue))
		if err != nil {
			return false, err
		}
		if !re.MatchString(string(value)) {
			return false, nil
		}
	}

	// Template annotation key must be identical to the prometheus alert annotation key.
	// Template annotation value should be a regular expression.
	for templateKey, templateValue := range prometheusAlertTemplate.Regexp.Annotations {
		value, ok := alert.Annotations[templateKey]
		if !ok {
			return false, nil
		}

		re, err := regexp.Compile(string(templateValue))
		if err != nil {
			return false, err
		}
		if !re.MatchString(string(value)) {
			return false, nil
		}
	}

	re, err = regexp.Compile(prometheusAlertTemplate.Regexp.StartsAt)
	if err != nil {
		return false, err
	}
	if !re.MatchString(alert.StartsAt.String()) {
		return false, nil
	}

	re, err = regexp.Compile(prometheusAlertTemplate.Regexp.EndsAt)
	if err != nil {
		return false, err
	}
	if !re.MatchString(alert.EndsAt.String()) {
		return false, nil
	}

	re, err = regexp.Compile(prometheusAlertTemplate.Regexp.GeneratorURL)
	if err != nil {
		return false, err
	}
	if !re.MatchString(alert.GeneratorURL) {
		return false, nil
	}

	return true, nil
}
