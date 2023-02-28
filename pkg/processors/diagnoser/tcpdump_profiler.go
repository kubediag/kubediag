/*
Copyright 2022 The KubeDiag Authors.

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

package diagnoser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/go-logr/logr"
	wsd "github.com/joewalnes/websocketd/libwebsocketd"
	"github.com/kubediag/kubediag/pkg/executor"
	"github.com/kubediag/kubediag/pkg/processors/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubediag/kubediag/pkg/processors"
)

type TcpdumpProfilerType string

const (
	ParameterKeyTcpdumpProfilerExpirationSeconds = "param.diagnoser.runtime.tcpdump_profiler.expiration_seconds"
	ParameterKeyTcpdumpProfilerContainer         = "param.diagnoser.runtime.tcpdump_profiler.container"
	ParameterKeyTcpdumpProfilerInterface         = "param.diagnoser.runtime.tcpdump_profiler.interface"
	ParameterKeyTcpdumpProfilerFilter            = "param.diagnoser.runtime.tcpdump_profiler.filter"

	ContextKeyTcpdumpProfilerResultEndpoint = "diagnoser.runtime.tcpdump_profiler.result.Endpoint"
)

var (
	// typeHost means network debugging on specified node
	typeHost TcpdumpProfilerType = "Host"
	// typePod means network debugging on specified pod
	typePod TcpdumpProfilerType = "Pod"
)

// tcpdumpProfilerConfig contains some configuration required by  tcpDumpProfiler.
type tcpdumpProfilerConfig struct {
	// Type is the type of tcpdump profiler.There are two possible types values:
	//
	// Host: The profiler will support network debugging on specified node
	// Pod: The profiler will support network debugging on specified pod
	TcpdumpProfilerType TcpdumpProfilerType `json:"type"`
	// ExpirationSeconds is the life time of tcpDumpHTTPServer.
	ExpirationSeconds uint64 `json:"expirationSeconds"`
	// Container is the name of specified container.
	Container string `json:"container"`
	// ContainerId is the id of specified container.
	ContainerId string `json:"containerId"`
	// Interface is the interface tcpdump listens on.
	Interface string `json:"interface"`
	// Filter is the given expression to select which packets will be dumped.
	Filter string `json:"filter"`
}

// tcpdumpProfiler will manage pods' or nodes' tcpdump information and provide a websoccket to do gdb online.
type tcpdumpProfiler struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// client is the API client that performs all operations against a docker server.
	client *dockerclient.Client
	// DataRoot is root directory of persistent kubediag data.
	dataRoot string
	// cache knows how to load Kubernetes objects.
	cache cache.Cache
	// tcpdumProfilerEnabled indicates whether tcpdumProfiler is enabled.
	tcpdumpProfilerEnabled bool
}

// NewTcpdumpProfiler creates a new tcpDumpProfiler.
func NewTcpdumpProfiler(
	ctx context.Context,
	logger logr.Logger,
	dockerEndpoint string,
	cache cache.Cache,
	dataRoot string,
	tcpdumpProfilerEnabled bool,
) (processors.Processor, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(dockerEndpoint))
	if err != nil {
		return nil, err
	}
	return &tcpdumpProfiler{
		Context:                ctx,
		Logger:                 logger,
		client:                 client,
		dataRoot:               dataRoot,
		cache:                  cache,
		tcpdumpProfilerEnabled: tcpdumpProfilerEnabled,
	}, nil
}

// Handler handles http requests for tcpdumpProfier.
func (t *tcpdumpProfiler) Handler(w http.ResponseWriter, r *http.Request) {
	if !t.tcpdumpProfilerEnabled {
		http.Error(w, "Tcpdump profiler is not enabled", http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			t.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var expirationSeconds int
		if _, ok := contexts[ParameterKeyTcpdumpProfilerExpirationSeconds]; !ok {
			expirationSeconds = processors.DefaultExpirationSeconds
		} else {
			expirationSeconds, err = strconv.Atoi(contexts[ParameterKeyTcpdumpProfilerExpirationSeconds])
			if err != nil {
				t.Error(err, "invalid expirationSeconds field")
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if expirationSeconds <= 0 {
				expirationSeconds = processors.DefaultExpirationSeconds
			}
		}

		// Init configuration required by tcpDumpProfiler
		config := &tcpdumpProfilerConfig{
			ExpirationSeconds: uint64(expirationSeconds),
			Container:         string(contexts[ParameterKeyTcpdumpProfilerContainer]),
			Interface:         string(contexts[ParameterKeyTcpdumpProfilerInterface]),
			Filter:            string(contexts[ParameterKeyTcpdumpProfilerFilter]),
		}

		podReference := utils.GetPodInfoFromContext(contexts)
		if podReference.Name == "" {
			config.TcpdumpProfilerType = typeHost
		} else {
			config.TcpdumpProfilerType = typePod
		}

		port, err := utils.GetAvailablePort()
		if err != nil {
			t.Logger.Error(err, "get available port failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mux := http.NewServeMux()
		var server *http.Server

		if config.TcpdumpProfilerType == typePod {
			// Case network debugging on specified pod.
			// Get extra information about target container in pod.
			pod := corev1.Pod{}
			err = t.cache.Get(t.Context,
				crtclient.ObjectKey{
					Namespace: contexts[executor.PodNamespaceTelemetryKey],
					Name:      contexts[executor.PodNameTelemetryKey],
				}, &pod)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to get pod: %v", err), http.StatusInternalServerError)
				return
			}

			if config.Container == "" {
				t.Logger.Info("container name unspecified. Select the first container as default", "container name", pod.Spec.Containers[0].Name)
				config.Container = pod.Spec.Containers[0].Name
			}
			config.ContainerId, err = getContainerId(pod, config.Container)
			if err != nil {
				t.Logger.Error(err, "unknown container name %s in pod %s", config.Container, pod.Name)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if config.TcpdumpProfilerType != typeHost {
			err = fmt.Errorf("unknown TcpDumpProfilerType: %s", config.TcpdumpProfilerType)
			t.Logger.Error(err, "failed to build tcpdump http server")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Build tcpdump HTTP server to run tcpdump.
		server, err = t.buildTcpdumpHTTPServer(config, contexts[executor.NodeTelemetryKey], port, mux, w, r)
		if err != nil {
			t.Logger.Error(err, "failed to build tcp dump http server")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			err := server.ListenAndServe()
			if err == http.ErrServerClosed {
				t.Info("tcp dump http server closed")
			} else if err != nil {
				t.Error(err, "failed to start tcp dump http server")
			}
		}()

		// Shutdown http server with expiration duration.
		go func() {
			select {
			// Wait for http server error.
			case <-ctx.Done():
				return
			// Wait for expiration.
			case <-time.After(time.Duration(config.ExpirationSeconds) * time.Second):
				err := server.Shutdown(ctx)
				if err != nil {
					t.Error(err, "failed to shutdown tcpdump http server")
				}
			}
		}()

		result := make(map[string]string)
		result[ContextKeyTcpdumpProfilerResultEndpoint] = fmt.Sprintf("http://%s:%d", contexts[executor.NodeTelemetryKey], port)
		data, err := json.Marshal(result)
		if err != nil {
			t.Error(err, "failed to marshal response body")
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// buildTcpdumpCommand builds tcpdump command with parameters in tcpDumpProfilerConfig.
func buildTcpdumpCommand(config *tcpdumpProfilerConfig) ([]string, error) {
	if config.Interface == "" {
		config.Interface = "any"
	}
	tcpdumpConfig := []string{"tcpdump", "-i", config.Interface, config.Filter}

	return tcpdumpConfig, nil
}

// getContainerId gets container id of specified container in pod by given container name.
func getContainerId(pod corev1.Pod, containerName string) (string, error) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerName == containerStatus.Name {
			result := strings.Split(containerStatus.ContainerID, "://")
			if len(result) != 2 {
				break
			}
			return result[1], nil
		}
	}

	return "", errors.Errorf("failed to find container '%s' in pod '%s'", containerName, pod.Name)
}

// getContainerPid returns the container pid.
func (t *tcpdumpProfiler) getContainerPid(containerId string) (string, error) {
	t.Info("Inspecting container on node with containerId.")
	t.client.NegotiateAPIVersion(t)
	containerInfo, err := t.client.ContainerInspect(t, containerId)
	if err != nil {
		return "", err
	}

	pid := containerInfo.State.Pid
	return fmt.Sprint(pid), nil
}

// buildTcpdumpHTTPServer will build a HTTP server to provide tcpdump.
func (t *tcpdumpProfiler) buildTcpdumpHTTPServer(config *tcpdumpProfilerConfig, node string, port int, serveMux *http.ServeMux, w http.ResponseWriter, r *http.Request) (*http.Server, error) {
	var err error
	pid := "1"
	if config.TcpdumpProfilerType == typePod {
		t.Info("Start collecting Pid of container.")
		pid, err = t.getContainerPid(config.ContainerId)
		if err != nil {
			t.Error(err, "get container pid failed")
			return nil, err
		}
	}
	nsenterCommand := []string{"nsenter", "-t", pid, "-n"}
	exec.Command(nsenterCommand[0], nsenterCommand[1:]...).Run()

	tcpdumpCommand, _ := buildTcpdumpCommand(config)
	wsServer := buildWebsocketHandler(tcpdumpCommand[0], tcpdumpCommand[1:])

	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wsServer.ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: serveMux,
	}, nil
}

// buildGDBWebsocketHandler will build a websocket handler which execute tcpdump command.
func buildWebsocketHandler(commandName string, commandArgs []string) *wsd.WebsocketdServer {
	wsConfig := &wsd.Config{
		CommandName: commandName,
		DevConsole:  true,
		CommandArgs: commandArgs,
	}
	webSocketHandler := wsd.NewWebsocketdServer(wsConfig, wsd.RootLogScope(0, emptyLogFunc), 6)
	return webSocketHandler
}
