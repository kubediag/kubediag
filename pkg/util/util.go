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

package util

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

const (
	// ProfilerEndpointExpiredValue is the value of endpoint in profiler status after expiration duration.
	ProfilerEndpointExpiredValue = "expired"
	// KubeletRunDirectory specifies the directory where the kubelet runtime information is stored.
	KubeletRunDirectory = "/var/lib/kubelet"
	// KubeletPodDirectory specifies the directory where the kubelet pod information is stored.
	KubeletPodDirectory = "/var/lib/kubelet/pods"
	// DefautlNamespace is the default namespace of kubediag.
	DefautlNamespace = "kubediag"
	// PodKillGracePeriodSeconds is the duration in seconds after the pod is forcibly halted
	// with a kill signal and the time when the pod is taken as an abormal pod.
	PodKillGracePeriodSeconds = 30
	// TerminatingPodDiagnosisNamePrefix is the name prefix for creating terminating pod diagnosis.
	TerminatingPodDiagnosisNamePrefix = "terminating-pod"
	// MemoryAnalyzerLeakSuspectsReportAPI is the eclipse memory analyzer api for leak suspects report.
	MemoryAnalyzerLeakSuspectsReportAPI = "org.eclipse.mat.api:suspects"
	// MemoryAnalyzerSystemOverviewReportAPI is the eclipse memory analyzer api for system overview report.
	MemoryAnalyzerSystemOverviewReportAPI = "org.eclipse.mat.api:overview"
	// MemoryAnalyzerTopComponentsReportAPI is the eclipse memory analyzer api for top components report.
	MemoryAnalyzerTopComponentsReportAPI = "org.eclipse.mat.api:top_components"
	// MemoryAnalyzerLeakSuspectsSuffix is the suffix for leak suspects report directory.
	MemoryAnalyzerLeakSuspectsSuffix = "_Leak_Suspects"
	// MemoryAnalyzerSystemOverviewSuffix is the suffix for system overview report directory.
	MemoryAnalyzerSystemOverviewSuffix = "_System_Overview"
	// MemoryAnalyzerTopComponentsSuffix is the suffix for top components report directory.
	MemoryAnalyzerTopComponentsSuffix = "_Top_Components"
	// MemoryAnalyzerHomepage is the html text for memory analyzer homepage.
	MemoryAnalyzerHomepage = `<h2>Eclipse Memory Analyzer</h2><ul><li><a href="/leaksuspects/">Leak Suspects</a></li><li><a href="/systemoverview/">System Overview</a></li><li><a href="/topcomponents/">Top Components</a></li></ul>`
	// GoProfilerPathPrefix is the path prefix for go profiler pprof url.
	GoProfilerPathPrefix = "/debug/pprof/"
	// KubeDiagPrefix is the key prefix for annotations about kubediag.
	KubeDiagPrefix = "diagnosis.kubediag.org/"
	// OperationSetUniqueLabelKey is the default key of the label that is added to existing OperationSets and Diagnoses
	// to prevent conflicts on changed OperationSets and running Diagnoses.
	OperationSetUniqueLabelKey = "adjacency-list-hash"
	// AlphaNums omits vowels from the set of available characters to reduce the chances of "bad words" being formed.
	AlphaNums = "bcdfghjklmnpqrstvwxz2456789"
)

