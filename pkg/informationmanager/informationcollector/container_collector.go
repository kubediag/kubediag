package informationcollector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/util"
)

// ContainerCollector manages information of all containers on the node.
type ContainerCollector interface {
	Handler(http.ResponseWriter, *http.Request)
	ListContainers() ([]types.Container, error)
}

// containerCollectorImpl implements ContainerCollector interface.
type containerCollectorImpl struct {
	// Context carries values across API boundaries.
	Context context.Context
	// The API client that performs all operations against a docker server.
	Client *client.Client
	// Log represents the ability to log messages.
	Log logr.Logger
	// Cache knows how to load Kubernetes objects.
	Cache cache.Cache
}

// NewContainerCollector creates a new ContainerCollector.
func NewContainerCollector(
	ctx context.Context,
	dockerEndpoint string,
	log logr.Logger,
	cache cache.Cache,
) (ContainerCollector, error) {
	cli, err := client.NewClientWithOpts(client.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}

	return &containerCollectorImpl{
		Context: ctx,
		Client:  cli,
		Log:     log,
		Cache:   cache,
	}, nil
}

// Handler handles http requests for container information.
func (cc *containerCollectorImpl) Handler(w http.ResponseWriter, r *http.Request) {
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

		// List all containers on the node.
		containers, err := cc.ListContainers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}

		// Set container information in status context.
		abnormal, err = util.SetAbnormalContext(abnormal, util.ContainerInformationContextKey, containers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to set context field: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(abnormal)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal abnormal: %v", err), http.StatusInternalServerError)
			return
		}

		// Response with error if abnormal data size exceeds max data size.
		if len(data) > util.MaxDataSize {
			http.Error(w, fmt.Sprintf("abnormal data size %d exceeds max data size %d", len(data), util.MaxDataSize), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	case "GET":
		// List all containers on the node.
		containers, err := cc.ListContainers()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list containers: %v", err), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(containers)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal containers: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// ListContainers lists all containers on the node.
func (cc *containerCollectorImpl) ListContainers() ([]types.Container, error) {
	cc.Log.Info("listing containers")

	cc.Client.NegotiateAPIVersion(cc.Context)
	containers, err := cc.Client.ContainerList(cc.Context, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	return containers, nil
}
