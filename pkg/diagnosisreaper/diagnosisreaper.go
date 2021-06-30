/*
Copyright 2020 The KubeDiag Authors.

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

package diagnosisreaper

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

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/types"
	"github.com/kubediag/kubediag/pkg/util"
)

var (
	diagnosisGarbageCollectionCycleCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_garbage_collection_cycle_count",
			Help: "Counter of diagnosis garbage collection cycle",
		},
	)
	diagnosisGarbageCollectionSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_garbage_collection_success_count",
			Help: "Counter of successful diagnosis garbage collections",
		},
	)
	diagnosisGarbageCollectionErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "diagnosis_garbage_collection_error_count",
			Help: "Counter of erroneous diagnosis garbage collections",
		},
	)
)

// DiagnosisReaper manages garbage collections of finished diagnoses.
type DiagnosisReaper struct {
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
	// diagnosisTTL is amount of time to retain diagnoses.
	diagnosisTTL time.Duration
	// minimumDiagnosisTTLDuration is minimum age for a finished diagnosis before it is garbage collected.
	minimumDiagnosisTTLDuration time.Duration
	// maximumDiagnosesPerNode is maximum number of finished diagnoses to retain per node.
	maximumDiagnosesPerNode int32
	// dataRoot is root directory of persistent kubediag data.
	dataRoot string
}

// NewDiagnosisReaper creates a new DiagnosisReaper.
func NewDiagnosisReaper(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	diagnosisTTL time.Duration,
	minimumDiagnosisTTLDuration time.Duration,
	maximumDiagnosesPerNode int32,
	dataRoot string,
) *DiagnosisReaper {

	metrics.Registry.MustRegister(
		diagnosisGarbageCollectionCycleCount,
		diagnosisGarbageCollectionSuccessCount,
		diagnosisGarbageCollectionErrorCount,
	)

	return &DiagnosisReaper{
		Context:                     ctx,
		Logger:                      logger,
		client:                      cli,
		scheme:                      scheme,
		cache:                       cache,
		nodeName:                    nodeName,
		diagnosisTTL:                diagnosisTTL,
		minimumDiagnosisTTLDuration: minimumDiagnosisTTLDuration,
		maximumDiagnosesPerNode:     maximumDiagnosesPerNode,
		dataRoot:                    dataRoot,
	}
}

// Run runs the diagnosis reaper.
func (dr *DiagnosisReaper) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !dr.cache.WaitForCacheSync(stopCh) {
		return
	}

	// The housekeeping interval of garbage collections is a quarter of diagnosisTTL.
	housekeepingInterval := dr.diagnosisTTL / 4
	go wait.Until(func() {
		dr.Info("running garbage collection")
		diagnosisGarbageCollectionCycleCount.Inc()

		// Garbage collect diagnoses on node.
		diagnoses, err := dr.listDiagnoses()
		if err != nil {
			diagnosisGarbageCollectionErrorCount.Inc()
			dr.Error(err, "failed to list diagnoses")
			return
		}

		reapedDiagnoses := make([]diagnosisv1.Diagnosis, 0)
		retainedDiagnoses := make([]diagnosisv1.Diagnosis, 0)
		if len(diagnoses) != 0 {
			for _, diagnosis := range diagnoses {
				// Garbage collect the diagnosis if it is under any of the following conditions:
				//
				// Its age is longer than diagnosisTTL.
				// Its age is longer than minimumDiagnosisTTLDuration and its phase is Failed or Succeeded.
				if time.Now().Sub(diagnosis.Status.StartTime.Time) > dr.diagnosisTTL {
					reapedDiagnoses = append(reapedDiagnoses, diagnosis)
				} else if diagnosis.Status.Phase == diagnosisv1.DiagnosisFailed || diagnosis.Status.Phase == diagnosisv1.DiagnosisSucceeded {
					if time.Now().Sub(diagnosis.Status.StartTime.Time) > dr.minimumDiagnosisTTLDuration {
						reapedDiagnoses = append(reapedDiagnoses, diagnosis)
					}
				}

				retainedDiagnoses = append(retainedDiagnoses, diagnosis)
			}

			// Garbage collect old diagnoses if count of retained diagnoses is greater than maximumDiagnosesPerNode.
			if len(retainedDiagnoses) > int(dr.maximumDiagnosesPerNode) {
				sorted := types.SortedDiagnosisListByStartTime(retainedDiagnoses)
				sort.Sort(sorted)
				for i := 0; i < len(retainedDiagnoses)-int(dr.maximumDiagnosesPerNode); i++ {
					reapedDiagnoses = append(reapedDiagnoses, sorted[i])
				}
			}

			if len(reapedDiagnoses) > 0 {
				for _, diagnosis := range reapedDiagnoses {
					err := dr.client.Delete(dr, &diagnosis)
					if err != nil {
						diagnosisGarbageCollectionErrorCount.Inc()
						dr.Error(err, "failed to delete diagnosis", "diagnosis", client.ObjectKey{
							Name:      diagnosis.Name,
							Namespace: diagnosis.Namespace,
						})
						continue
					} else {
						diagnosisGarbageCollectionSuccessCount.Inc()
					}
				}

				dr.Info("diagnoses has been garbage collected", "time", time.Now(), "count", len(reapedDiagnoses))
			}
		}

		// Garbage collect java profiler data on node.
		absolutePath := filepath.Join(dr.dataRoot, "profilers/java/memory")
		err = DeleteExpiredProfilerData(absolutePath, dr.diagnosisTTL, dr)
		if err != nil {
			dr.Error(err, "failed to garbage collect java profiler data")
		}

		// Garbage collect go profiler data on node.
		absoluteGoProfilerPath := filepath.Join(dr.dataRoot, "profilers/go/pprof")
		err = DeleteExpiredProfilerData(absoluteGoProfilerPath, dr.diagnosisTTL, dr)
		if err != nil {
			dr.Error(err, "failed to garbage collect go profiler data")
		}

	}, housekeepingInterval, stopCh)
}

// listDiagnoses lists Diagnoses from cache.
func (dr *DiagnosisReaper) listDiagnoses() ([]diagnosisv1.Diagnosis, error) {
	var diagnosisList diagnosisv1.DiagnosisList
	if err := dr.cache.List(dr, &diagnosisList); err != nil {
		return nil, err
	}

	diagnosesOnNode := util.RetrieveDiagnosesOnNode(diagnosisList.Items, dr.nodeName)

	return diagnosesOnNode, nil
}

// DeleteExpiredProfilerData deletes profiler data by removing files or directories if the file age is longer
// than diagnosisTTL.
func DeleteExpiredProfilerData(path string, diagnosisTTL time.Duration, log logr.Logger) error {
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Garbage collect the profiler data if it is under any of the following conditions:
		//
		// Its age is longer than diagnosisTTL.
		if time.Now().Sub(entry.ModTime()) > diagnosisTTL {
			err := util.RemoveFile(filepath.Join(path, entry.Name()))
			if err != nil {
				log.Error(err, "unable to remove file")
			}
		}
	}

	return nil
}
