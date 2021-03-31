/*
Copyright 2021 The Kube Diagnoser Authors.

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

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/util"
)

const (
	// MaxDataSize specifies max size of data which could be processed by kube diagnoser.
	// It is the message size limitation in grpc: https://github.com/grpc/grpc-go/blob/v1.30.0/clientconn.go#L95.
	MaxDataSize = 1024 * 1024 * 2
	// HTTPRequestBodyParameterKey is the key of parameter to be passed to opreation.
	HTTPRequestBodyParameterKey = "parameter"

	// TraceDiagnosisUUID is uid of diagnosis object, with uuid in http header, we could build a diagnosis flow
	TraceDiagnosisUUID      = "diagnosis-uuid"
	// TraceDiagnosisNamespace is namespace of diagnosis object
	TraceDiagnosisNamespace = "diagnosis-namespace"
	// TraceDiagnosisName is name of diagnosis object
	TraceDiagnosisName      = "diagnosis-name"
	// TracePodNamespace is namespace of pod which diagnosis concern to
	TracePodNamespace       = "pod-namespace"
	// TracePodName is name of pod which diagnosis concern to
	TracePodName            = "pod-name"
	// TracePodContainerName is name of container in pod which diagnosis concern to
	TracePodContainerName   = "pod-container-name"
	// TraceNodeName is name of node which diagnosis concern to
	TraceNodeName           = "node-name"

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
	executorCommandExecutorSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_command_executor_success_count",
			Help: "Counter of successful command executor runs by executor",
		},
	)
	executorCommandExecutorFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_command_executor_fail_count",
			Help: "Counter of failed command executor runs by executor",
		},
	)
	executorProfilerSuccessCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_profiler_success_count",
			Help: "Counter of successful profiler runs by executor",
		},
	)
	executorProfilerFailCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "executor_profiler_fail_count",
			Help: "Counter of failed profiler runs by executor",
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
	// port is the port for the kube diagnoser to serve on.
	port int
	// dataRoot is root directory of persistent kube diagnoser data.
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
		executorCommandExecutorSuccessCount,
		executorCommandExecutorFailCount,
		executorProfilerSuccessCount,
		executorProfilerFailCount,
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

	// Fetch operation set according to diagnosis.
	var operationset diagnosisv1.OperationSet
	err := ex.client.Get(ex, client.ObjectKey{
		Name: diagnosis.Spec.OperationSet,
	}, &operationset)
	if err != nil {
		return diagnosis, err
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
		return diagnosis, err
	}

	// Construct request data for current operation by adding predefined contexts and operation results.
	// The request data is a map[string][]byte which contains key value pairs of operation names and results.
	data := make(map[string]string)
	for idStr, parameter := range diagnosis.Spec.Parameters {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return diagnosis, err
		}

		// Set request data if context id is the same as current node id.
		if id == node.ID {
			data[HTTPRequestBodyParameterKey] = parameter
		}
	}
	for idStr, operationResult := range diagnosis.Status.OperationResults {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return diagnosis, err
		}

		// Set request data if operation result id is among dependences of current node.
		for _, dependence := range node.Dependences {
			if id == dependence {
				if operationResult.Result != nil {
					data[operationResult.Operation] = *operationResult.Result
				}
			}
		}
	}

	ex.Info("running operation", "diagnosis", client.ObjectKey{
		Name:      diagnosis.Name,
		Namespace: diagnosis.Namespace,
	}, "node", node, "operationset", operationset.Name, "path", path)

	// Build diagnosis trace info, which will be frequently quoted to get some global information.
	traceInfo := buildDiagnosisTraceInfo(diagnosis)

	// Execute the operation by sending http request to the processor.
	succeeded, body, err := ex.doHTTPRequestWithContext(operation, data, traceInfo)
	if err != nil {
		return diagnosis, err
	}

	// Update the operation result into diagnosis status.
	if succeeded {
		ex.Info("operation executed successfully", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		}, "node", node, "operationset", operationset.Name, "path", path)
		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeNormal, "OperationSucceeded", "Operation %s executed successfully", operation.Name)

		// Set operation result according to response body from operaton processor.
		result := string(body)
		if diagnosis.Status.OperationResults == nil {
			diagnosis.Status.OperationResults = make(map[string]diagnosisv1.OperationResult)
		}
		idStr := strconv.Itoa(node.ID)
		diagnosis.Status.OperationResults[idStr] = diagnosisv1.OperationResult{
			Operation: node.Operation,
			Result:    &result,
		}

		// Set current path as succeeded path if current operation is succeeded.
		if diagnosis.Status.SucceededPath == nil {
			diagnosis.Status.SucceededPath = make(diagnosisv1.Path, 0, len(path))
		}
		diagnosis.Status.SucceededPath = append(diagnosis.Status.SucceededPath, node)

		// Set phase to succeeded if current path has been finished and all operations are succeeded.
		// Increment node index if path has remaining operations to executed.
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
		} else {
			checkpoint.NodeIndex++
		}
	} else {
		ex.Info("failed to execute operation", "diagnosis", client.ObjectKey{
			Name:      diagnosis.Name,
			Namespace: diagnosis.Namespace,
		}, "node", node, "operationset", operationset.Name, "path", path)
		ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "OperationFailed", "Failed to execute operation %s", operation.Name)

		// Set current path as failed path and clear succeeded path if current operation is failed.
		if diagnosis.Status.FailedPaths == nil {
			diagnosis.Status.FailedPaths = make([]diagnosisv1.Path, 0, len(paths))
		}
		diagnosis.Status.FailedPaths = append(diagnosis.Status.FailedPaths, path)
		diagnosis.Status.SucceededPath = nil

		// Set phase to failed if all paths are failed.
		// Increment path index if paths has remaining paths to executed.
		if checkpoint.PathIndex == len(paths)-1 {
			ex.Info("failed to run diagnosis", "diagnosis", client.ObjectKey{
				Name:      diagnosis.Name,
				Namespace: diagnosis.Namespace,
			})
			ex.eventRecorder.Eventf(&diagnosis, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run diagnosis %s/%s", diagnosis.Namespace, diagnosis.Name)

			diagnosis.Status.Phase = diagnosisv1.DiagnosisFailed
		} else {
			checkpoint.PathIndex++
		}
	}

	if err := ex.client.Status().Update(ex, &diagnosis); err != nil {
		return diagnosis, fmt.Errorf("unable to update Diagnosis: %s", err)
	}

	// Increment counter of successful diagnosis syncs by executor.
	executorSyncSuccessCount.Inc()

	return diagnosis, nil
}

// doHTTPRequestWithContext sends a http request to the operation processor with payload.
// It returns a bool, a response body and an error as results.
func (ex *executor) doHTTPRequestWithContext(operation diagnosisv1.Operation, data, traceInfo map[string]string) (bool, []byte, error) {
	// Set http request contexts and construct http client. Use kube diagnoser agent bind address as the processor
	// address if external ip and external port not specified.
	var host string
	var port int32
	if operation.Spec.Processor.ExternalIP != nil {
		host = *operation.Spec.Processor.ExternalIP
	} else {
		host = ex.bindAddress
	}
	if operation.Spec.Processor.ExternalPort != nil {
		port = *operation.Spec.Processor.ExternalPort
	} else {
		port = int32(ex.port)
	}
	path := *operation.Spec.Processor.Path
	scheme := strings.ToLower(string(*operation.Spec.Processor.Scheme))
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

	// insert trace info into http request's header
	for key, value := range traceInfo {
		req.Header.Set(key, value)
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
		ex.Info("http response with 200 status", "status", res.Status)
		return false, body, nil
	}

	return true, body, nil
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

// buildDiagnosisTraceInfo build a map with all trace info of a diagnosis object.
func buildDiagnosisTraceInfo(diagnosis diagnosisv1.Diagnosis) map[string]string {
	traceInfo := map[string]string{
		TraceDiagnosisNamespace: diagnosis.Namespace,
		TraceDiagnosisName:      diagnosis.Name,
		TraceDiagnosisUUID:      string(diagnosis.UID),
		TraceNodeName:           diagnosis.Spec.NodeName,
	}
	if diagnosis.Spec.PodReference != nil {
		traceInfo[TracePodNamespace] = diagnosis.Spec.PodReference.Namespace
		traceInfo[TracePodName] = diagnosis.Spec.PodReference.Name
		traceInfo[TracePodContainerName] = diagnosis.Spec.PodReference.Container
	}
	return traceInfo
}

