package networking

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	types2 "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-diagnoser/kube-diagnoser/pkg/processors"
)

// serviceNetworkInfoCollector manages network information of container on the node.
type serviceNetworkInfoCollector struct {
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

// NewServiceNetworkInfoCollector creates a new serviceNetworkInfoCollector.
func NewServiceNetworkInfoCollector(
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

	return &serviceNetworkInfoCollector{
		Context:                     ctx,
		Logger:                      logger,
		cache:                       cache,
		client:                      cli,
		networkInfoCollectorEnabled: networkInfoCollectorEnabled,
	}, nil
}

// Handler handles http requests for network information.
func (nic *serviceNetworkInfoCollector) Handler(w http.ResponseWriter, r *http.Request) {
	if !nic.networkInfoCollectorEnabled {
		http.Error(w, fmt.Sprintf("network info collector is not enabled"), http.StatusUnprocessableEntity)
		return
	}
	switch r.Method {
	case http.MethodPost:
		nic.Info("handle POST request")
		// read request body and unmarshal into a CoreFileConfig
		contexts, err := processors.ExtractParametersFromHTTPContext(r)
		if err != nil {
			nic.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		trouble := &networkingDescription{}
		if contexts[networkAbnormalDescription] == "" {
			http.Error(w, fmt.Sprintf("processor can not get '%s' from context", networkAbnormalDescription), http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal([]byte(contexts[networkAbnormalDescription]), trouble)
		if err != nil {
			nic.Error(err, "unmarshal parameter in data failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if trouble.Src == nil || trouble.Dst == nil {
			http.Error(w, fmt.Sprintf("incompleted abnormal description: %+v", trouble), http.StatusInternalServerError)
			return
		}
		if trouble.Dst.ServiceNamespacedName == "" {
			http.Error(w, fmt.Sprintf("can not support diagnosis networking trouble with no-service dst: %+v", *trouble.Src), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		// 1. get svc and endpoints info
		// 2. get resolv.conf in client container/node
		// 3. get iptables on client node
		// 4. try connect in client container/node
		subStrs := strings.Split(trouble.Dst.ServiceNamespacedName, `/`)
		service := &v1.Service{}
		err = nic.cache.Get(nic.Context, client.ObjectKey{
			Namespace: subStrs[0],
			Name:      subStrs[1],
		}, service)
		if err != nil {
			http.Error(w, fmt.Sprintf("get service: %s failed, %s", trouble.Dst.ServiceNamespacedName, err.Error()), http.StatusInternalServerError)
			return
		}
		service.ManagedFields = nil
		serviceJson, err := json.Marshal(service)
		if err != nil {
			http.Error(w, fmt.Sprintf("marshal service: %s failed, %s", trouble.Dst.ServiceNamespacedName, err.Error()), http.StatusInternalServerError)
			return
		}
		result[serviceNetworkInfoService] = string(serviceJson)

		endpoints := &v1.Endpoints{}
		err = nic.cache.Get(nic.Context, client.ObjectKey{
			Namespace: subStrs[0],
			Name:      subStrs[1],
		}, endpoints)
		if err != nil {
			http.Error(w, fmt.Sprintf("get endpoints: %s failed, %s", trouble.Dst.ServiceNamespacedName, err.Error()), http.StatusInternalServerError)
			return
		}
		endpoints.ManagedFields = nil
		endpointsJson, err := json.Marshal(endpoints)
		if err != nil {
			http.Error(w, fmt.Sprintf("marshal endpoints: %s failed, %s", trouble.Dst.ServiceNamespacedName, err.Error()), http.StatusInternalServerError)
			return
		}
		result[serviceNetworkInfoEndpoints] = string(endpointsJson)

		var resolvData []byte
		pid := 1
		switch {
		case trouble.Src.PodNamespacedName != "":
			subStr := strings.Split(trouble.Src.PodNamespacedName, `/`)
			if len(subStr) != 2 {
				http.Error(w, fmt.Sprintf("pod name is illegal: %s", trouble.Src.PodNamespacedName), http.StatusInternalServerError)
				return
			}
			podNamespace := subStr[0]
			podName := subStr[1]
			var userCID string
			userCID, _, pid, err = nic.getFilteredContainerInfo(podNamespace, podName)
			if err != nil {
				http.Error(w, fmt.Sprintf("get container info failed, %s", err.Error()), http.StatusInternalServerError)
				return
			}
			resolvData, err = nic.readFileFromContainer(userCID, resolvConfigPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("get resolv config file from container failed, %s", err.Error()), http.StatusInternalServerError)
				return
			}
		case isNodeIP(trouble.Src.ipNet):
			resolvData, err = ioutil.ReadFile(resolvConfigPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("read node resolv.conf failed, %s", err.Error()), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, fmt.Sprintf("can not diagnosis trouble from endpoint outside the k8s cluster"), http.StatusInternalServerError)
			return
		}
		result[serviceNetworkInfoResolvConfig] = string(resolvData)

		telnetServiceResult, telnetEndpointsResult, err := nic.doServiceTelnet(service, endpoints, pid)
		if err != nil {
			http.Error(w, fmt.Sprintf("doServiceTelnet failed, %s", err.Error()), http.StatusInternalServerError)
			return
		}
		result[serviceNetworkInfoTelnetService] = telnetServiceResult
		result[serviceNetworkInfoTelnetEndpoints] = telnetEndpointsResult

		iptData, err := exec.Command("iptables-save").CombinedOutput()
		if err != nil {
			http.Error(w, fmt.Sprintf("dump iptables on node failed, %s", err.Error()), http.StatusInternalServerError)
			return
		}
		result[serviceNetworkInfoNodeIPTables] = string(iptData)

		if pid > 1 {
			iptData, err := execInNS("iptables-save", pid)
			if err != nil {
				http.Error(w, fmt.Sprintf("dump iptables in pod failed, %s", err.Error()), http.StatusInternalServerError)
				return
			}
			result[serviceNetworkInfoPodIPTables] = iptData
		}

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

func (nic *serviceNetworkInfoCollector) getFilteredContainerInfo(namespace, podName string) (string, string, int, error) {
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("io.kubernetes.pod.name=%s", podName))
	filters.Add("label", fmt.Sprintf("io.kubernetes.pod.namespace=%s", namespace))
	containers, err := nic.client.ContainerList(nic.Context, types2.ContainerListOptions{
		All:     false,
		Filters: filters,
	})
	if err != nil {
		return "", "", -1, err
	}
	nic.Info("list containers", "containers", containers)
	var userContainerID, sandboxID string
	for _, c := range containers {
		if c.Labels["io.kubernetes.container.name"] == "POD" {
			sandboxID = c.ID
			continue
		}
		userContainerID = c.ID
	}
	if sandboxID == "" || userContainerID == "" {
		return userContainerID, sandboxID, -1, fmt.Errorf("can not find matched containerid. user-contaienr: %s, sandbox: %s", userContainerID, sandboxID)
	}
	containerJson, err := nic.client.ContainerInspect(nic.Context, sandboxID)
	if err != nil {
		return userContainerID, sandboxID, -1, err
	}
	return userContainerID, sandboxID, containerJson.State.Pid, err
}

func (nic *serviceNetworkInfoCollector) findSandboxPidFromContainers(containers []types2.Container) (int, error) {
	for _, c := range containers {
		if c.Labels["io.kubernetes.container.name"] == "POD" {
			containerJson, err := nic.client.ContainerInspect(nic.Context, c.ID)
			if err != nil {
				return -1, err
			}
			return containerJson.State.Pid, nil
		}
	}
	return -1, fmt.Errorf("user contianer not found")
}

func (nic *serviceNetworkInfoCollector) readFileFromContainer(containerID, filePath string) ([]byte, error) {
	reader, _, err := nic.client.CopyFromContainer(nic.Context, containerID, filePath)
	fileName := path.Base(filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	tr := tar.NewReader(reader)
	var b []byte
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if hdr.Name == fileName {
			b, err = ioutil.ReadAll(tr)
			break
		}
		if err != nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (nic *serviceNetworkInfoCollector) doServiceTelnet(svc *v1.Service, eps *v1.Endpoints, pid int) (string, string, error) {
	netns, err := ns.GetNS(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return "", "", err
	}
	defer netns.Close()
	var telnetServiceResult, telnetEndpointResult string
	err = netns.Do(func(netNS ns.NetNS) error {
		for _, port := range svc.Spec.Ports {
			nic.Info(fmt.Sprintf("telnet: %v://%s:%d", port.Protocol, svc.Spec.ClusterIP, int(port.Port)))
			subResult := doTelnet(svc.Spec.ClusterIP, int(port.Port), port.Protocol)
			nic.Info(fmt.Sprintf("telnet: %v://%s:%d", port.Protocol, svc.Spec.ClusterIP, int(port.Port)), "result", subResult)
			telnetServiceResult = fmt.Sprintf("%s%s\n", telnetServiceResult, subResult)
		}
		for _, ep := range eps.Subsets {
			for _, port := range ep.Ports {
				for _, addr := range ep.Addresses {
					nic.Info(fmt.Sprintf("telnet: %v://%s:%d", port.Protocol, addr.IP, int(port.Port)))
					subResult := doTelnet(addr.IP, int(port.Port), port.Protocol)
					nic.Info(fmt.Sprintf("telnet: %v://%s:%d", port.Protocol, addr.IP, int(port.Port)), "result", subResult)
					telnetEndpointResult = fmt.Sprintf("%s%s\n", telnetEndpointResult, subResult)
				}
			}
		}
		return nil
	})
	return telnetServiceResult, telnetEndpointResult, err
}

func execInNS(command string, pid int) (string, error) {
	netns, err := ns.GetNS(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return "", err
	}
	defer netns.Close()
	var result string
	err = netns.Do(func(netNS ns.NetNS) error {
		data, err := exec.Command(command).CombinedOutput()
		if err != nil {
			return err
		}
		result = string(data)
		return nil
	})
	return result, err
}

func doTelnet(addr string, port int, protocol v1.Protocol) string {
	switch protocol {
	case v1.ProtocolUDP:
		udpAddr := &net.UDPAddr{
			IP:   net.ParseIP(addr),
			Port: port,
		}
		udpConn, err := net.DialUDP("udp4", nil, udpAddr)
		if err != nil {
			return fmt.Sprintf("telnet %v: %s:%d failed: %s", protocol, addr, port, err.Error())
		}
		defer udpConn.Close()
		return fmt.Sprintf("telnet %v: %s:%d succeed", protocol, addr, port)
	case v1.ProtocolTCP:
		fallthrough
	default:
		tcpAddr := &net.TCPAddr{
			IP:   net.ParseIP(addr),
			Port: port,
		}
		tcpConn, err := net.DialTimeout("tcp", tcpAddr.String(), time.Second*5)
		if err != nil {
			return fmt.Sprintf("telnet %v: %s:%d failed: %s", protocol, addr, port, err.Error())
		}
		defer tcpConn.Close()
		return fmt.Sprintf("telnet %v: %s:%d succeed", protocol, addr, port)
	}
}

func isNodeIP(ipNet *net.IPNet) bool {
	if ipNet == nil {
		return false
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if strings.Split(addr.String(), `/`)[0] == ipNet.IP.String() {
			return true
		}
	}
	return false
}
