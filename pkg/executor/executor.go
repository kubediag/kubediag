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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/controllers"
	"github.com/kubediag/kubediag/pkg/function"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	// MaxDataSize specifies max size of data which could be processed by kubediag.
	// It is the message size limitation in grpc: https://github.com/grpc/grpc-go/blob/v1.30.0/clientconn.go#L95.
	MaxDataSize = 1024 * 1024 * 2

	// TaskUIDTelemetryKey is the telemetry key of task object uid.
	TaskUIDTelemetryKey = "task.uid"
	// TaskNamespaceTelemetryKey is the telemetry key of task namespace.
	TaskNamespaceTelemetryKey = "task.namespace"
	// TaskNameTelemetryKey is the telemetry key of task name.
	TaskNameTelemetryKey = "task.name"
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

	// DefaultFunctionNamepace is the dafault namespace for k8s object created by function processor.
	DefaultFunctionNamespace = "kubediag"
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

// TaskBackoff is the recommended backoff for a failure when syncing diagnosis.
var TaskBackoff = wait.Backoff{
	Steps:    4,
	Duration: 30 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
}

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
	// dockerClient is the API client that performs all operations against a docker server.
	dockerClient dockerclient.Client
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
	// taskCh is a channel for queuing Tasks to be processed by executor.
	taskCh chan diagnosisv1.Task
}

// NewExecutor creates a new executor.
func NewExecutor(
	ctx context.Context,
	logger logr.Logger,
	cli client.Client,
	dockerClient dockerclient.Client,
	eventRecorder record.EventRecorder,
	scheme *runtime.Scheme,
	cache cache.Cache,
	nodeName string,
	bindAddress string,
	port int,
	dataRoot string,
	taskCh chan diagnosisv1.Task,
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
		dockerClient:  dockerClient,
		eventRecorder: eventRecorder,
		scheme:        scheme,
		cache:         cache,
		nodeName:      nodeName,
		transport:     transport,
		bindAddress:   bindAddress,
		port:          port,
		dataRoot:      dataRoot,
		taskCh:        taskCh,
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
		case task := <-ex.taskCh:
			err := ex.client.Get(ex, client.ObjectKey{
				Name:      task.Name,
				Namespace: task.Namespace,
			}, &task)
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				ex.addTaskToExecutorQueue(task)
				continue
			}

			// Only process task in DiagnosisRunning phase.
			if task.Status.Phase != diagnosisv1.TaskRunning {
				continue
			}

			// Only process diagnosis on designated node.
			if util.IsTaskNodeNameMatched(task, ex.nodeName) {
				go func() {
					task, err := ex.SyncTaskWithRetry(TaskBackoff, task)
					if err != nil {
						ex.Error(err, "failed to sync Task", "task", client.ObjectKey{
							Name:      task.Name,
							Namespace: task.Namespace,
						})
						executorSyncErrorCount.Inc()
						return
					}

					ex.Info("syncing Task successfully", "task", client.ObjectKey{
						Name:      task.Name,
						Namespace: task.Namespace,
					})
				}()
			}
		// Stop executor on stop signal.
		case <-stopCh:
			return
		}
	}
}

// SyncTaskWithRetry syncs diagnoses with backoff.
func (ex *executor) SyncTaskWithRetry(backoff wait.Backoff, task diagnosisv1.Task) (diagnosisv1.Task, error) {
	err := errors.New("timed out waiting for the condition")
	for backoff.Steps > 0 {
		if task, err = ex.syncTask(task); err == nil {
			return task, nil
		}
		if backoff.Steps == 1 {
			break
		}
		time.Sleep(backoff.Step())
	}

	// Set phase to failed.
	ex.eventRecorder.Eventf(&task, corev1.EventTypeWarning, "DiagnosisFailed", "Failed to run task %s/%s since sync task failed", task.Namespace, task.Name)
	task.Status.Phase = diagnosisv1.TaskFailed
	util.UpdateTaskCondition(&task.Status, &diagnosisv1.TaskCondition{
		Type:    diagnosisv1.TaskAccepted,
		Status:  corev1.ConditionTrue,
		Reason:  "SyncTaskFailed",
		Message: err.Error(),
	})
	if err := ex.client.Status().Update(ex, &task); err != nil {
		return task, fmt.Errorf("unable to update Diagnosis: %s", err)
	}
	executorSyncFailCount.Inc()
	return task, err
}

