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

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/executor"
	"github.com/kubediag/kubediag/pkg/processors"
	"github.com/kubediag/kubediag/pkg/processors/utils"
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	ParameterKeyGoProfilerExpirationSeconds  = "param.diagnoser.runtime.go_profiler.expiration_seconds"
	ParameterKeyGoProfilerType               = "param.diagnoser.runtime.go_profiler.type"
	ParameterKeyGoProfilerSource             = "param.diagnoser.runtime.go_profiler.source"
	ParameterKeyGoProfilerTLSSecretNamespace = "param.diagnoser.runtime.go_profiler.tls.secret_reference.namespace"
	ParameterKeyGoProfilerTLSSecretName      = "param.diagnoser.runtime.go_profiler.tls.secret_reference.name"

	ContextKeyGoProfilerResultEndpoint = "diagnoser.runtime.go_profiler.result.endpoint"
)

// goProfiler manages information of all pods on the node.
type goProfiler struct {
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// Cache knows how to load Kubernetes objects.
	cache cache.Cache
	// DataRoot is root directory of persistent kubediag data.
	dataRoot string
	// BindAddress is the address on which to advertise.
	BindAddress string
	// GoProfilerEnabled indicates whether goProfiler is enabled.
	goProfilerEnabled bool
}

// goProfilerRequestParameter specifies the action to perform for profiling a go program.
type goProfilerRequestParameter struct {
	// Type is the type of go profiler.There are three possible types values:
	//
	// Heap: The profiler will be run by heap
	// Profile: The profiler will be run by cpu profile
	// Goroutine: The profiler will be run by goroutine
	GoProfilerType goProfilerType `json:"type"`
	// Source specifies the profile source. It must be a local file path or a URL.
	Source string `json:"source"`

	// Number of seconds after which the profiler endpoint expires.
	// Defaults to 7200 seconds. Minimum value is 1.
	// +optional
	ExpirationSeconds int64 `json:"expirationSeconds,omitempty"`

	// TLS specifies the secret reference for source
	// +optional
	TLS goProfilerTLS `json:"tls,omitempty"`
}

// GoProfilerType is a valid value for GoProfiler.Type.
type goProfilerType string

type goProfilerTLS struct {
	// SecretReference specifies the secret in cluster which hold token and ca.crt to access GoProfiler Source.
	// +optional
	SecretReference diagnosisv1.NamespacedName `json:"secretReference,omitempty"`
}

const (
	// GoroutineGoProfilerType means that the go profiler is run by goroutine.
	goroutineGoProfilerType goProfilerType = "Goroutine"
	// CPUGoProfilerType means that the go profiler is run by cpu.
	cpuGoProfilerType goProfilerType = "Profile"
	// MemoryGoProfilerType means that the go profiler is run by heap.
	heapGoProfilerType goProfilerType = "Heap"
	// GoProfilerPathPrefix is the path prefix for go profiler pprof url.
	goProfilerPathPrefix = "/debug/pprof/"
)

// NewGoProfiler creates a new GoProfiler.
func NewGoProfiler(
	ctx context.Context,
	logger logr.Logger,
	cache cache.Cache,
	dataRoot string,
	bindAddress string,
	goProfilerEnabled bool,
) processors.Processor {
	return &goProfiler{
		Context:           ctx,
		Logger:            logger,
		cache:             cache,
		dataRoot:          dataRoot,
		BindAddress:       bindAddress,
		goProfilerEnabled: goProfilerEnabled,
	}
}

