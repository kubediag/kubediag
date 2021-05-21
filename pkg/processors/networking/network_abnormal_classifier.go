package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-diagnoser/kube-diagnoser/pkg/processors"
)

// networkClassifier manages network information of container on the node.
type networkClassifier struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// client is the API client that performs all operations against a docker server.
	client *dockerclient.Client
	// networkInfoCollectorEnabled indicates whether serviceNetworkInfoCollector is enabled.
	networkInfoCollectorEnabled bool
}

// NewNetworkClassifier creates a new networkClassifier.
func NewNetworkClassifier(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	dockerEndpoint string,
	networkInfoCollectorEnabled bool,
) (processors.Processor, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}

	return &networkClassifier{
		Context:                     ctx,
		Logger:                      logger,
		cache:                       cache,
		client:                      cli,
		networkInfoCollectorEnabled: networkInfoCollectorEnabled,
	}, nil
}

// Handler handles http requests for network information.
func (nc *networkClassifier) Handler(w http.ResponseWriter, r *http.Request) {
	if !nc.networkInfoCollectorEnabled {
		http.Error(w, fmt.Sprintf("network info collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}
	switch r.Method {
	case http.MethodPost:
		nc.Info("handle POST request")
		contexts, err := processors.ExtractParametersFromHTTPContext(r)
		if err != nil {
			nc.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		abnormalDesc := &networkingDescription{}
		err = json.Unmarshal([]byte(contexts[networkAbnormalClassifierParamDescription]), abnormalDesc)
		if err != nil {
			nc.Error(err, "unmarshal description in post raw failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if abnormalDesc.Src == nil || abnormalDesc.Dst == nil {
			http.Error(w, fmt.Sprintf("abnormal description is incompleted: %+v", *abnormalDesc), http.StatusInternalServerError)
			return
		}
		// classify networking abnormal into some types and complete it
		// 1. pod to pod
		// 2. pod to service(include nodeport)
		// 3. node to service(include nodeport)
		// 4. out-of-cluster to nodeport service
		err = nc.completeEndpoints(abnormalDesc)
		if err != nil {
			http.Error(w, fmt.Sprintf("complete abnormalDesc description failed: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		raw, err := json.Marshal(abnormalDesc)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal network info: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[networkAbnormalDescription] = string(raw)
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal result: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

func (nc *networkClassifier) completeEndpoints(description *networkingDescription) error {
	podList := v1.PodList{}
	err := nc.cache.List(nc.Context, &podList, &client.ListOptions{})
	if err != nil {
		return err
	}
	svcList := v1.ServiceList{}
	err = nc.cache.List(nc.Context, &svcList, &client.ListOptions{})
	if err != nil {
		return err
	}
	nodeList := v1.NodeList{}
	err = nc.cache.List(nc.Context, &nodeList, &client.ListOptions{})
	if err != nil {
		return err
	}

	if description.Src.PodNamespacedName == "" {
		// inject pod info if match
		if !strings.Contains(description.Src.IPNet, `/`) {
			description.Src.IPNet = fmt.Sprintf("%s/32", description.Src.IPNet)
		}
		_, description.Src.ipNet, err = net.ParseCIDR(description.Src.IPNet)
		if err != nil {
			return fmt.Errorf("src should be a pod or an ip address: %s", err.Error())
		}

		srcIP := description.Src.ipNet.IP.String()
		for _, pod := range podList.Items {
			if pod.Status.PodIP == srcIP {
				description.Src.PodNamespacedName = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
				break
			}
		}
	}
	if description.Dst.PodNamespacedName == "" && description.Dst.ServiceNamespacedName == "" {
		// inject pod/service info if match
		if !strings.Contains(description.Dst.IPNet, `/`) {
			description.Dst.IPNet = fmt.Sprintf("%s/32", description.Dst.IPNet)
		}
		_, description.Dst.ipNet, err = net.ParseCIDR(description.Dst.IPNet)
		if err != nil {
			return fmt.Errorf("dst should be a pod or a service or an ip address: %s", err.Error())
		}
		dstIP := description.Dst.ipNet.IP.String()
		dstPort32 := int32(description.Dst.Port)
		for _, pod := range podList.Items {
			if pod.Status.PodIP == dstIP {
				description.Dst.PodNamespacedName = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
				return nil
			}
		}
		for _, svc := range svcList.Items {
			if svc.Spec.ClusterIP == dstIP {
				description.Dst.ServiceNamespacedName = fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
				return nil
			}
			if svc.Spec.Type == v1.ServiceTypeNodePort && containsNodePort(svc.Spec.Ports, dstPort32) {
				description.Dst.ServiceNamespacedName = fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
				return nil
			}
		}
	}
	return nil
}

func containsNodePort(ports []v1.ServicePort, targetPort int32) bool {
	for _, portRef := range ports {
		if portRef.NodePort == targetPort && targetPort != 0 {
			return true
		}
	}
	return false
}

type endpoint struct {
	ipNet                 *net.IPNet
	IPNet                 string `json:"ipNet,omitempty"`
	Port                  int    `json:"port,omitempty"`
	PodNamespacedName     string `json:"podName,omitempty"`
	ServiceNamespacedName string `json:"serviceName,omitempty"`
}

type networkingDescription struct {
	Src *endpoint `json:"src"`
	Dst *endpoint `json:"dst"`
}
