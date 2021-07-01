/*
Copyright 2021 The KubeDiag Authors.
Copyright 2014 The Kubernetes Authors.
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

package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

const (

	// Poll is how often to poll namespace, diagnosis and operationset
	poll             = 2 * time.Second
	pollShortTimeout = 30 * time.Second
	pollLongTimeout  = 5 * time.Minute

	// selectorKey is the key of lable map.
	selectorKey = "e2e-test"
	// podListCollectorOperation is the name of pod list collector operation.
	podListCollectorOperation = "pod-list-collector"
)

var (

	// RunID is a unique identifier of the e2e run.
	// Beware that this ID is not the same for all tests in the e2e run, because each Ginkgo node creates it separately.
	RunID = uuid.NewUUID()
)

type Framework struct {
	BaseName string
	context.Context
	KubeClient  client.Client
	Namespace   *corev1.Namespace // Every test has at least one namespace unless creation is skipped
	SelectorKey string            // selectorKey is the key of lable map.
}

// NewDefaultFramework makes a new framework and sets up a BeforeEach/AfterEach for
// you (you can write additional before/after each functions).
func NewDefaultFramework(baseName string) *Framework {

	return NewFramework(baseName)
}

// NewFramework creates a test framework.
func NewFramework(baseName string) *Framework {
	ginkgo.By("Init framework")
	f := &Framework{
		BaseName:    baseName,
		Context:     context.Background(),
		SelectorKey: selectorKey,
	}

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)

	return f
}

// BeforeEach gets a client and makes a namespace.
func (f *Framework) BeforeEach() {
	if f.KubeClient == nil {
		ginkgo.By("Creating a kubernetes client")
		f.KubeClient = kubeClient
	}

	ginkgo.By(fmt.Sprintf("Building a namespace api object, basename %s", f.BaseName))
	namespace, err := CreateTestingNS(strings.ToLower(f.BaseName), f.KubeClient, map[string]string{
		"e2e-framework": f.BaseName,
	})
	ExpectNoError(err)
	f.Namespace = namespace

}

// AfterEach deletes the namespace.
func (f *Framework) AfterEach() {
	ginkgo.By(fmt.Sprintf("Destory the namespace, basename %s", f.BaseName))
	err := f.KubeClient.Delete(context.Background(), f.Namespace)
	ExpectNoError(err)
}

// CreateTestingNS should be used by every test, note that we append a common prefix to the provided test name.
// Please see NewFramework instead of using this directly.
func CreateTestingNS(baseName string, c client.Client, labels map[string]string) (*corev1.Namespace, error) {
	if labels == nil {
		labels = map[string]string{}
	}
	labels[selectorKey] = string(RunID)

	// We don't use ObjectMeta.GenerateName feature, as in case of API call
	// failure we don't know whether the namespace was created and what is its
	// name.
	name := fmt.Sprintf("%v-%v", baseName, RandomSuffix())

	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "",
			Labels:    labels,
		},
		Status: corev1.NamespaceStatus{},
	}
	// Be robust about making the namespace creation call.
	if err := wait.PollImmediate(poll, pollShortTimeout, func() (bool, error) {
		var err error
		err = c.Create(context.Background(), namespaceObj)
		if err != nil {
			if apierrs.IsAlreadyExists(err) {
				// regenerate on conflict
				Logf("Namespace name %q was already taken, generate a new name and retry", namespaceObj.Name)
				namespaceObj.Name = fmt.Sprintf("%v-%v", baseName, RandomSuffix())
			} else {
				Logf("Unexpected error while creating namespace: %v", err)
			}
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return namespaceObj, nil
}

// RandomSuffix provides a random string to append to namespace, operationset and diagnosis.
func RandomSuffix() string {
	return strconv.Itoa(rand.Intn(10000))
}

// NewSimpleOperationset returns an operationset with only one operation.
func (f *Framework) NewSimpleOperationset(name string, ns string, operationName string) *diagnosisv1.OperationSet {
	return &diagnosisv1.OperationSet{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", name, RandomSuffix()),
			Namespace: ns,
			Labels: map[string]string{
				f.SelectorKey: name,
			},
		},
		Spec: diagnosisv1.OperationSetSpec{
			AdjacencyList: []diagnosisv1.Node{
				{
					ID:          0,
					To:          diagnosisv1.NodeSet{1},
					Operation:   "",
					Dependences: []int{},
				},
				{
					ID:          1,
					To:          []int{},
					Operation:   operationName,
					Dependences: []int{},
				}}},
		Status: diagnosisv1.OperationSetStatus{},
	}

}

func (f *Framework) NewDiagnosisWithRandomNode(name, ns, operationSet string) *diagnosisv1.Diagnosis {
	randomNode, err := f.GetRandomNode()
	ExpectNoError(err)
	return f.NewDiagnosis(name, ns, randomNode.Name, operationSet)
}

// NewDiagnosis returns a diagnosis with specified operationset.
func (f *Framework) NewDiagnosis(name, ns, nodeName, operationSet string) *diagnosisv1.Diagnosis {

	return &diagnosisv1.Diagnosis{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", name, RandomSuffix()),
			Namespace: ns,
			Labels: map[string]string{
				f.SelectorKey: name,
			},
		},
		Spec:   diagnosisv1.DiagnosisSpec{NodeName: nodeName, OperationSet: operationSet},
		Status: diagnosisv1.DiagnosisStatus{},
	}
}

// GetRandomNode Chooses a random node
func (f *Framework) GetRandomNode() (*corev1.Node, error) {
	var nodeList corev1.NodeList
	err := f.KubeClient.List(f, &nodeList)
	ExpectNoError(err)
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("Could not find an available node")
	}
	randomNode := nodeList.Items[rand.Intn(len(nodeList.Items))]

	return &randomNode, nil
}