// Handler handles http requests for pod information.
func (gp *goProfiler) Handler(w http.ResponseWriter, r *http.Request) {
	if !gp.goProfilerEnabled {
		http.Error(w, fmt.Sprintf("go profiler is not enabled"), http.StatusUnprocessableEntity)
		return
	}

	switch r.Method {
	case "POST":
		gp.Info("handle POST request")
		// read request body and unmarshal into a goProfilerRequestParameter
		contexts, err := utils.ExtractParametersFromHTTPContext(r)
		if err != nil {
			gp.Error(err, "extract contexts failed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var expirationSeconds int
		if _, ok := contexts[ParameterKeyGoProfilerExpirationSeconds]; !ok {
			expirationSeconds = processors.DefaultExpirationSeconds
		} else {
			expirationSeconds, err = strconv.Atoi(contexts[ParameterKeyGoProfilerExpirationSeconds])
			if err != nil {
				gp.Error(err, "invalid expirationSeconds field")
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if expirationSeconds <= 0 {
				expirationSeconds = processors.DefaultExpirationSeconds
			}
		}

		parameter := goProfilerRequestParameter{
			GoProfilerType:    goProfilerType(contexts[ParameterKeyGoProfilerType]),
			Source:            contexts[ParameterKeyGoProfilerSource],
			ExpirationSeconds: int64(expirationSeconds),
			TLS: goProfilerTLS{
				SecretReference: diagnosisv1.NamespacedName{
					Namespace: contexts[ParameterKeyGoProfilerTLSSecretNamespace],
					Name:      contexts[ParameterKeyGoProfilerTLSSecretName],
				},
			},
		}

		if parameter.GoProfilerType != cpuGoProfilerType && parameter.GoProfilerType != goroutineGoProfilerType &&
			parameter.GoProfilerType != heapGoProfilerType {
			http.Error(w, fmt.Sprintf("Go profiler type must be Profile, Heap or Goroutine."), http.StatusNotAcceptable)
			return
		}
		if parameter.Source == "" {
			http.Error(w, fmt.Sprintf("Must specify go profiler source."), http.StatusNotAcceptable)
			return

		}
		if strings.HasPrefix(parameter.Source, "https") {
			if parameter.TLS.SecretReference.Name == "" {
				http.Error(w, fmt.Sprintf("Must specify secretReference name when souce is https."), http.StatusNotAcceptable)
				return
			}
		}

		namespace := contexts[executor.DiagnosisNamespaceTelemetryKey]
		name := contexts[executor.DiagnosisNameTelemetryKey]
		podInfo := utils.GetPodInfoFromContext(contexts)

		endpoint, err := gp.runGoProfiler(name, namespace, gp.BindAddress, parameter, &podInfo, gp.dataRoot)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to run go profiler: %v", err), http.StatusInternalServerError)
			return
		}

		result := make(map[string]string)
		result[ContextKeyGoProfilerResultEndpoint] = endpoint
		data, err := json.Marshal(result)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to marshal go profiler results: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}

// RunGoProfiler runs go profiling with timeout.
func (gp *goProfiler) runGoProfiler(name string, namespace string, bindAddress string, parameter goProfilerRequestParameter, podReference *diagnosisv1.PodReference, dataRoot string) (string, error) {
	gp.Info("Start to run go profiling", "diagnosis", client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	})

	port, err := util.GetAvailablePort()
	if err != nil {
		return "", err
	}
	endpoint := fmt.Sprintf("%s:%d", bindAddress, port)

	// Add timeout seconds for cpu profile
	timeout := time.Duration(5) * time.Second
	source := fmt.Sprintf("%s%s%s", parameter.Source, goProfilerPathPrefix, strings.ToLower(string(parameter.GoProfilerType)))
	if parameter.GoProfilerType == cpuGoProfilerType {
		timeout += time.Duration(30) * time.Second
		source = fmt.Sprintf("%s?timeout=30s", source)
	}

	// Set go profiler directory
	now := time.Now().Format("20060102150405")
	datadir := filepath.Join(dataRoot, "profilers/go/pprof")
	if podReference == nil || podReference.Name == "" {
		datadir = filepath.Join(datadir, now)
	} else {
		datadir = filepath.Join(datadir, podReference.Namespace+"."+podReference.Name+"."+podReference.Container, now)
	}

	if _, err := os.Stat(datadir); os.IsNotExist(err) {
		err := os.MkdirAll(datadir, os.ModePerm)
		if err != nil {
			return "", err
		}
		gp.Info("Make dir succeessfully.", "datadir", datadir)
	}

	// Set go profiler file name
	datafile := fmt.Sprintf("%s.%s.%s.prof", namespace, name, parameter.GoProfilerType)

	// HTTPS source
	if strings.HasPrefix(parameter.Source, "https") {
		name := parameter.TLS.SecretReference.Name
		namespace := parameter.TLS.SecretReference.Namespace
		if namespace == "" {
			namespace = metav1.NamespaceDefault
		}

		secretData, err := GetSecretData(gp.cache, name, namespace)
		if err != nil {
			return "", err
		}

		tokenByte, ok := secretData[corev1.ServiceAccountTokenKey]
		if !ok {
			return "", fmt.Errorf("secret token is not specified")

		}
		caByte, ok := secretData[corev1.ServiceAccountRootCAKey]
		if !ok {
			return "", fmt.Errorf("secret ca.crt is not specified")
		}

		err = DownloadProfileFile(fmt.Sprintf("%s/%s", datadir, datafile), source, tokenByte, caByte, timeout)
		if err != nil {
			return "", fmt.Errorf("download file failed with error: %s", err)
		}
		gp.Info("Save go profiler file successfully.", "path", fmt.Sprintf("%s/%s", datadir, datafile))
	} else {
		err = DownloadProfileFile(fmt.Sprintf("%s/%s", datadir, datafile), source, nil, nil, timeout)
		if err != nil {
			return "", fmt.Errorf("download file failed with error: %s", err)
		}
		gp.Info("Save go profiler file successfully.", "path", fmt.Sprintf("%s/%s", datadir, datafile))
	}
	gp.Info("Start to execute command.")
	var buf bytes.Buffer
	command := exec.Command("go", "tool", "pprof", "-no_browser", fmt.Sprintf("-http=%s", endpoint), fmt.Sprintf("%s/%s", datadir, datafile))
	// Setting a new process group id to avoid suicide.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout = &buf
	command.Stderr = &buf
	err = command.Start()
	if err != nil {
		return "", err
	}
	gp.Info("Command executed succeessfully.")

	ctx, cancel := context.WithCancel(context.Background())

	// Start go profiler.
	exit := make(chan error)
	go func() {
		defer cancel()
		exit <- command.Wait()
	}()

	// Shutdown go profiler with expiration duration.
	go func() {
		select {
		// Wait for go profiler error.
		case <-ctx.Done():
			return
		// Wait for expiration and shutdown go profiler http server by killing the profiler process.
		case <-time.After(time.Duration(parameter.ExpirationSeconds) * time.Second):
			// Kill the process and all of its children with its process group id.
			pgid, err := syscall.Getpgid(command.Process.Pid)
			if err != nil {
				gp.Error(err, "failed to get process group id on go profiler expired", "source", parameter.Source)
			} else {
				err = syscall.Kill(-pgid, syscall.SIGKILL)
				if err != nil {
					gp.Error(err, "failed to kill process on go profiler expired", "source", parameter.Source)
				} else {
					gp.Info("Process has been killed", "source", parameter.Source, "endpoint", endpoint)

				}
			}
		}
	}()

	return fmt.Sprintf("Visit http://%s, this server will expire in %d seconds.", endpoint, parameter.ExpirationSeconds), nil
}

// DownloadProfileFile do http request to download profile file and write into specified filepath
func DownloadProfileFile(filepath string, source string, token []byte, ca []byte, timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
	}
	if token != nil {
		conf := &transport.Config{
			TLS: transport.TLSConfig{
				CAData: ca,
			},
			BearerToken: string(token),
		}
		transport, err := transport.New(conf)
		if err != nil {
			return err
		}
		client.Transport = transport
	}
	req, err := http.NewRequest("GET", source, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to do request to %s: %v", source, resp.Status)
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// GetSecretData obtain secret.data from specified secret
func GetSecretData(ca cache.Cache, name string, namespace string) (map[string][]byte, error) {
	var secret corev1.Secret
	ctx := context.Background()
	if err := ca.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, &secret); err != nil {
		return nil, err
	}

	return secret.Data, nil
}
