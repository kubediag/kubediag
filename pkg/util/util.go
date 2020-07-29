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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
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
func ValidateAbnormalResult(result diagnosisv1.Abnormal, curr diagnosisv1.Abnormal) error {
	if !reflect.DeepEqual(result.Spec, curr.Spec) {
		return fmt.Errorf("spec field of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Identifiable, curr.Status.Identifiable) {
		return fmt.Errorf("identifiable filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Recoverable, curr.Status.Recoverable) {
		return fmt.Errorf("recoverable filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Phase, curr.Status.Phase) {
		return fmt.Errorf("phase filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Conditions, curr.Status.Conditions) {
		return fmt.Errorf("conditions filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Message, curr.Status.Message) {
		return fmt.Errorf("message filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Reason, curr.Status.Reason) {
		return fmt.Errorf("reason filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Output, curr.Status.Output) {
		return fmt.Errorf("output filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.StartTime, curr.Status.StartTime) {
		return fmt.Errorf("startTime filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Diagnoser, curr.Status.Diagnoser) {
		return fmt.Errorf("diagnoser filed of Abnormal must not be modified")
	}
	if !reflect.DeepEqual(result.Status.Recoverer, curr.Status.Recoverer) {
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
