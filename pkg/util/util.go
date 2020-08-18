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

package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
)

const (
	// PodInformationContextKey is the key of pod information in abnormal context.
	PodInformationContextKey = "podInformation"
	// ContainerInformationContextKey is the key of container information in abnormal context.
	ContainerInformationContextKey = "containerInformation"
	// ProcessInformationContextKey is the key of process information in abnormal context.
	ProcessInformationContextKey = "processInformation"
	// PodDiskUsageDiagnosisContextKey is the key of pod disk usage diagnosis result in abnormal context.
	PodDiskUsageDiagnosisContextKey = "podDiskUsageDiagnosis"
	// MaxDataSize specifies max size of data which could be processed by kube diagnoser.
	// It is the message size limitation in grpc: https://github.com/grpc/grpc-go/blob/v1.30.0/clientconn.go#L95.
	MaxDataSize = 1024 * 1024 * 2
	// KubeletRunDirectory specifies the directory where the kubelet runtime information is stored.
	KubeletRunDirectory = "/var/lib/kubelet"
	// KubeletPodDirectory specifies the directory where the kubelet pod information is stored.
	KubeletPodDirectory = "/var/lib/kubelet/pods"
)

// UpdateAbnormalCondition updates existing abnormal condition or creates a new one. Sets
// LastTransitionTime to now if the status has changed.
// Returns true if abnormal condition has changed or has been added.
func UpdateAbnormalCondition(status *diagnosisv1.AbnormalStatus, condition *diagnosisv1.AbnormalCondition) bool {
	condition.LastTransitionTime = metav1.Now()
	// Try to find this abnormal condition.
	conditionIndex, oldCondition := GetAbnormalCondition(status, condition.Type)

	if oldCondition == nil {
		// We are adding new abnormal condition.
		status.Conditions = append(status.Conditions, *condition)
		return true
	}

	// We are updating an existing condition, so we need to check if it has changed.
	if condition.Status == oldCondition.Status {
		condition.LastTransitionTime = oldCondition.LastTransitionTime
	}

	isEqual := condition.Status == oldCondition.Status &&
		condition.Reason == oldCondition.Reason &&
		condition.Message == oldCondition.Message &&
		condition.LastTransitionTime.Equal(&oldCondition.LastTransitionTime)

	status.Conditions[conditionIndex] = *condition

	// Return true if one of the fields have changed.
	return !isEqual
}

// GetAbnormalCondition extracts the provided condition from the given status.
// Returns -1 and nil if the condition is not present, otherwise returns the index of the located condition.
func GetAbnormalCondition(status *diagnosisv1.AbnormalStatus, conditionType diagnosisv1.AbnormalConditionType) (int, *diagnosisv1.AbnormalCondition) {
	if status == nil {
		return -1, nil
	}

	return GetAbnormalConditionFromList(status.Conditions, conditionType)
}

// GetAbnormalConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func GetAbnormalConditionFromList(conditions []diagnosisv1.AbnormalCondition, conditionType diagnosisv1.AbnormalConditionType) (int, *diagnosisv1.AbnormalCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}

	return -1, nil
}

// FormatURL formats a URL from args.
func FormatURL(scheme string, host string, port string, path string) *url.URL {
	u, err := url.Parse(path)
	// Something is busted with the path, but it's too late to reject it. Pass it along as is.
	if err != nil {
		u = &url.URL{
			Path: path,
		}
	}

	u.Scheme = scheme
	u.Host = net.JoinHostPort(host, port)

	return u
}

