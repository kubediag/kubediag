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

package e2e

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

var _ = Describe("Diagnosis", func() {

	f := NewDefaultFramework("diagnosis")
	var (
		operationset *diagnosisv1.OperationSet
		diagnosis    *diagnosisv1.Diagnosis
		err          error
	)

	ginkgo.AfterEach(func() {
		// Operationset can not be deleted in framework.AfterEach, it is not a namespace scoped resource.
		if operationset != nil {
			By(fmt.Sprintf("Deleting operationset %s", operationset.Name))
			err := f.KubeClient.Get(f, types.NamespacedName{Name: operationset.Name}, operationset)
			if err != nil && apierrs.IsNotFound(err) {
				return
			}
			ExpectNoError(err)
			err = f.KubeClient.Delete(f, operationset)
			ExpectNoError(err)
		}
	})

	Context("When calling pod collector processor", func() {
		It("Should run a diagnosis successful", func() {
			By("Creating a pod list collector operationset")
			operationset = f.NewSimpleOperationset("operationset-pod-list-collector", f.Namespace.Name, podListCollectorOperation)
			err = f.KubeClient.Create(f, operationset)
			ExpectNoError(err, "failed to create operationset %s", operationset.Name)
			err = WaitForOperationsetPath(f.KubeClient, operationset.Name, 1, poll, pollShortTimeout)
			ExpectNoError(err)

			By("Creating a pod collector diagnosis")
			diagnosis = f.NewDiagnosisWithRandomNode("diagnosis-pod-list-collector", f.Namespace.Name, operationset.Name)
			err = f.KubeClient.Create(f, diagnosis)
			ExpectNoError(err, "failed to create diagnosis %s in namespace: %s", diagnosis.Name, f.Namespace.Name)
			err = WaitForDiagnosisPhaseSucceeded(f.KubeClient, diagnosis.Name, diagnosis.Namespace, poll, pollLongTimeout)
			ExpectNoError(err)

		})
	})

})

// WaitForOperationsetPath waits for the operationset's path to match the given path.
func WaitForOperationsetPath(c client.Client, operationSetName string, expectedPathLength int, pollInterval, pollTimeout time.Duration) error {
	var operationset diagnosisv1.OperationSet

	err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		var err error
		err = c.Get(nil, client.ObjectKey{Name: operationSetName}, &operationset)
		if err != nil {
			return false, nil
		}
		if len(operationset.Status.Paths) == 0 {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("error waiting for operationset %q path to match expectation: %v",
			operationSetName, err)
	}

	if len(operationset.Status.Paths) != expectedPathLength {
		return fmt.Errorf("error waiting for operationset %q (got %d)  path to match expectation (expected %d): %v",
			operationSetName, len(operationset.Status.Paths), expectedPathLength, err)
	}

	return nil
}

// WaitForDiagnosisPhaseSucceeded waits for the diagnosis's phase to be succeeded.
func WaitForDiagnosisPhaseSucceeded(c client.Client, diagnosisName, diagnosisNamespace string, pollInterval, pollTimeout time.Duration) error {
	var diagnosis diagnosisv1.Diagnosis
	err := wait.PollImmediate(pollInterval, pollTimeout, func() (bool, error) {
		var err error
		err = c.Get(nil, client.ObjectKey{Name: diagnosisName, Namespace: diagnosisNamespace}, &diagnosis)
		if err != nil {
			return false, nil
		}
		if diagnosis.Status.Phase != diagnosisv1.DiagnosisSucceeded {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("Error waiting for diagnosis %q phase to succeeded: %v", diagnosis.Name, err)
	}

	return nil
}