// syncTask syncs tasks.
// TODO: Control the logic to enqueue a task on failure. For example, A task with max data size should not be enqueued.
func (ex *executor) syncTask(task diagnosisv1.Task) (diagnosisv1.Task, error) {
	ex.Info("starting to sync Task", "task", client.ObjectKey{
		Name:      task.Name,
		Namespace: task.Namespace,
	})

	// Fetch operation according to operation node information.
	var operation diagnosisv1.Operation
	err := ex.client.Get(ex, client.ObjectKey{
		Name: task.Spec.Operation,
	}, &operation)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ex.Info("operation is not found", "operation", operation.Name, "task", client.ObjectKey{
				Name:      task.Name,
				Namespace: task.Namespace,
			})

			ex.eventRecorder.Eventf(&task, corev1.EventTypeWarning, "TaskFailed", "Failed to run task %s/%s since operation is not found", task.Namespace, task.Name)
			task.Status.Phase = diagnosisv1.TaskFailed
			util.UpdateTaskCondition(&task.Status, &diagnosisv1.TaskCondition{
				Type:    diagnosisv1.OperationNotFound,
				Status:  corev1.ConditionTrue,
				Reason:  "OperationNotFound",
				Message: fmt.Sprintf("Operation %s is not found", operation.Name),
			})
			if err := ex.client.Status().Update(ex, &task); err != nil {
				return task, fmt.Errorf("unable to update Task: %s", err)
			}
			executorSyncFailCount.Inc()
			return task, nil
		}

		return task, err
	}

	// Construct request data for current operation by adding contexts and operation results.
	// The request data is a map[string]string which contains key value pairs.
	data := make(map[string]string)
	for key, value := range task.Spec.Parameters {
		data[key] = value
	}
	updateTaskContext(data, task)

	ex.Info("running operation", "task", client.ObjectKey{
		Name:      task.Name,
		Namespace: task.Namespace,
	}, "operation", operation.Name)

	// Execute the operation by sending http request to the processor or running predefined script.
	var succeeded bool
	var result map[string]string
	if operation.Spec.Processor.HTTPServer != nil {
		succeeded, result, err = ex.doHTTPRequestWithContext(operation, data)
		if err != nil {
			executorOperationErrorCounter.Inc()
			return task, err
		}
	} else if operation.Spec.Processor.ScriptRunner != nil {
		succeeded, result, err = ex.runScriptWithContext(operation, data)
		if err != nil {
			executorOperationErrorCounter.Inc()
			return task, err
		}
	} else if operation.Spec.Processor.Function != nil {
		succeeded, result, err = ex.runFunctionWithContext(operation, data)
		if err != nil {
			executorOperationErrorCounter.Inc()
			return task, err
		}
	}

	// Update the operation result into task status.
	if succeeded {
		ex.Info("operation executed successfully", "task", client.ObjectKey{
			Name:      task.Name,
			Namespace: task.Namespace,
		}, "operation", operation.Name)
		ex.eventRecorder.Eventf(&task, corev1.EventTypeNormal, "OperationSucceeded", "Operation %s executed successfully", operation.Name)
		executorOperationSuccessCounter.Inc()

		// Set operation result according to response from operaton processor.
		if task.Status.Results == nil {
			task.Status.Results = make(map[string]string)
		}
		for key, value := range result {
			task.Status.Results[key] = value
		}

		task.Status.Phase = diagnosisv1.TaskSucceeded
		if err := ex.client.Status().Update(ex, &task); err != nil {
			return task, fmt.Errorf("unable to update Task: %s", err)
		}
		executorSyncSuccessCount.Inc()
		return task, nil
	} else {
		ex.Info("failed to execute operation", "task", client.ObjectKey{
			Name:      task.Name,
			Namespace: task.Namespace,
		}, "operation", operation.Name)
		ex.eventRecorder.Eventf(&task, corev1.EventTypeWarning, "OperationFailed", "Failed to execute operation %s", operation.Name)
		executorOperationFailCounter.Inc()

		task.Status.Phase = diagnosisv1.TaskFailed
		if err := ex.client.Status().Update(ex, &task); err != nil {
			return task, fmt.Errorf("unable to update Diagnosis: %s", err)
		}
		executorSyncFailCount.Inc()
		return task, nil
	}
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
		ex.Error(err, "failed to unmarshal response body", "response", string(body))
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

