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

package garbagecollection

import (
	"context"
	"io/ioutil"
	"os"
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
	taskGarbageCollectionCycleCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "task_garbage_collection_cycle_count",
			Help: "Counter of task garbage collection cycle",
		},
	)
	taskGarbageCollectionSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "task_garbage_collection_success_count",
			Help: "Counter of successful task garbage collections",
		},
	)
	taskGarbageCollectionErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "task_garbage_collection_error_count",
			Help: "Counter of erroneous task garbage collections",
		},
	)
)

// TaskReaper manages garbage collections of finished diagnoses.
type TaskReaper struct {
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
	// taskTTL is amount of time to retain tasks.
	taskTTL time.Duration
	// minimumTaskTTLDuration is minimum age for a finished task before it is garbage collected.
	minimumTaskTTLDuration time.Duration
	// maximumTasksPerNode is maximum number of finished tasks to retain per node.
	maximumTasksPerNode int32
	// dataRoot is root directory of persistent kubediag data.
	dataRoot string
}

// NewTaskReaper creates a new TaskReaper.
func NewTaskReaper(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	taskTTL time.Duration,
	minimumTaskTTLDuration time.Duration,
	maximumTasksPerNode int32,
	dataRoot string,
) *TaskReaper {
	metrics.Registry.MustRegister(
		taskGarbageCollectionCycleCount,
		taskGarbageCollectionSuccessCount,
		taskGarbageCollectionErrorCount,
	)

	return &TaskReaper{
		Context:                ctx,
		Logger:                 logger,
		client:                 cli,
		scheme:                 scheme,
		cache:                  cache,
		nodeName:               nodeName,
		taskTTL:                taskTTL,
		minimumTaskTTLDuration: minimumTaskTTLDuration,
		maximumTasksPerNode:    maximumTasksPerNode,
		dataRoot:               dataRoot,
	}
}

// Run runs the task reaper.
func (tr *TaskReaper) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !tr.cache.WaitForCacheSync(stopCh) {
		return
	}

	// The housekeeping interval of garbage collections is a quarter of taskTTL.
	housekeepingInterval := tr.taskTTL / 4
	go wait.Until(func() {
		tr.Info("running garbage collection")
		taskGarbageCollectionCycleCount.Inc()

		// Garbage collect tasks on node.
		tasks, err := tr.listTasks()
		if err != nil {
			taskGarbageCollectionErrorCount.Inc()
			tr.Error(err, "failed to list tasks")
			return
		}

		reapedTasks := make([]diagnosisv1.Task, 0)
		retainedTasks := make([]diagnosisv1.Task, 0)
		if len(tasks) != 0 {
			for _, task := range tasks {
				// Garbage collect the task if it is under any of the following conditions:
				//
				// Its age is longer than taskTTL.
				// Its age is longer than minimumTaskTTLDuration and its phase is Failed or Succeeded.
				if time.Now().Sub(task.Status.StartTime.Time) > tr.taskTTL {
					reapedTasks = append(reapedTasks, task)
				} else if task.Status.Phase == diagnosisv1.TaskFailed || task.Status.Phase == diagnosisv1.TaskSucceeded {
					if time.Now().Sub(task.Status.StartTime.Time) > tr.minimumTaskTTLDuration {
						reapedTasks = append(reapedTasks, task)
					}
				}

				retainedTasks = append(retainedTasks, task)
			}

			// Garbage collect old tasks if count of retained tasks is greater than maximumTasksPerNode.
			if len(retainedTasks) > int(tr.maximumTasksPerNode) {
				sorted := types.SortedTaskListByStartTime(retainedTasks)
				sort.Sort(sorted)
				for i := 0; i < len(retainedTasks)-int(tr.maximumTasksPerNode); i++ {
					reapedTasks = append(reapedTasks, sorted[i])
				}
			}

			if len(reapedTasks) > 0 {
				for _, task := range reapedTasks {
					err := tr.client.Delete(tr, &task)
					if err != nil {
						taskGarbageCollectionErrorCount.Inc()
						tr.Error(err, "failed to delete task", "task", client.ObjectKey{
							Name:      task.Name,
							Namespace: task.Namespace,
						})
						continue
					} else {
						taskGarbageCollectionSuccessCount.Inc()
					}
				}

				tr.Info("tasks has been garbage collected", "time", time.Now(), "count", len(reapedTasks))
			}
		}

		// Garbage collect java profiler data on node.
		absolutePath := filepath.Join(tr.dataRoot, "profilers/java/memory")
		err = DeleteExpiredProfilerData(absolutePath, tr.taskTTL, tr)
		if err != nil {
			tr.Error(err, "failed to garbage collect java profiler data")
		}

		// Garbage collect go profiler data on node.
		absoluteGoProfilerPath := filepath.Join(tr.dataRoot, "profilers/go/pprof")
		err = DeleteExpiredProfilerData(absoluteGoProfilerPath, tr.taskTTL, tr)
		if err != nil {
			tr.Error(err, "failed to garbage collect go profiler data")
		}

		// Garbage collect task data on node.
		absoluteDiagnosisPath := filepath.Join(tr.dataRoot, "tasks")
		err = DeleteExpiredProfilerData(absoluteDiagnosisPath, tr.taskTTL, tr)
		if err != nil {
			tr.Error(err, "failed to garbage collect task data")
		}

	}, housekeepingInterval, stopCh)
}

// listTasks lists Diagnoses from cache.
func (tr *TaskReaper) listTasks() ([]diagnosisv1.Task, error) {
	var taskList diagnosisv1.TaskList
	if err := tr.cache.List(tr, &taskList); err != nil {
		return nil, err
	}

	tasksOnNode := util.RetrieveTasksOnNode(taskList.Items, tr.nodeName)

	return tasksOnNode, nil
}

// DeleteExpiredProfilerData deletes profiler data by removing files or directories if the file age is longer
// than taskTTL.
func DeleteExpiredProfilerData(path string, taskTTL time.Duration, log logr.Logger) error {
	// Return if the profiler data directory does not exist.
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}

	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Garbage collect the profiler data if it is under any of the following conditions:
		//
		// Its age is longer than taskTTL.
		if time.Now().Sub(entry.ModTime()) > taskTTL {
			err := util.RemoveFile(filepath.Join(path, entry.Name()))
			if err != nil {
				log.Error(err, "unable to remove file")
			}
		}
	}

	return nil
}
