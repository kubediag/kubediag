/*
Copyright 2022 The KubeDiag Authors.

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

package pagerdutyeventer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	kafkago "github.com/segmentio/kafka-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	offset64      uint64 = 14695981039346656037
	prime64       uint64 = 1099511628211
	separatorByte byte   = 255
)

var (
	pagerdutyEventReceivedCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pagerduty_event_received_count",
			Help: "Counter of pagerduty event received",
		},
	)
	pagerdutyEventProcessSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pagerduty_event_process_success_count",
			Help: "Counter of successful pagerduty event process",
		},
	)
	pagerdutyEventProcessErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "pagerduty_event_process_error_count",
			Help: "Counter of erroneous pagerduty event process",
		},
	)
)

// PagerDutyEventer can handle valid pagerduty events.
type PagerDutyEventer interface {
	// Handler handles http requests.
	Handler(http.ResponseWriter, *http.Request)
}

// pagerdutyEventer manages pagerduty event received by kubediag.
type pagerdutyEventer struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// sinkEventToKafka enables the pagerduty handler to write message to kafka cluster.
	sinkEventToKafka bool
	// kafkaAddress is the addresses used to connect to the kafka cluster.
	kafkaAddress string
	// pagerdutyEventerEnabled indicates whether pagerdutyEventer is enabled.
	pagerdutyEventerEnabled bool
}

// NewPagerDutyEventer creates a new PagerDutyEventer.
func NewPagerDutyEventer(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	cache cache.Cache,
	sinkEventToKafka bool,
	kafkaAddress string,
	pagerdutyEventerEnabled bool,
) PagerDutyEventer {
	metrics.Registry.MustRegister(
		pagerdutyEventReceivedCount,
	)

	return &pagerdutyEventer{
		Context:                 ctx,
		Logger:                  logger,
		client:                  cli,
		cache:                   cache,
		sinkEventToKafka:        sinkEventToKafka,
		kafkaAddress:            kafkaAddress,
		pagerdutyEventerEnabled: pagerdutyEventerEnabled,
	}
}

// Handler handles http requests for pagerduty events.
func (pe *pagerdutyEventer) Handler(w http.ResponseWriter, r *http.Request) {
	if !pe.pagerdutyEventerEnabled {
		http.Error(w, "pagerdutyEventer is not enabled", http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		pagerdutyEventReceivedCount.Inc()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			pagerdutyEventProcessErrorCount.Inc()
			pe.Error(err, "unable to read request body")
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Unmarshal request body into pagerduty message.
		var pagerDutyMessage PagerDutyMessage
		err = json.Unmarshal(body, &pagerDutyMessage)
		if err != nil {
			pagerdutyEventProcessErrorCount.Inc()
			pe.Error(err, "failed to unmarshal request body")
			http.Error(w, fmt.Sprintf("failed to unmarshal request body: %v", err), http.StatusInternalServerError)
			return
		}

		// Return 400 if the payload is nil.
		if pagerDutyMessage.Payload == nil {
			pagerdutyEventProcessErrorCount.Inc()
			pe.Error(fmt.Errorf("nil payload"), "invalid pagerduty message payload.", "body", r.Body)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		pagerdutyEvent := new(bytes.Buffer)
		err = json.NewEncoder(pagerdutyEvent).Encode(pagerDutyMessage.Payload)
		if err != nil {
			pagerdutyEventProcessErrorCount.Inc()
			pe.Error(err, "unable to encode pagerduty event.", "payload", pagerDutyMessage.Payload)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Store pagerduty event in the etcd of kubernetes.
		name := fmt.Sprintf("%s.%s.%d", strings.ToLower(pagerDutyMessage.Payload.Group), strings.ToLower(pagerDutyMessage.Payload.Class), commonEventToSignature(*pagerDutyMessage.Payload))
		namespace := util.DefautlNamespace
		now := metav1.Now()
		namespacedName := types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}

		labels := make(map[string]string)
		labels["source"] = pagerDutyMessage.Payload.Source
		labels["severity"] = pagerDutyMessage.Payload.Severity
		labels["class"] = pagerDutyMessage.Payload.Class
		labels["component"] = pagerDutyMessage.Payload.Component
		labels["group"] = pagerDutyMessage.Payload.Group

		var commonEvent diagnosisv1.CommonEvent
		if err := pe.client.Get(pe, namespacedName, &commonEvent); err != nil {
			if apierrors.IsNotFound(err) {
				commonEvent := diagnosisv1.CommonEvent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    labels,
					},
					Spec: diagnosisv1.CommonEventSpec{
						Summary:       pagerDutyMessage.Payload.Summary,
						Source:        pagerDutyMessage.Payload.Source,
						Severity:      pagerDutyMessage.Payload.Severity,
						Timestamp:     pagerDutyMessage.Payload.Timestamp,
						Class:         pagerDutyMessage.Payload.Class,
						Component:     pagerDutyMessage.Payload.Component,
						Group:         pagerDutyMessage.Payload.Group,
						CustomDetails: pagerDutyMessage.Payload.CustomDetails,
					},
				}

				err = pe.client.Create(pe, &commonEvent)
				if err != nil {
					pagerdutyEventProcessErrorCount.Inc()
					pe.Error(err, "unable to create Event")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			} else {
				pagerdutyEventProcessErrorCount.Inc()
				pe.Error(err, "unable to fetch Event")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
		} else {
			if commonEvent.Status.LastUpdateTime == nil || commonEvent.Status.LastUpdateTime.Before(&now) {
				commonEvent.Status.Count += 1
				commonEvent.Status.LastUpdateTime = &now
				if err := pe.client.Status().Update(pe, &commonEvent); err != nil {
					pe.Error(err, "unable to update Event")
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
			}
		}

		// Write the pagerduty message to kafka.
		if pe.sinkEventToKafka {
			topic := pagerDutyMessage.Payload.Group
			partition := 0
			err = pe.writePagerDutyMessageToKafka(topic, partition, pagerdutyEvent)
			if err != nil {
				pagerdutyEventProcessErrorCount.Inc()
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			pe.Info("sending pagerduty message to kafka successfully", "payload", pagerDutyMessage.Payload)
		}

		// Increment counter of successful pagerduty event process.
		pagerdutyEventProcessSuccessCount.Inc()

		w.Write([]byte("OK"))
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// writePagerDutyMessageToKafka writes a pagerduty message to kafka.
func (pe *pagerdutyEventer) writePagerDutyMessageToKafka(topic string, partition int, pagerdutyEvent *bytes.Buffer) error {
	conn, err := kafkago.DialLeader(context.Background(), "tcp", pe.kafkaAddress, topic, partition)
	if err != nil {
		pe.Error(err, "failed to connect kafka.", "address", pe.kafkaAddress, "topic", topic)
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.WriteMessages(
		kafkago.Message{Value: pagerdutyEvent.Bytes()},
	)
	if err != nil {
		pe.Error(err, "failed to write kafka messages.", "message", pagerdutyEvent.Bytes())
		return err
	}

	if err := conn.Close(); err != nil {
		pe.Error(err, "failed to close kafka writer.")
		return err
	}

	pe.Info("sending pagerduty message to kafka successfully", "message", pagerdutyEvent.Bytes())

	return nil
}

// commonEventToSignature returns a signature for a given common event.
func commonEventToSignature(payload PagerDutyPayload) uint64 {
	sum := hashNew()
	sum = hashAdd(sum, payload.Summary)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Source)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Severity)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Timestamp)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Class)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Component)
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, payload.Group)
	sum = hashAddByte(sum, separatorByte)

	customDetailNames := make([]string, 0, len(payload.CustomDetails))
	for customDetailName := range payload.CustomDetails {
		customDetailNames = append(customDetailNames, customDetailName)
	}
	sort.Strings(customDetailNames)
	for _, customDetailName := range customDetailNames {
		sum = hashAdd(sum, customDetailName)
		sum = hashAddByte(sum, separatorByte)
		sum = hashAdd(sum, payload.CustomDetails[customDetailName])
		sum = hashAddByte(sum, separatorByte)
	}

	return sum
}

// hashNew initializes a new fnv64a hash value.
func hashNew() uint64 {
	return offset64
}

// hashAdd adds a string to a fnv64a hash value, returning the updated hash.
func hashAdd(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// hashAddByte adds a byte to a fnv64a hash value, returning the updated hash.
func hashAddByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	h *= prime64
	return h
}
