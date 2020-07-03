/*
Copyright 2020 The Kube Diagnoser Authors.

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

package main

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/controllers"
	"netease.com/k8s/kube-diagnoser/pkg/diagnoserchain"
	"netease.com/k8s/kube-diagnoser/pkg/informationmanager"
	"netease.com/k8s/kube-diagnoser/pkg/recovererchain"
	"netease.com/k8s/kube-diagnoser/pkg/sourcemanager"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// KubeDiagnoserAgentOptions is the main context object for the kube diagnoser agent.
type KubeDiagnoserAgentOptions struct {
	// Address is the address on which to advertise.
	Address string
	// NodeName specifies the node name.
	NodeName string
	// MetricsAddress is the address the metric endpoint binds to.
	MetricsAddress string
	// EnableLeaderElection enables leader election for kube diagnoser agent.
	EnableLeaderElection bool
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = diagnosisv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	opts := NewKubeDiagnoserAgentOptions()

	cmd := &cobra.Command{
		Use: "kube-diagnoser",
		Long: `The Kubernetes diagnoser agent runs on each node. This watches Abnormals
and executes information collection, diagnosis and recovery according to specification
of an Abnormal.`,
		Run: func(cmd *cobra.Command, args []string) {
			setupLog.Error(opts.Run(), "failed to run kube diagnoser agent")
			os.Exit(1)
		},
	}

	opts.AddFlags(cmd.Flags())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// NewKubeDiagnoserAgentOptions creates a new KubeDiagnoserAgentOptions with a default config.
func NewKubeDiagnoserAgentOptions() *KubeDiagnoserAgentOptions {
	return &KubeDiagnoserAgentOptions{
		Address:              "0.0.0.0:8090",
		MetricsAddress:       "0.0.0.0:10357",
		EnableLeaderElection: false,
	}
}

// Run setups all controllers and starts the manager.
func (opts *KubeDiagnoserAgentOptions) Run() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: opts.MetricsAddress,
		Port:               9443,
		LeaderElection:     opts.EnableLeaderElection,
		LeaderElectionID:   "8a2b2861.netease.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return fmt.Errorf("unable to start manager: %v", err)
	}

	// Channels for queuing Abnormals along the pipeline of information collection, diagnosis, recovery.
	sourceManagerCh := make(chan diagnosisv1.Abnormal, 1000)
	informationManagerCh := make(chan diagnosisv1.Abnormal, 1000)
	diagnoserChainCh := make(chan diagnosisv1.Abnormal, 1000)
	recovererChainCh := make(chan diagnosisv1.Abnormal, 1000)
	stopCh := make(chan struct{})

	// Run source manager, information manager, diagnoser chain and recoverer chain.
	sourceManager := sourcemanager.NewSourceManager(
		mgr.GetClient(),
		ctrl.Log.WithName("sourcemanager"),
		mgr.GetScheme(),
		mgr.GetCache(),
		sourceManagerCh,
		informationManagerCh,
		stopCh,
	)
	go func() {
		err := sourceManager.Run()
		if err != nil {
			setupLog.Error(err, "unable to start source manager")
		}
	}()
	informationManager := informationmanager.NewInformationManager(
		mgr.GetClient(),
		ctrl.Log.WithName("informationmanager"),
		mgr.GetScheme(),
		mgr.GetCache(),
		informationManagerCh,
		diagnoserChainCh,
		stopCh,
	)
	go func() {
		err := informationManager.Run()
		if err != nil {
			setupLog.Error(err, "unable to start information manager")
		}
	}()
	diagnoserChain := diagnoserchain.NewDiagnoserChain(
		mgr.GetClient(),
		ctrl.Log.WithName("diagnoserchain"),
		mgr.GetScheme(),
		mgr.GetCache(),
		diagnoserChainCh,
		recovererChainCh,
		stopCh,
	)
	go func() {
		err := diagnoserChain.Run()
		if err != nil {
			setupLog.Error(err, "unable to start diagnoser chain")
		}
	}()
	recovererChain := recovererchain.NewRecovererChain(
		mgr.GetClient(),
		ctrl.Log.WithName("recovererchain"),
		mgr.GetScheme(),
		mgr.GetCache(),
		recovererChainCh,
		stopCh,
	)
	go func() {
		err := recovererChain.Run()
		if err != nil {
			setupLog.Error(err, "unable to start recoverer chain")
		}
	}()

	// Setup reconcilers for Abnormal, InformationCollector, Diagnoser and Recoverer.
	if err = (controllers.NewAbnormalReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Abnormal"),
		mgr.GetScheme(),
		sourceManagerCh,
		informationManagerCh,
		diagnoserChainCh,
		recovererChainCh,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Abnormal")
		return fmt.Errorf("unable to create controller for Abnormal: %v", err)
	}
	if err = (controllers.NewInformationCollectorReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("InformationCollector"),
		mgr.GetScheme(),
		informationManagerCh,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "InformationCollector")
		return fmt.Errorf("unable to create controller for InformationCollector: %v", err)
	}
	if err = (controllers.NewDiagnoserReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Diagnoser"),
		mgr.GetScheme(),
		diagnoserChainCh,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Diagnoser")
		return fmt.Errorf("unable to create controller for Diagnoser: %v", err)
	}
	if err = (controllers.NewRecovererReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Recoverer"),
		mgr.GetScheme(),
		recovererChainCh,
	)).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Recoverer")
		return fmt.Errorf("unable to create controller for Recoverer: %v", err)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return fmt.Errorf("problem running manager: %v", err)
	}

	return nil
}

// AddFlags adds flags to fs and binds them to options.
func (opts *KubeDiagnoserAgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.Address, "address", opts.Address, "The address on which to advertise.")
	fs.StringVar(&opts.NodeName, "node-name", opts.NodeName, "The node name.")
	fs.StringVar(&opts.MetricsAddress, "metrics-address", opts.MetricsAddress, "The address the metric endpoint binds to.")
	fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", opts.EnableLeaderElection, "Enables leader election for kube diagnoser agent.")
}