// UpdateDiagnosisCondition updates existing diagnosis condition or creates a new one. Sets
// LastTransitionTime to now if the status has changed.
// Returns true if diagnosis condition has changed or has been added.
func UpdateDiagnosisCondition(status *diagnosisv1.DiagnosisStatus, condition *diagnosisv1.DiagnosisCondition) bool {
	condition.LastTransitionTime = metav1.Now()
	// Try to find this diagnosis condition.
	conditionIndex, oldCondition := GetDiagnosisCondition(status, condition.Type)

	if oldCondition == nil {
		// We are adding new diagnosis condition.
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

// GetDiagnosisCondition extracts the provided condition from the given status.
// Returns -1 and nil if the condition is not present, otherwise returns the index of the located condition.
func GetDiagnosisCondition(status *diagnosisv1.DiagnosisStatus, conditionType diagnosisv1.DiagnosisConditionType) (int, *diagnosisv1.DiagnosisCondition) {
	if status == nil {
		return -1, nil
	}

	return GetDiagnosisConditionFromList(status.Conditions, conditionType)
}

// GetDiagnosisConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func GetDiagnosisConditionFromList(conditions []diagnosisv1.DiagnosisCondition, conditionType diagnosisv1.DiagnosisConditionType) (int, *diagnosisv1.DiagnosisCondition) {
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

// GetPodUnhealthyReason extracts the reason of terminated or waiting container in the pod if the pod is
// not ready. The parameter must be an unhealthy pod.
// It returns the reason of the first terminated or waiting container.
func GetPodUnhealthyReason(pod corev1.Pod) string {
	// Return the reason of the first terminated or waiting container.
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// Skip ready containers.
		if containerStatus.Ready {
			continue
		}

		if containerStatus.State.Terminated != nil {
			return containerStatus.State.Terminated.Reason
		} else if containerStatus.State.Waiting != nil {
			return containerStatus.State.Waiting.Reason
		}
	}

	// Return the reason of the first unready container if last termination state is documented.
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// Skip ready containers.
		if containerStatus.Ready {
			continue
		}

		if containerStatus.LastTerminationState.Terminated != nil {
			return containerStatus.LastTerminationState.Terminated.Reason
		}
	}

	// The pod unhealthy reason will be Unknown if no unhealthy container status is reported.
	return "Unknown"
}

// UpdatePodUnhealthyReasonStatistics updates container state reason map of unhealthy pods.
// It returns true if the reason is not empty, otherwise false.
func UpdatePodUnhealthyReasonStatistics(containerStateReasons map[string]int, reason string) bool {
	if containerStateReasons == nil {
		containerStateReasons = make(map[string]int)
	}

	if reason == "" {
		return false
	}
	containerStateReasons[reason]++

	return true
}

// IsNodeReady returns true if its Ready condition is set to true and it does not have NetworkUnavailable
// condition set to true.
func IsNodeReady(node corev1.Node) bool {
	nodeReady := false
	networkReady := true
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				nodeReady = true
			}
		}
		if condition.Type == corev1.NodeNetworkUnavailable {
			if condition.Status == corev1.ConditionTrue {
				networkReady = false
			}
		}
	}

	return nodeReady && networkReady
}

// GetNodeUnhealthyConditionType extracts the condition type of unhealthy node. The parameter must be an
// unhealthy node.
// It returns the type of the first unhealthy condition.
func GetNodeUnhealthyConditionType(node corev1.Node) corev1.NodeConditionType {
	for _, condition := range node.Status.Conditions {
		// Return the reason of the first unhealthy condition.
		if condition.Type != corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return condition.Type
		}
	}

	// The node condition will be Unknown if no unhealthy condition is reported.
	return "Unknown"
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

// QueueDiagnosis sends a diagnosis to a channel. It returns an error if the channel is blocked.
func QueueDiagnosis(ctx context.Context, channel chan diagnosisv1.Diagnosis, diagnosis diagnosisv1.Diagnosis) error {
	select {
	case <-ctx.Done():
		return nil
	case channel <- diagnosis:
		return nil
	default:
		return fmt.Errorf("channel is blocked")
	}
}

// QueueOperationSet sends an operation set to a channel. It returns an error if the channel is blocked.
func QueueOperationSet(ctx context.Context, channel chan diagnosisv1.OperationSet, operationSet diagnosisv1.OperationSet) error {
	select {
	case <-ctx.Done():
		return nil
	case channel <- operationSet:
		return nil
	default:
		return fmt.Errorf("channel is blocked")
	}
}

