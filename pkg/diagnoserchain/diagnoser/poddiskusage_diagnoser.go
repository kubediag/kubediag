package diagnoser

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// PodDiskUsageDiagnoser manages diagnosis that finding disk usage of pods.
type PodDiskUsageDiagnoser interface {
	Handler(http.ResponseWriter, *http.Request)
	ListPods(diagnosisv1.Abnormal) ([]corev1.Pod, error)
}

// podDiskUsageDiagnoserImpl implements PodDiskUsageDiagnoser interface.
type podDiskUsageDiagnoserImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// Log represents the ability to log messages.
	Log logr.Logger
}

// PodDiskUsageList contains disk usage information of pods.
// It satisfies sort.Interface by implemeting the following methods:
//
// Len() int
// Less(i, j int) bool
// Swap(i, j int)
type PodDiskUsageList []PodDiskUsage

// PodDiskUsage contains disk usage information of a pod.
type PodDiskUsage struct {
	// ObjectMeta is metadata of the pod.
	metav1.ObjectMeta `json:"metadata"`
	// DiskUsage is the disk usage of the pod in bytes.
	DiskUsage int `json:"diskUsage"`
	// Path is the pod data path.
	Path string `json:"path"`
}

// NewPodDiskUsageDiagnoser creates a new PodDiskUsageDiagnoser.
func NewPodDiskUsageDiagnoser(
	ctx context.Context,
	log logr.Logger,
) PodDiskUsageDiagnoser {
	return &podDiskUsageDiagnoserImpl{
		Context: ctx,
		Log:     log,
	}
}

// Handler handles http requests for diagnosing pod disk usage.
func (pd *podDiskUsageDiagnoserImpl) Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var abnormal diagnosisv1.Abnormal
		err = json.Unmarshal(body, &abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to unmarshal request body into an abnormal: %v", err), http.StatusNotAcceptable)
			return
		}

		// List all pods on the node.
		pods, err := pd.ListPods(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list pods: %v", err), http.StatusInternalServerError)
			return
		}

		// List 10 pods with the most disk usage in descending order.
		sorted := make(PodDiskUsageList, 0, len(pods))
		for _, pod := range pods {
			// Get pod data path with kubelet pod directory and uid.
			path := filepath.Join(util.KubeletPodDirectory, string(pod.UID))
			diskUsage, err := util.DiskUsage(path)
			if err != nil {
				pd.Log.Error(err, "failed to get pod disk usage")
			}
			podDiskUsage := PodDiskUsage{
				ObjectMeta: pod.ObjectMeta,
				DiskUsage:  diskUsage,
				Path:       path,
			}
			sorted = append(sorted, podDiskUsage)
		}
		sort.Sort(sort.Reverse(sorted))
		if len(sorted) > 10 {
			sorted = sorted[:10]
		}

		// Remove pod information in status context.
		abnormal, removed, err := util.RemoveAbnormalContext(abnormal, util.PodInformationContextKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
			return
		}
		if !removed {
			http.Error(w, fmt.Sprintf("failed to remove context field: %v", err), http.StatusInternalServerError)
			return
		}

		// Set pod disk usage diagnosis result in status context.
		abnormal, err = util.SetAbnormalContext(abnormal, util.PodDiskUsageDiagnosisContextKey, sorted)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal abnormal: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// ListPods lists all pods on the node by retrieving information in abnormal.
func (pd *podDiskUsageDiagnoserImpl) ListPods(abnormal diagnosisv1.Abnormal) ([]corev1.Pod, error) {
	pd.Log.Info("listing pods")
	data, err := util.GetAbnormalContext(abnormal, util.PodInformationContextKey)
	if err != nil {
		return nil, err
	}

	var pods []corev1.Pod
	err = json.Unmarshal(data, &pods)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

// Len is the number of elements in PodDiskUsageList.
func (pl PodDiskUsageList) Len() int {
	return len(pl)
}

// Less reports whether the element with index i should sort before the element with index j.
func (pl PodDiskUsageList) Less(i, j int) bool {
	if i > len(pl) || j > len(pl) {
		return false
	}

	return pl[i].DiskUsage < pl[j].DiskUsage
}

// Swap swaps the elements with indexes i and j.
func (pl PodDiskUsageList) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}
