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
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	kafkago "github.com/segmentio/kafka-go"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
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
			pe.Error(fmt.Errorf("nil payload"), "invalid pagerduty message payload.", "body", r.Body)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Write pagerduty message to kafka.
		topic := pagerDutyMessage.Payload.Group
		partition := 0
		conn, err := kafkago.DialLeader(context.Background(), "tcp", pe.kafkaAddress, topic, partition)
		if err != nil {
			pe.Error(err, "failed to connect kafka.", "address", pe.kafkaAddress, "topic", topic)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		pagerdutyEvent := new(bytes.Buffer)
		err = json.NewEncoder(pagerdutyEvent).Encode(pagerDutyMessage.Payload)
		if err != nil {
			pe.Error(err, "unable to encode pagerduty event.", "payload", pagerDutyMessage.Payload)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err = conn.WriteMessages(
			kafkago.Message{Value: pagerdutyEvent.Bytes()},
		)
		if err != nil {
			pe.Error(err, "failed to write kafka messages.", "payload", pagerDutyMessage.Payload)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if err := conn.Close(); err != nil {
			pe.Error(err, "failed to close kafka writer.")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		pe.Info("sending pagerduty message to kafka successfully", "payload", pagerDutyMessage.Payload)

		// Increment counter of successful pagerduty event process.
		pagerdutyEventProcessSuccessCount.Inc()

		w.Write([]byte("OK"))
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
