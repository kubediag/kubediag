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
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var kubeClient client.Client
var testEnv *envtest.Environment

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	UseExistingCluster := true
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join("..", "..", "config", "crd", "bases")},
		UseExistingCluster: &UseExistingCluster,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = diagnosisv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping kubeClient")
	kubeClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(kubeClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
