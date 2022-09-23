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

package garbagecollection

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	commoneventv1 "github.com/kubediag/kubediag/api/v1"
)

var (
	commonEventGarbageCollectionCycleCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "common_event_garbage_collection_cycle_count",
			Help: "Counter of common event garbage collection cycle",
		},
	)
	commonEventGarbageCollectionSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "common_event_garbage_collection_success_count",
			Help: "Counter of successful common event garbage collections",
		},
	)
	commonEventGarbageCollectionErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "common_event_garbage_collection_error_count",
			Help: "Counter of erroneous common event garbage collections",
		},
	)
)

// CommonEventReaper manages garbage collections of finished common events.
type CommonEventReaper struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// commonEventTTL is amount of time to retain common events.
	commonEventTTL time.Duration
}

// NewCommonEventReaper creates a new CommonEventReaper.
func NewCommonEventReaper(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	scheme *runtime.Scheme,
	cache cache.Cache,
	commonEventTTL time.Duration,
) *CommonEventReaper {

	metrics.Registry.MustRegister(
		commonEventGarbageCollectionCycleCount,
		commonEventGarbageCollectionSuccessCount,
		commonEventGarbageCollectionErrorCount,
	)

	return &CommonEventReaper{
		Context:        ctx,
		Logger:         logger,
		client:         cli,
		scheme:         scheme,
		cache:          cache,
		commonEventTTL: commonEventTTL,
	}
}

// Run runs the common event reaper.
func (cr *CommonEventReaper) Run(ctx context.Context) {
	// Wait for all caches to sync before processing.
	if !cr.cache.WaitForCacheSync(ctx) {
		return
	}

	// The housekeeping interval of garbage collections is a quarter of commonEventTTL.
	housekeepingInterval := cr.commonEventTTL / 4
	go wait.Until(func() {
		cr.Info("running garbage collection")
		commonEventGarbageCollectionCycleCount.Inc()

		// Garbage collect common events on node.
		commonEvents, err := cr.listCommonEvents()
		if err != nil {
			commonEventGarbageCollectionErrorCount.Inc()
			cr.Error(err, "failed to list common events")
			return
		}

		reapedCommonEvents := make([]commoneventv1.CommonEvent, 0)
		now := time.Now()
		if len(commonEvents) != 0 {
			for _, commonEvent := range commonEvents {
				// Garbage collect the common event if its last updated time is before commonEventTTL.
				if commonEvent.Status.LastUpdateTime == nil {
					if now.Sub(commonEvent.CreationTimestamp.Time) > cr.commonEventTTL {
						reapedCommonEvents = append(reapedCommonEvents, commonEvent)
					}
				} else {
					if now.Sub(commonEvent.Status.LastUpdateTime.Time) > cr.commonEventTTL {
						reapedCommonEvents = append(reapedCommonEvents, commonEvent)
					}
				}
			}

			if len(reapedCommonEvents) > 0 {
				for _, commonEvent := range reapedCommonEvents {
					err := cr.client.Delete(cr, &commonEvent)
					if err != nil {
						commonEventGarbageCollectionErrorCount.Inc()
						cr.Error(err, "failed to delete common event", "commonevent", client.ObjectKey{
							Name:      commonEvent.Name,
							Namespace: commonEvent.Namespace,
						})
						continue
					} else {
						commonEventGarbageCollectionSuccessCount.Inc()
					}
				}

				cr.Info("common events has been garbage collected", "time", now, "count", len(reapedCommonEvents))
			}
		}
	}, housekeepingInterval, ctx.Done())
}

// listCommonEvents lists CommonEvents from cache.
func (cr *CommonEventReaper) listCommonEvents() ([]commoneventv1.CommonEvent, error) {
	var commonEventList commoneventv1.CommonEventList
	if err := cr.cache.List(cr, &commonEventList); err != nil {
		return nil, err
	}

	return commonEventList.Items, nil
}