// DoHTTPRequestWithAbnormal sends a http request to diagnoser, recoverer or information collector with payload of abnormal.
// It returns an Abnormal and an error as results.
func DoHTTPRequestWithAbnormal(abnormal diagnosisv1.Abnormal, url *url.URL, cli http.Client, log logr.Logger) (diagnosisv1.Abnormal, error) {
	data, err := json.Marshal(abnormal)
	if err != nil {
		return abnormal, err
	}

	req, err := http.NewRequest("POST", url.String(), bytes.NewBuffer(data))
	if err != nil {
		return abnormal, err
	}

	res, err := cli.Do(req)
	if err != nil {
		return abnormal, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(err, "failed to read http response body", "response", string(body))
		return abnormal, err
	}

	if res.StatusCode == http.StatusOK {
		err = json.Unmarshal(body, &abnormal)
		if err != nil {
			log.Error(err, "failed to marshal response body", "response", string(body))
			return abnormal, err
		}

		log.Info("succeed to complete http request", "abnormal", client.ObjectKey{
			Name:      abnormal.Name,
			Namespace: abnormal.Namespace,
		}, "status", res.Status)
		return abnormal, nil
	}

	log.Info("failed to complete http request", "status", res.Status, "response", string(body))
	return abnormal, fmt.Errorf("failed with status: %s", res.Status)
}