// runFunctionWithContext runs a function with provided context.
// It returns a bool, a map and an error as results.
func (ex *executor) runFunctionWithContext(operation diagnosisv1.Operation, data map[string]string) (bool, map[string]string, error) {
	if operation.Spec.Processor.Function == nil {
		return false, nil, fmt.Errorf("function not specified")
	}

	imageName, tag := function.GetImageNameAndTag(&operation)
	// Check if exist the image in local host.
	if !function.ImageExists(ex.dockerClient, imageName, tag) {
		ex.Info("image does not exist, try to build image", "image", imageName+":"+tag)
		// imageBuildMessage stores information returned by docker server after building an image.
		imageBuildMessage := new(bytes.Buffer)
		err := function.BuildFunctionImage(ex.dockerClient, &operation, string(operation.Spec.Processor.Function.Runtime), imageBuildMessage)
		if err != nil {
			ex.Error(err, "failed to build docker image for function processor")
			return false, nil, err
		}
		ex.Info(imageBuildMessage.String())
	}

	namespacedName, err := ex.EnsureK8sResource(&operation)
	if err != nil {
		return false, nil, fmt.Errorf("failed to ensure k8s object (Pod) for function processor")
	}

	pod := corev1.Pod{}
	err = ex.client.Get(ex, namespacedName, &pod)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get Pod for processing function")
	}

	var host string
	var port int32
	host = pod.Status.PodIP
	port = pod.Spec.Containers[0].Ports[0].ContainerPort
	path := "/"
	scheme := "http"
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

	// Send the http request.
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
		ex.Error(err, "failed to unmarshal response body", "response", string(body))
		// If response code is 200 but body is not a string-map, we think this processor is finished but failed and will not return error
		return false, nil, nil
	}

	return true, result, nil
}

// ensureK8sResource creates/updates k8s object (pod) for the operation.
func (ex *executor) EnsureK8sResource(operation *diagnosisv1.Operation) (namespacedName types.NamespacedName, err error) {
	namespacedName = types.NamespacedName{
		Namespace: DefaultFunctionNamespace,
		Name:      operation.Name,
	}

	or, err := util.GetOwnerReference(operation.Kind, operation.APIVersion, operation.Name, operation.UID)
	if err != nil {
		return
	}

	labels := getDefaultLabel(nil)

	imageName, tag := function.GetImageNameAndTag(operation)

	pod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:            operation.Name,
			Namespace:       DefaultFunctionNamespace,
			OwnerReferences: or,
			Labels:          labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "function",
					Image: imageName + ":" + tag,
					// Use local image created by Operation Controller.
					ImagePullPolicy: corev1.PullNever,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
						},
					},
				},
			},
			NodeName: ex.nodeName,
		},
	}

	ctx := context.Background()
	err = ex.client.Create(ctx, &pod)
	if err != nil && apierrors.IsAlreadyExists(err) {
		// In case the Pod is already exists
		// update just certain fields
		newPod := &corev1.Pod{}
		err = ex.client.Get(ctx, namespacedName, newPod)
		if err != nil {
			return
		}
		if !hasDefaultLabel(newPod.ObjectMeta.Labels) {
			err = fmt.Errorf("found a conflicting pod object %s/%s. Aborting", namespacedName.Namespace, namespacedName.Name)
			return
		}

		// A merge patch will preserve other fields modified at runtime.
		patch := client.MergeFrom(newPod.DeepCopy())
		newPod.ObjectMeta.Labels = labels
		newPod.ObjectMeta.OwnerReferences = or
		newPod.Spec.Containers[0].Image = pod.Spec.Containers[0].Image
		err = ex.client.Patch(ctx, newPod, patch)
		if err != nil {
			return
		}
	}

	return
}

func getDefaultLabel(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["created-by"] = "kubediag"
	return labels
}

func hasDefaultLabel(labels map[string]string) bool {
	if labels == nil || labels["created-by"] != "kubediag" {
		return false
	}
	return true
}

// addTaskToExecutorQueue adds Task to the queue processed by executor.
func (ex *executor) addTaskToExecutorQueue(task diagnosisv1.Task) {
	err := util.QueueTask(ex, ex.taskCh, task)
	if err != nil {
		ex.Error(err, "failed to send task to executor queue", "task", client.ObjectKey{
			Name:      task.Name,
			Namespace: task.Namespace,
		})
	}
}

// updateTaskContext updates data with task contexts.
func updateTaskContext(data map[string]string, task diagnosisv1.Task) {
	data[TaskNamespaceTelemetryKey] = task.Namespace
	data[TaskNameTelemetryKey] = task.Name
	data[TaskUIDTelemetryKey] = string(task.UID)
	data[NodeTelemetryKey] = task.Spec.NodeName
	if task.Spec.PodReference != nil {
		data[PodNamespaceTelemetryKey] = task.Spec.PodReference.Namespace
		data[PodNameTelemetryKey] = task.Spec.PodReference.Name
		if task.Spec.PodReference.Container != "" {
			data[ContainerTelemetryKey] = task.Spec.PodReference.Container
		}
	}
}
