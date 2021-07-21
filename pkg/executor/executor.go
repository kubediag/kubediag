/*
Copyright 2021 The KubeDiag Authors.

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

package executor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/controllers"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	// MaxDataSize specifies max size of data which could be processed by kubediag.
	// It is the message size limitation in grpc: https://github.com/grpc/grpc-go/blob/v1.30.0/clientconn.go#L95.
	MaxDataSize = 1024 * 1024 * 2

	// DiagnosisUIDTelemetryKey is the telemetry key of diagnosis object uid.
	DiagnosisUIDTelemetryKey = "diagnosis.uid"
	// DiagnosisNamespaceTelemetryKey is the telemetry key of diagnosis namespace.
	DiagnosisNamespaceTelemetryKey = "diagnosis.namespace"
	// DiagnosisNameTelemetryKey is the telemetry key of diagnosis name.
	DiagnosisNameTelemetryKey = "diagnosis.name"
	// PodNamespaceTelemetryKey is the telemetry key of pod namespace.
	PodNamespaceTelemetryKey = "pod.namespace"
	// PodNameTelemetryKey is the telemetry key of pod name.
	PodNameTelemetryKey = "pod.name"
	// ContainerTelemetryKey is the telemetry key of container.
	ContainerTelemetryKey = "container"
	// NodeTelemetryKey is the telemetry key of node.
	NodeTelemetryKey = "node"
)

var (
	executorSyncSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_sync_success_count",
			Help: "Counter of successful diagnosis syncs by executor",
		},
	)
	executorSyncSkipCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_sync_skip_count",
			Help: "Counter of skipped diagnosis syncs by executor",
		},
	)
	executorSyncFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_sync_fail_count",
			Help: "Counter of failed diagnosis syncs by executor",
		},
	)
	executorSyncErrorCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_sync_error_count",
			Help: "Counter of erroneous diagnosis syncs by executor",
		},
	)
	executorOperationErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_operation_error_counter",
			Help: "Counter of erroneous diagnosis syncs by operation",
		},
	)
	executorOperationSuccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_operation_success_counter",
			Help: "Counter of success diagnosis syncs by operation",
		},
	)
	executorOperationFailCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_operation_fail_counter",
			Help: "Counter of fail diagnosis syncs by operation",
		},
	)
)

// Executor changes the state of a diagnosis by executing operations.
type Executor interface {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// Run runs the Executor.
	Run(<-chan struct{})
}

// executor runs the diagnosis pipeline by executing operations defined in diagnosis.
type executor struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// client knows how to perform CRUD operations on Kubernetes objects.
	client client.Client
	// eventRecorder knows how to record events on behalf of an EventSource.
	eventRecorder record.EventRecorder
	// scheme defines methods for serializing and deserializing API objects.
	scheme *runtime.Scheme
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// nodeName specifies the node name.
	nodeName string
	// transport is the transport for sending http requests to executors.
	transport *http.Transport
	// bindAddress is the address on which to advertise.
	bindAddress string
	// port is the port for the kubediag to serve on.
	port int
	// dataRoot is root directory of persistent kubediag data.
	dataRoot string
	// executorCh is a channel for queuing Diagnoses to be processed by executor.
	executorCh chan diagnosisv1.Diagnosis
}

// NewExecutor creates a new executor.
func NewExecutor(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	bindAddress string,
	port int,
	dataRoot string,
	executorCh chan diagnosisv1.Diagnosis,
) Executor {
	metrics.Registry.MustRegister(
		executorSyncSuccessCount,
		executorSyncSkipCount,
		executorSyncFailCount,
		executorSyncErrorCount,
		executorOperationErrorCounter,
		executorOperationSuccessCounter,
		executorOperationFailCounter,
	)

	transport := utilnet.SetTransportDefaults(
		&http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(nil),
		})

	return &executor{
		Context:       ctx,
		Logger:        logger,
		client:        cli,
		eventRecorder: eventRecorder,
		scheme:        scheme,
		cache:         cache,
		nodeName:      nodeName,
		transport:     transport,
		bindAddress:   bindAddress,
		port:          port,
		dataRoot:      dataRoot,
		executorCh:    executorCh,
	}
}

// Run runs the executor.
func (ex *executor) Run(stopCh <-chan struct{}) {
	// Wait for all caches to sync before processing.
	if !ex.cache.WaitForCacheSync(stopCh) {
		return
	}

	for {
		select {
		// Process diagnoses queuing in executor channel.
		case diagnosis := <-ex.executorCh:
			err := ex.client.Get(ex, client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			}, &diagnosis)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				ex.addDiagnosisToExecutorQueue(diagnosis)
				continue
			}

			// Only process diagnosis in DiagnosisRunning phase.
			if diagnosis.Status.Phase != diagnosisv1.DiagnosisRunning {
				continue
			}

			// Only process diagnosis on designated node.
			if util.IsDiagnosisNodeNameMatched(diagnosis, ex.nodeName) {
				diagnosis, err := ex.syncDiagnosis(diagnosis)
				if err != nil {
					ex.Error(err, "failed to sync Diagnosis", "diagnosis", client.ObjectKey{
						Name:      diagnosis.Name,
						Namespace: diagnosis.Namespace,
					})
					executorSyncErrorCount.Inc()
					ex.addDiagnosisToExecutorQueue(diagnosis)
					continue
				}

				ex.Info("syncing Diagnosis successfully", "diagnosis", client.ObjectKey{
					Name:      diagnosis.Name,
					Namespace: diagnosis.Namespace,
				})
			}
		// Stop executor on stop signal.
		case <-stopCh:
			return
		}
	}
}

// syncDiagnosis syncs diagnoses.
func (ex *executor) syncDiagnosis(diagnosis diagnosisv1.Diagnosis) (diagnosisv1.Diagnosis, error) {
	ex.Info("starting to sync Diagnosis", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	})

	// Fetch operationSet according to diagnosis.
	var operationset diagnosisv1.OperationSet
	err := ex.client.Get(ex, client.ObjectKey{
		Name: diagnosis.Spec.OperationSet,
	}, &operationset)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ex.Info("operation set is not found", "operationset", diagnosis.Spec.OperationSet, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})

			ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set is not found", diagnosis.Namespace, diagnosis.Name)
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.OperationSetNotFound,
				Status:  corev1.ConditionTrue,
				Reason:  "OperationSetNotFound",
				Message: fmt.Sprintf("OperationSet %s is not found", diagnosis.Spec.OperationSet),
			})
			if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
				return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			executorSyncFailCount.Inc()
			return diagnosis, nil
		}

		return diagnosis, err
	}

	// Validate the operation set is ready.
	if !operationset.Status.Ready {
		ex.Info("the graph has not been updated according to the latest specification")

		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set is not ready", diagnosis.Namespace, diagnosis.Name)
		diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
		util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
			Type:    diagnosisv1.OperationSetNotReady,
			Status:  corev1.ConditionTrue,
			Reason:  "OperationSetNotReady",
			Message: fmt.Sprintf("OperationSet is not ready because the graph has not been updated according to the latest specification"),
		})
		if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
			return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
		}
		executorSyncFailCount.Inc()
		return diagnosis, nil
	}

	// Update hash value calculated from adjacency list of operation set.
	diagnosisLabels := diagnosis.GetLabels()
	if diagnosisLabels == nil {
		diagnosisLabels = make(map[string]string)
	}
	diagnosisAdjacencyListHash, ok := diagnosisLabels[util.OperationSetUniqueLabelKey]
	if !ok {
		diagnosisLabels[util.OperationSetUniqueLabelKey] = util.ComputeHash(operationset.Spec.AdjacencyList)
		diagnosis.SetLabels(diagnosisLabels)
		if err := ex.client.Update(ex, &diagnosis); err != nil {
			return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
		}

		return diagnosis, fmt.Errorf("hash value of adjacency list calculated")
	}

	// Validate the graph defined by operation set is not changed.
	operationSetLabels := operationset.GetLabels()
	if operationSetLabels == nil {
		operationSetLabels = make(map[string]string)
	}
	operationSetAdjacencyListHash := operationSetLabels[util.OperationSetUniqueLabelKey]
	if operationSetAdjacencyListHash != diagnosisAdjacencyListHash {
		ex.Info("hash value caculated from adjacency list has been changed", "new", operationSetAdjacencyListHash, "old", diagnosisAdjacencyListHash)

		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation set has been changed during execution", diagnosis.Namespace, diagnosis.Name)
		diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
		util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
			Type:    diagnosisv1.OperationSetChanged,
			Status:  corev1.ConditionTrue,
			Reason:  "OperationSetChanged",
			Message: fmt.Sprintf("OperationSet specification has been changed during diagnosis execution"),
		})
		if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
			return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
		}
		executorSyncFailCount.Inc()
		return diagnosis, nil
	}

	// Set initial checkpoint before operation execution.
	if diagnosis.Status.Checkpoint == nil {
		diagnosis.Status.Checkpoint = &diagnosisv1.Checkpoint{
			PathIndex: 0,
			NodeIndex: 0,
		}
	}

	// Retrieve operation node information.
	checkpoint := diagnosis.Status.Checkpoint
	paths := operationset.Status.Paths
	if checkpoint.PathIndex >= len(paths) {
		return diagnosis, fmt.Errorf("invalid path index %d of length %d", checkpoint.PathIndex, len(paths))
	}
	path := paths[checkpoint.PathIndex]
	if checkpoint.NodeIndex >= len(path) {
		return diagnosis, fmt.Errorf("invalid node index %d of length %d", checkpoint.NodeIndex, len(path))
	}
	node := path[checkpoint.NodeIndex]

	// Fetch operation according to operation node information.
	var operation diagnosisv1.Operation
	err = ex.client.Get(ex, client.ObjectKey{
		Name: node.Operation,
	}, &operation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ex.Info("operation is not found", "operation", node.Operation, "operationset", diagnosis.Spec.OperationSet, "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})

			ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s since operation is not found", diagnosis.Namespace, diagnosis.Name)
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.OperationNotFound,
				Status:  corev1.ConditionTrue,
				Reason:  "OperationNotFound",
				Message: fmt.Sprintf("Operation %s is not found", node.Operation),
			})
			if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
				return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			executorSyncFailCount.Inc()
			return diagnosis, nil
		}

		return diagnosis, err
	}

	// Construct request data for current operation by adding contexts and operation results.
	// The request data is a map[string]string which contains key value pairs.
	data := make(map[string]string)
	for key, value := range diagnosis.Spec.Parameters {
		data[key] = value
	}
	for key, value := range diagnosis.Status.OperationResults {
		data[key] = value
	}
	updateDiagnosisContext(data, diagnosis)

	ex.Info("running operation", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	}, "node", node, "operationset", operationset.Name, "path", path)

	// Execute the operation by sending http request to the processor or running predefined script.
	var succeeded bool
	var result map[string]string
	if operation.Spec.Processor.HTTPServer != nil {
		succeeded, result, err = ex.doHTTPRequestWithContext(operation, data)
		if err != nil {
			executorOperationErrorCounter.Inc()
			return diagnosis, err
		}
	} else if operation.Spec.Processor.ScriptRunner != nil {
		succeeded, result, err = ex.runScriptWithContext(operation, data)
		if err != nil {
			executorOperationErrorCounter.Inc()
			return diagnosis, err
		}
	}

	// Update the operation result into diagnosis status.
	if succeeded {
		ex.Info("operation executed successfully", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		}, "node", node, "operationset", operationset.Name, "path", path)
		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "OperationSucceeded", "Operation %s executed successfully", operation.Name)
		executorOperationSuccessCounter.Inc()

		// Set operation result according to response from operaton processor.
		if diagnosis.Status.OperationResults == nil {
			diagnosis.Status.OperationResults = make(map[string]string)
		}
		for key, value := range result {
			diagnosis.Status.OperationResults[key] = value
		}

		// Set current path as succeeded path if current operation is succeeded.
		if diagnosis.Status.SucceededPath == nil {
			diagnosis.Status.SucceededPath = make(diagnosisv1.Path, 0, len(path))
		}
		diagnosis.Status.SucceededPath = append(diagnosis.Status.SucceededPath, node)

		// Set phase to succeeded if current path has been finished and all operations are succeeded.
		if checkpoint.NodeIndex == len(path)-1 {
			ex.Info("running diagnosis successfully", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "DiagnosisSucceeded", "Running %s/%s diagnosis successfully", diagnosis.Namespace, diagnosis.Name)

			util.UpdateDiagnosisCondition(&diagnosis.Status, &diagnosisv1.DiagnosisCondition{
				Type:    diagnosisv1.DiagnosisComplete,
				Status:  corev1.ConditionTrue,
				Reason:  "DiagnosisComplete",
				Message: fmt.Sprintf("Diagnosis is completed"),
			})
			diagnosis.Status.Phase = diagnosisv1.DiagnosisSucceeded
			if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
				return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			executorSyncSuccessCount.Inc()
			return diagnosis, nil
		}

		// Increment node index if path has remaining operations to executed.
		checkpoint.NodeIndex++
	} else {
		ex.Info("failed to execute operation", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		}, "node", node, "operationset", operationset.Name, "path", path)
		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "OperationFailed", "Failed to execute operation %s", operation.Name)
		executorOperationFailCounter.Inc()

		// Set current path as failed path and clear succeeded path if current operation is failed.
		if diagnosis.Status.FailedPaths == nil {
			diagnosis.Status.FailedPaths = make([]diagnosisv1.Path, 0, len(paths))
		}
		diagnosis.Status.FailedPaths = append(diagnosis.Status.FailedPaths, path)
		diagnosis.Status.SucceededPath = nil

		// Set phase to failed if all paths are failed.
		if checkpoint.PathIndex == len(paths)-1 {
			ex.Info("failed to run diagnosis", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s", diagnosis.Namespace, diagnosis.Name)
			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
			if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
				return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
			}
			executorSyncFailCount.Inc()
			return diagnosis, nil
		}

		// Increment path index if paths has remaining paths to executed.
		checkpoint.PathIndex++
	}

	if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
		return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
	}

	return diagnosis, nil
}

// doHTTPRequestWithContext sends a http request to the operation processor with payload.
// It returns a bool, a map and an error as results.
func (ex *executor) doHTTPRequestWithContext(operation diagnosisv1.Operation, data map[string]string) (bool, map[string]string, error) {
	if operation.Spec.Processor.HTTPServer == nil {
		return false, nil, fmt.Errorf("http server not specified")
	}

	// Set http request contexts and construct http client. Use kubediag agent bind address as the processor
	// address if external ip and external port not specified.
	var host string
	var port int32
	if operation.Spec.Processor.HTTPServer.Address != nil {
		host = *operation.Spec.Processor.HTTPServer.Address
	} else {
		host = ex.bindAddress
	}
	if operation.Spec.Processor.HTTPServer.Port != nil {
		port = *operation.Spec.Processor.HTTPServer.Port
	} else {
		port = int32(ex.port)
	}
	path := *operation.Spec.Processor.HTTPServer.Path
	scheme := strings.ToLower(string(*operation.Spec.Processor.HTTPServer.Scheme))
	url := util.FormatURL(scheme, host, strconv.Itoa(int(port)), path)
	timeout := time.Duration(*operation.Spec.Processor.TimeoutSeconds) * time.Second
	cli := &http.Client{
		Timeout:   timeout,
		Transport: ex.transport,
	}

	// Marshal request body and construct http request.
	body, err := json.Marshal(data)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal request body: %s", err)
	}
	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(body))
	if err != nil {
		return false, nil, err
	}

	// Send the http request to operation processor.
	res, err := cli.Do(req)
	if err != nil {
		return false, nil, err
	}
	defer res.Body.Close()
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		ex.Error(err, "failed to read http response body", "response", string(body))
		return false, nil, err
	}

	// Return an error if response body size exceeds max data size.
	if len(body) > MaxDataSize {
		return false, nil, fmt.Errorf("response body size %d exceeds max data size %d", len(body), MaxDataSize)
	}

	if res.StatusCode != http.StatusOK {
		ex.Info("http response with erroneous status", "status", res.Status, "response", string(body))
		return false, nil, nil
	}

	var result map[string]string
	err = json.Unmarshal(body, &result)
	if err != nil {
		ex.Error(err, "failed to marshal response body", "response", string(body))
		// If response code is 200 but body is not a string-map, we think this processor is finished but failed and will not return error
		return false, nil, nil
	}

	return true, result, nil
}

// runScriptWithContext runs a script with the arguments provided by context.
// It returns a bool, a map and an error as results.
func (ex *executor) runScriptWithContext(operation diagnosisv1.Operation, data map[string]string) (bool, map[string]string, error) {
	if operation.Spec.Processor.ScriptRunner == nil {
		return false, nil, fmt.Errorf("script runner not specified")
	}

	// Generate all argument according to script runner definition and execute the script with timeout.
	var args []string
	for _, key := range operation.Spec.Processor.ScriptRunner.ArgKeys {
		if value, ok := data[key]; ok {
			args = append(args, value)
		}
	}
	scriptFilePath := filepath.Join(ex.dataRoot, controllers.ScriptSubDirectory, operation.Name)
	command := append([]string{"/bin/sh", scriptFilePath}, args...)
	output, err := util.BlockingRunCommandWithTimeout(command, *operation.Spec.Processor.TimeoutSeconds)

	// Update script execution result with output and error.
	result := make(map[string]string)
	if operation.Spec.Processor.ScriptRunner.OperationResultKey != nil {
		if output != nil {
			key := strings.Join([]string{"operation", *operation.Spec.Processor.ScriptRunner.OperationResultKey, "output"}, ".")
			result[key] = string(output)
		}
		if err != nil {
			key := strings.Join([]string{"operation", *operation.Spec.Processor.ScriptRunner.OperationResultKey, "error"}, ".")
			result[key] = err.Error()
		}
	}

	return true, result, nil
}

// addDiagnosisToExecutorQueue adds Diagnosis to the queue processed by executor.
func (ex *executor) addDiagnosisToExecutorQueue(diagnosis diagnosisv1.Diagnosis) {
	err := util.QueueDiagnosis(ex, ex.executorCh, diagnosis)
	if err != nil {
		ex.Error(err, "failed to send diagnosis to executor queue", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		})
	}
}

// updateDiagnosisContext updates data with diagnosis contexts.
func updateDiagnosisContext(data map[string]string, diagnosis diagnosisv1.Diagnosis) {
	data[DiagnosisNamespaceTelemetryKey] = diagnosis.Namespace
	data[DiagnosisNameTelemetryKey] = diagnosis.Name
	data[DiagnosisUIDTelemetryKey] = string(diagnosis.UID)
	data[NodeTelemetryKey] = diagnosis.Spec.NodeName
	if diagnosis.Spec.PodReference != nil {
		data[PodNamespaceTelemetryKey] = diagnosis.Spec.PodReference.Namespace
		data[PodNameTelemetryKey] = diagnosis.Spec.PodReference.Name
		if diagnosis.Spec.PodReference.Container != "" {
			data[ContainerTelemetryKey] = diagnosis.Spec.PodReference.Container
		}
	}
}
