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

package abnormalreaper

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/types"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

var (
	abnormalGarbageCollectionSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "abnormal_garbage_collection_success_count",
			Help: "Counter of successful abnormal garbage collections",
		},
	)
	abnormalGarbageCollectionErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "abnormal_garbage_collection_error_count",
			Help: "Counter of erroneous abnormal garbage collections",
		},
	)
	nodeAbnormalCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "node_abnormal_count",
			Help: "Number of abnormals currently on node",
		},
	)
)

// AbnormalReaper manages garbage collections of finished abnormals.
type AbnormalReaper struct {
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
	// nodeName specifies the node name.
	nodeName string
	// abnormalTTL is amount of time to retain abnormals.
	abnormalTTL time.Duration
	// minimumAbnormalTTLDuration is minimum age for a finished abnormal before it is garbage collected.
	minimumAbnormalTTLDuration time.Duration
	// maximumAbnormalsPerNode is maximum number of finished abnormals to retain per node.
	maximumAbnormalsPerNode int32
	// dataRoot is root directory of persistent kube diagnoser data.
	dataRoot string
}

// NewAbnormalReaper creates a new AbnormalReaper.
func NewAbnormalReaper(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	abnormalTTL time.Duration,
	minimumAbnormalTTLDuration time.Duration,
	maximumAbnormalsPerNode int32,
	dataRoot string,
) *AbnormalReaper {
	metrics.Registry.MustRegister(
		abnormalGarbageCollectionSuccessCount,
		abnormalGarbageCollectionErrorCount,
		nodeAbnormalCount,
	)

	return &AbnormalReaper{
		Context:                    ctx,
		Logger:                     logger,
		client:                     cli,
		scheme:                     scheme,
		cache:                      cache,
		nodeName:                   nodeName,
		abnormalTTL:                abnormalTTL,
		minimumAbnormalTTLDuration: minimumAbnormalTTLDuration,
		maximumAbnormalsPerNode:    maximumAbnormalsPerNode,
		dataRoot:                   dataRoot,
	}
}

// Run runs the abnormal reaper.
func (ar *AbnormalReaper) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !ar.cache.WaitForCacheSync(stopCh) {
		return
	}

	// The housekeeping interval of garbage collections is a quarter of abnormalTTL.
	housekeepingInterval := ar.abnormalTTL / 4
	go wait.Until(func() {
		ar.Info("running garbage collection")

		// Garbage collect abnormals on node.
		abnormals, err := ar.listAbnormals()
		if err != nil {
			abnormalGarbageCollectionErrorCount.Inc()
			ar.Error(err, "failed to list abnormals")
			return
		}

		nodeAbnormalCount.Set(float64(len(abnormals)))

		reapedAbnormals := make([]diagnosisv1.Abnormal, 0)
		retainedAbnormals := make([]diagnosisv1.Abnormal, 0)
		if len(abnormals) != 0 {
			for _, abnormal := range abnormals {
				// Garbage collect the abnormal if it is under any of the following conditions:
				//
				// Its age is longer than abnormalTTL.
				// Its age is longer than minimumAbnormalTTLDuration and its phase is Failed or Succeeded.
				if time.Now().Sub(abnormal.Status.StartTime.Time) > ar.abnormalTTL {
					reapedAbnormals = append(reapedAbnormals, abnormal)
				} else if abnormal.Status.Phase == diagnosisv1.AbnormalFailed || abnormal.Status.Phase == diagnosisv1.AbnormalSucceeded {
					if time.Now().Sub(abnormal.Status.StartTime.Time) > ar.minimumAbnormalTTLDuration {
						reapedAbnormals = append(reapedAbnormals, abnormal)
					}
				}

				retainedAbnormals = append(retainedAbnormals, abnormal)
			}

			// Garbage collect old abnormals if count of retained abnormals is greater than maximumAbnormalsPerNode.
			if len(retainedAbnormals) > int(ar.maximumAbnormalsPerNode) {
				sorted := types.SortedAbnormalListByStartTime(retainedAbnormals)
				sort.Sort(sorted)
				for i := 0; i < len(retainedAbnormals)-int(ar.maximumAbnormalsPerNode); i++ {
					reapedAbnormals = append(reapedAbnormals, sorted[i])
				}
			}

			if len(reapedAbnormals) > 0 {
				for _, abnormal := range reapedAbnormals {
					err := ar.client.Delete(ar, &abnormal)
					if err != nil {
						abnormalGarbageCollectionErrorCount.Inc()
						ar.Error(err, "failed to delete abnormal", "abnormal", client.ObjectKey{
							Name:      abnormal.Name,
							Namespace: abnormal.Namespace,
						})
						continue
					} else {
						abnormalGarbageCollectionSuccessCount.Inc()
					}
				}

				ar.Info("abnormals has been garbage collected", "time", time.Now(), "count", len(reapedAbnormals))
			}
		}

		// Garbage collect java profiler data on node.
		absolutePath := filepath.Join(ar.dataRoot, "profilers/java/memory")
		err = DeleteExpiredProfilerData(absolutePath, ar.abnormalTTL, ar)
		if err != nil {
			ar.Error(err, "failed to garbage collect java profiler data")
		}
	}, housekeepingInterval, stopCh)
}

// listAbnormals lists Abnormals from cache.
func (ar *AbnormalReaper) listAbnormals() ([]diagnosisv1.Abnormal, error) {
	var abnormalList diagnosisv1.AbnormalList
	if err := ar.cache.List(ar, &abnormalList); err != nil {
		return nil, err
	}

	abnormalsOnNode := util.RetrieveAbnormalsOnNode(abnormalList.Items, ar.nodeName)

	return abnormalsOnNode, nil
}

// DeleteExpiredProfilerData deletes profiler data by removing files or directories if the file age is longer
// than abnormalTTL.
func DeleteExpiredProfilerData(path string, abnormalTTL time.Duration, log logr.Logger) error {
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Garbage collect the profiler data if it is under any of the following conditions:
		//
		// Its age is longer than abnormalTTL.
		if time.Now().Sub(entry.ModTime()) > abnormalTTL {
			err := util.RemoveFile(filepath.Join(path, entry.Name()))
			if err != nil {
				log.Error(err, "unable to remove file")
			}
		}
	}

	return nil
}