// QueueEvent sends an event to a channel. It returns an error if the channel is blocked.
func QueueEvent(ctx context.Context, channel chan corev1.Event, event corev1.Event) error {
	select {
	case <-ctx.Done():
		return nil
	case channel <- event:
		return nil
	default:
		return fmt.Errorf("channel is blocked")
	}
}

// IsDiagnosisCompleted return true if Diagnosis is failed or succeed
func IsDiagnosisCompleted(diagnosis diagnosisv1.Diagnosis) bool {
	return diagnosis.Status.Phase == diagnosisv1.DiagnosisSucceeded || diagnosis.Status.Phase == diagnosisv1.DiagnosisFailed
}

// IsDiagnosisNodeNameMatched checks if the diagnosis is on the specific node.
// It returns true if node name of the diagnosis is empty or matches provided node name, otherwise false.
func IsDiagnosisNodeNameMatched(diagnosis diagnosisv1.Diagnosis, nodeName string) bool {
	return diagnosis.Spec.NodeName == "" || diagnosis.Spec.NodeName == nodeName
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

// RetrieveDiagnosesOnNode retrieves all diagnoses on the provided node.
func RetrieveDiagnosesOnNode(diagnoses []diagnosisv1.Diagnosis, nodeName string) []diagnosisv1.Diagnosis {
	diagnosesOnNode := make([]diagnosisv1.Diagnosis, 0)
	for _, diagnosis := range diagnoses {
		if diagnosis.Spec.NodeName == nodeName {
			diagnosesOnNode = append(diagnosesOnNode, diagnosis)
		}
	}

	return diagnosesOnNode
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

// DiskUsage calculates the disk usage of a directory by executing "du" command.
func DiskUsage(path string) (int, error) {
	// Uses the same niceness level as cadvisor.fs does when running "du".
	// Uses -B 1 to always scale to a blocksize of 1 byte.
	// Set 10 seconds timeout for "du" command.
	command := []string{"nice", "-n", "19", "du", "-s", "-B", "1", path}
	out, err := BlockingRunCommandWithTimeout(command, 60)
	if err != nil {
		return 0, fmt.Errorf("execute command du ($ nice -n 19 du -s -B 1) on path %s with error %v", path, err)
	}

	size, err := strconv.Atoi(strings.Fields(string(out))[0])
	if err != nil {
		return 0, fmt.Errorf("unable to parse du output %s due to error %v", out, err)
	}

	return size, nil
}

// GetProgramPID finds the process ID of a running program by executing "pidof" command.
func GetProgramPID(program string) ([]int, error) {
	command := []string{"pidof", program}
	out, err := BlockingRunCommandWithTimeout(command, 60)
	if err != nil {
		return nil, fmt.Errorf("execute command pidof %s with error %v", program, err)
	}

	pids := make([]int, 0)
	pidStrs := strings.Fields(string(out))
	for _, pidStr := range pidStrs {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse pid string %s due to error %v", pidStr, err)
		}
		pids = append(pids, pid)
	}

	if len(pids) == 0 {
		return nil, fmt.Errorf("unable to find any pid")
	}

	return pids, nil
}

// RemoveFile removes a file or a directory by executing "rm" command.
func RemoveFile(path string) error {
	command := []string{"rm", "-r", "-f", path}
	_, err := BlockingRunCommandWithTimeout(command, 60)
	if err != nil {
		return fmt.Errorf("execute command rm ($ rm -r -f) on path %s with error %v", path, err)
	}

	return nil
}

// ParseHPROFFile parses hprof file with eclipse memory analyzer. The results are stored in zip files under
// the same directory of hprof file.
// It takes command working directory, hprof file path and timeout seconds as parameters.
func ParseHPROFFile(workdir string, path string, timeoutSeconds int32) error {
	_, err := BlockingRunCommandWithTimeout([]string{"/mat/ParseHeapDump.sh", path, MemoryAnalyzerLeakSuspectsReportAPI, MemoryAnalyzerSystemOverviewReportAPI, MemoryAnalyzerTopComponentsReportAPI}, timeoutSeconds)
	if err != nil {
		return fmt.Errorf("unable to parse hprof file %s with error %v", path, err)
	}

	return nil
}

// DecompressHPROFFileArchives decompresses result archives from hprof files by executing "unzip" command.
func DecompressHPROFFileArchives(dirname string, fileInfo os.FileInfo, timeoutSeconds int32) (string, string, string, error) {
	leakSuspectsFilePath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerLeakSuspectsSuffix+".zip")
	leakSuspectsDirectoryPath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerLeakSuspectsSuffix)
	err := Unzip(leakSuspectsFilePath, leakSuspectsDirectoryPath, timeoutSeconds)
	if err != nil {
		return "", "", "", err
	}

	systemOverviewFilePath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerSystemOverviewSuffix+".zip")
	systemOverviewDirectoryPath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerSystemOverviewSuffix)
	err = Unzip(systemOverviewFilePath, systemOverviewDirectoryPath, timeoutSeconds)
	if err != nil {
		return "", "", "", err
	}

	topComponentsFilePath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerTopComponentsSuffix+".zip")
	topComponentsDirectoryPath := filepath.Join(dirname, strings.TrimSuffix(fileInfo.Name(), filepath.Ext(fileInfo.Name()))+MemoryAnalyzerTopComponentsSuffix)
	err = Unzip(topComponentsFilePath, topComponentsDirectoryPath, timeoutSeconds)
	if err != nil {
		return "", "", "", err
	}

	return leakSuspectsDirectoryPath, systemOverviewDirectoryPath, topComponentsDirectoryPath, nil
}

