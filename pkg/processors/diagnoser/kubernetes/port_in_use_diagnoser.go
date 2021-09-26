package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
)

const (
	timeOutSeconds int32 = 60

	DiagnoserPort = "param.diagnoser.port_in_use.port"

	ContextKeyPortInUseDiagnosisResult   = "diagnoser.kubernetes.port_in_use.result"
	ContextKeyPortInUseService           = "diagnoser.kubernetes.port_in_use.service"
	ContextKeyPortInUseKubeletListenPort = "diagnoser.kubernetes.port_in_use.kubelet_listen_port"
)

// PortInUseDiganoser will diagnosis a bug happens when port is in use (connection reset by peer).
type portInUseDiganoser struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger

	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// portInUseEnabled indicates whether portInUseDiganoser and portInUseRecover is enabled.
	portInUseEnabled bool
}

// NewPortInUseDiagnoser creates a new portInUseDiganoser
func NewPortInUseDiagnoser(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	portInUseEnabled bool,
) processors.Processor {
	return &portInUseDiganoser{
		Context:          ctx,
		Logger:           logger,
		cache:            cache,
		portInUseEnabled: portInUseEnabled,
	}
}

func (pud *portInUseDiganoser) Handler(w http.ResponseWriter, r *http.Request) {
	if !pud.portInUseEnabled {
		http.Error(w, fmt.Sprintf("port in use diagnosis is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			pud.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		diagnoserPort := contexts[DiagnoserPort]
		result := make(map[string]string)

		// List all services on the node.
		services, err := pud.listServices()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list services: #{err}"), http.StatusInternalServerError)
			return
		}

		// condition 1: port is used by a service
		for _, service := range services {
			portInUse := false
			pud.Info("checking service.", "service:", service.Spec.Ports)

			for _, port := range service.Spec.Ports {
				pud.Info("checking ports.", "port:", fmt.Sprint(port.Port), "targetPort:", port.TargetPort.String(), "nodePort:", fmt.Sprint(port.NodePort))
				if fmt.Sprint(port.Port) == diagnoserPort ||
					port.TargetPort.String() == diagnoserPort ||
					fmt.Sprint(port.NodePort) == diagnoserPort {
					pud.Info("match pod in service!", "serviceSpecPorts:", service.Spec.Ports)
					portInUse = true
					break
				}
			}

			if portInUse == true {
				raw, err := json.Marshal(service)
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to marshal service: #{err}"), http.StatusInternalServerError)
					return
				}

				result[ContextKeyPortInUseService] = string(raw)
				break
			}
		}

		// condition 2: kubelet listens the specific port
		message := ""

		pud.Info("Start collecting socket.")
		//out, err := util.BlockingRunCommandWithTimeout([]string{"ss", "-tlnp", "|", "grep", "kubelet"}, timeOutSeconds)

		cmd := "ss -tlnp | grep kubelet"
		out, err := exec.Command("bash", "-c", cmd).CombinedOutput()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute command: %s", cmd), http.StatusInternalServerError)
		}

		if strings.Contains(string(out), diagnoserPort) {
			message = fmt.Sprintf("connection reset by peer has been encountered. listen tcp :%s: bind: address already in use", diagnoserPort)
		}

		result[ContextKeyPortInUseKubeletListenPort] = string(out)
		result[ContextKeyPortInUseDiagnosisResult] = message

		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal result: #{err}"), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Context-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method #{r.Method} is not aupported"), http.StatusMethodNotAllowed)
	}
}

// listServices lists Services from cache.
func (pc *portInUseDiganoser) listServices() ([]corev1.Service, error) {
	pc.Info("listing Services on node")

	var serviceList corev1.ServiceList
	if err := pc.cache.List(pc, &serviceList); err != nil {
		return nil, err
	}

	return serviceList.Items, nil
}