// ValidateAbnormalResult validates an abnormal after processed by a diagnoser, recoverer or information collector.
func ValidateAbnormalResult(result diagnosisv1.Abnormal, current diagnosisv1.Abnormal) error {
	if !reflect.DeepEqual(result.Spec, current.Spec) {
		return fmt.Errorf("spec field of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Identifiable, current.Status.Identifiable) {
		return fmt.Errorf("identifiable filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Recoverable, current.Status.Recoverable) {
		return fmt.Errorf("recoverable filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Phase, current.Status.Phase) {
		return fmt.Errorf("phase filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Conditions, current.Status.Conditions) {
		return fmt.Errorf("conditions filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Message, current.Status.Message) {
		return fmt.Errorf("message filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Reason, current.Status.Reason) {
		return fmt.Errorf("reason filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.StartTime, current.Status.StartTime) {
		return fmt.Errorf("startTime filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Diagnoser, current.Status.Diagnoser) {
		return fmt.Errorf("diagnoser filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Recoverer, current.Status.Recoverer) {
		return fmt.Errorf("recoverer filed of Abnormal must not be modified")
	}

	return nil
}

// QueueAbnormal sends an abnormal to a channel. It returns an error if the channel is blocked.
func QueueAbnormal(ctx context.Context, channel chan diagnosisv1.Abnormal, abnormal diagnosisv1.Abnormal) error {
	select {
	case <-ctx.Done():
		return nil
	case channel <- abnormal:
		return nil
	default:
		return fmt.Errorf("channel is blocked")
	}
}

// QueueAbnormalWithTimer sends an abnormal to a channel after a timer expires.
func QueueAbnormalWithTimer(ctx context.Context, duration time.Duration, channel chan diagnosisv1.Abnormal, abnormal diagnosisv1.Abnormal) error {
	timer := time.NewTimer(duration)
	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		return QueueAbnormal(ctx, channel, abnormal)
	}
}

// IsAbnormalNodeNameMatched checks if the abnormal is on the specific node.
// It returns true if node name of the abnormal is empty or matches provided node name, otherwise false.
func IsAbnormalNodeNameMatched(abnormal diagnosisv1.Abnormal, nodeName string) bool {
	return abnormal.Spec.NodeName == "" || abnormal.Spec.NodeName == nodeName
}

// SetAbnormalContext sets context field of an abnormal with provided key and value.
func SetAbnormalContext(abnormal diagnosisv1.Abnormal, key string, value interface{}) (diagnosisv1.Abnormal, error) {
	if abnormal.Status.Context == nil {
		abnormal.Status.Context = new(runtime.RawExtension)
	}
	current, err := abnormal.Status.Context.MarshalJSON()
	if err != nil {
		return abnormal, err
	}

	// Parsed context will be nil if raw data is empty.
	// Use map[string]interface{} instead of map[string][]byte for readability in json or yaml format.
	context := make(map[string]interface{})
	err = json.Unmarshal(current, &context)
	if err != nil {
		return abnormal, err
	}

	// Reinitialize context if context is nil.
	if context == nil {
		context = make(map[string]interface{})
	}
	context[key] = value
	result, err := json.Marshal(context)
	if err != nil {
		return abnormal, err
	}

	err = abnormal.Status.Context.UnmarshalJSON(result)
	if err != nil {
		return abnormal, err
	}

	return abnormal, nil
}

// GetAbnormalContext gets context field of an abnormal with provided key.
func GetAbnormalContext(abnormal diagnosisv1.Abnormal, key string) ([]byte, error) {
	if abnormal.Status.Context == nil {
		return nil, fmt.Errorf("abnormal context nil")
	}
	current, err := abnormal.Status.Context.MarshalJSON()
	if err != nil {
		return nil, err
	}

	// Parsed context will be nil if raw data is empty.
	context := make(map[string]interface{})
	err = json.Unmarshal(current, &context)
	if err != nil {
		return nil, err
	}

	// Return error if abnormal context is empty.
	if context == nil {
		return nil, fmt.Errorf("abnormal context empty")
	}
	value, ok := context[key]
	if !ok {
		return nil, fmt.Errorf("context key not exist: %s", key)
	}

	result, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// RemoveAbnormalContext removes context field of an abnormal with provided key.
func RemoveAbnormalContext(abnormal diagnosisv1.Abnormal, key string) (diagnosisv1.Abnormal, bool, error) {
	if abnormal.Status.Context == nil {
		return abnormal, true, nil
	}
	current, err := abnormal.Status.Context.MarshalJSON()
	if err != nil {
		return abnormal, false, err
	}

	// Parsed context will be nil if raw data is empty.
	context := make(map[string]interface{})
	err = json.Unmarshal(current, &context)
	if err != nil {
		return abnormal, false, err
	}

	// Delete value with provided key from context.
	if context == nil {
		return abnormal, true, nil
	}
	delete(context, key)

	result, err := json.Marshal(context)
	if err != nil {
		return abnormal, false, err
	}

	err = abnormal.Status.Context.UnmarshalJSON(result)
	if err != nil {
		return abnormal, false, err
	}

	return abnormal, true, nil
}

// RetrievePodsOnNode retrieves all pods on the provided node.
func RetrievePodsOnNode(pods []corev1.Pod, nodeName string) []corev1.Pod {
	podsOnNode := make([]corev1.Pod, 0)
	for _, pod := range pods {
		if pod.Spec.NodeName == nodeName {
			podsOnNode = append(podsOnNode, pod)
		}
	}

	return podsOnNode
}

// GetTotalBytes gets total bytes in filesystem.
func GetTotalBytes(path string) uint64 {
	var stat syscall.Statfs_t
	syscall.Statfs(path, &stat)

	return stat.Blocks * uint64(stat.Bsize)
}

// GetFreeBytes gets free bytes in filesystem.
func GetFreeBytes(path string) uint64 {
	var stat syscall.Statfs_t
	syscall.Statfs(path, &stat)

	return stat.Bfree * uint64(stat.Bsize)
}

// GetAvailableBytes gets available bytes in filesystem.
func GetAvailableBytes(path string) uint64 {
	var stat syscall.Statfs_t
	syscall.Statfs(path, &stat)

	return stat.Bavail * uint64(stat.Bsize)
}

// GetUsedBytes gets used bytes in filesystem.
func GetUsedBytes(path string) uint64 {
	var stat syscall.Statfs_t
	syscall.Statfs(path, &stat)

	return (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
}

// DiskUsage calculates the disk usage of a directory by executing du command.
func DiskUsage(path string) (int, error) {
	// Uses the same niceness level as cadvisor.fs does when running du.
	// Uses -B 1 to always scale to a blocksize of 1 byte.
	out, err := exec.Command("nice", "-n", "19", "du", "-s", "-B", "1", path).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("execute command du ($ nice -n 19 du -s -B 1) on path %s with error %v", path, err)
	}

	size, err := strconv.Atoi(strings.Fields(string(out))[0])
	if err != nil {
		return 0, fmt.Errorf("unable to parse du output %s due to error %v", out, err)
	}

	return size, nil
}
