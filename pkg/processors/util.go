/*
Copyright 2021 The Kube Diagnoser Authors.

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

package processors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	v1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/executor"
)

// DecodeOperationContext unmarshals json encoding into a map[string][]byte, which is the format of operation context.
func DecodeOperationContext(body []byte) (map[string][]byte, error) {
	data := make(map[string][]byte)
	err := json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func ExtractParametersFromHTTPContext(r *http.Request) (map[string]string, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %v", err)
	}
	data := make(map[string]string)
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %v", err)
	}
	return data, nil
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

// getPodInfoFromHeader will get pod info from http request header.
func getPodInfoFromHeader(r *http.Request) v1.PodReference {
	return v1.PodReference{
		NamespacedName: v1.NamespacedName{
			Namespace: r.Header.Get(executor.TracePodNamespace),
			Name:      r.Header.Get(executor.TracePodName),
		},
		Container: r.Header.Get(executor.TracePodContainerName),
	}
}