// Unzip decompresses a zip archive, moving all files and folders within the zip file to an output directory
// by executing "unzip" command.
// It takes source zip file, destination output directory and timeout seconds as parameters.
func Unzip(src string, dst string, timeoutSeconds int32) error {
	_, err := BlockingRunCommandWithTimeout([]string{"unzip", src, "-d", dst}, timeoutSeconds)
	if err != nil {
		return fmt.Errorf("unzip file %s to %s with error %v", src, dst, err)
	}

	return nil
}

// BlockingRunCommandWithTimeout executes command in blocking mode with timeout seconds.
func BlockingRunCommandWithTimeout(command []string, timeoutSeconds int32) ([]byte, error) {
	timeoutCommand := []string{"timeout", fmt.Sprintf("%ds", timeoutSeconds)}
	timeoutCommand = append(timeoutCommand, command...)
	out, err := exec.Command(timeoutCommand[0], timeoutCommand[1:]...).CombinedOutput()
	if err != nil {
		return out, err
	}

	return out, nil
}

// GetAvailablePort returns a free open port that is ready to use.
func GetAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// StringToNamespacedName converts a string to NamespacedName.
func StringToNamespacedName(s string) (types.NamespacedName, error) {
	ss := strings.Split(s, string(types.Separator))
	if len(ss) != 2 {
		return types.NamespacedName{}, fmt.Errorf("invalid namespaced name string %s", s)
	}

	return types.NamespacedName{
		Namespace: ss[0],
		Name:      ss[1],
	}, nil
}

// ComputeHash returns a hash value calculated from a template. The hash will be safe encoded to avoid bad words.
func ComputeHash(template interface{}) string {
	hasher := fnv.New32a()
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", template)

	return SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// SafeEncodeString encodes s using the same characters as rand.String. This reduces the chances of bad words and
// ensures that strings generated from hash functions appear consistent throughout the API.
func SafeEncodeString(s string) string {
	r := make([]byte, len(s))
	for i, b := range []rune(s) {
		r[i] = AlphaNums[(int(b) % len(AlphaNums))]
	}
	return string(r)
}
