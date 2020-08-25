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
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/abnormalreaper"
	"netease.com/k8s/kube-diagnoser/pkg/controllers"
	"netease.com/k8s/kube-diagnoser/pkg/diagnoserchain"
	"netease.com/k8s/kube-diagnoser/pkg/diagnoserchain/diagnoser"
	"netease.com/k8s/kube-diagnoser/pkg/informationmanager"
	"netease.com/k8s/kube-diagnoser/pkg/informationmanager/informationcollector"
	"netease.com/k8s/kube-diagnoser/pkg/recovererchain"
	"netease.com/k8s/kube-diagnoser/pkg/recovererchain/recoverer"
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
	// DockerEndpoint specifies the docker endpoint.
	DockerEndpoint string
	// AbnormalTTL is amount of time to retain abnormals.
	AbnormalTTL time.Duration
	// MinimumAbnormalTTLDuration is minimum age for a finished abnormal before it is garbage collected.
	MinimumAbnormalTTLDuration time.Duration
	// MaximumAbnormalsPerNode is maximum number of finished abnormals to retain per node.
	MaximumAbnormalsPerNode int32
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
		Address:                    "0.0.0.0:8090",
		MetricsAddress:             "0.0.0.0:10357",
		EnableLeaderElection:       false,
		AbnormalTTL:                240 * time.Hour,
		MinimumAbnormalTTLDuration: 30 * time.Minute,
		MaximumAbnormalsPerNode:    20,
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

	stopCh := SetupSignalHandler()

	// Channels for queuing Abnormals along the pipeline of information collection, diagnosis, recovery.
	sourceManagerCh := make(chan diagnosisv1.Abnormal, 1000)
	informationManagerCh := make(chan diagnosisv1.Abnormal, 1000)
	diagnoserChainCh := make(chan diagnosisv1.Abnormal, 1000)
	recovererChainCh := make(chan diagnosisv1.Abnormal, 1000)

	// Run source manager, information manager, diagnoser chain and recoverer chain.
	sourceManager := sourcemanager.NewSourceManager(
		context.Background(),
		ctrl.Log.WithName("sourcemanager"),
		mgr.GetClient(),
		mgr.GetEventRecorderFor("kube-diagnoser/sourcemanager"),
		mgr.GetScheme(),
		mgr.GetCache(),
		opts.NodeName,
		sourceManagerCh,
		informationManagerCh,
	)
	go func(stopCh chan struct{}) {
		sourceManager.Run(stopCh)
	}(stopCh)
	informationManager := informationmanager.NewInformationManager(
		context.Background(),
		ctrl.Log.WithName("informationmanager"),
		mgr.GetClient(),
		mgr.GetEventRecorderFor("kube-diagnoser/informationmanager"),
		mgr.GetScheme(),
		mgr.GetCache(),
		opts.NodeName,
		informationManagerCh,
		diagnoserChainCh,
	)
	go func(stopCh chan struct{}) {
		informationManager.Run(stopCh)
	}(stopCh)
	diagnoserChain := diagnoserchain.NewDiagnoserChain(
		context.Background(),
		ctrl.Log.WithName("diagnoserchain"),
		mgr.GetClient(),
		mgr.GetEventRecorderFor("kube-diagnoser/diagnoserchain"),
		mgr.GetScheme(),
		mgr.GetCache(),
		opts.NodeName,
		diagnoserChainCh,
		recovererChainCh,
	)
	go func(stopCh chan struct{}) {
		diagnoserChain.Run(stopCh)
	}(stopCh)
	recovererChain := recovererchain.NewRecovererChain(
		context.Background(),
		ctrl.Log.WithName("recovererchain"),
		mgr.GetClient(),
		mgr.GetEventRecorderFor("kube-diagnoser/recovererchain"),
		mgr.GetScheme(),
		mgr.GetCache(),
		opts.NodeName,
		recovererChainCh,
	)
	go func(stopCh chan struct{}) {
		recovererChain.Run(stopCh)
	}(stopCh)

	// Run abnormal reaper for garbage collection.
	abnormalreaper := abnormalreaper.NewAbnormalReaper(
		context.Background(),
		ctrl.Log.WithName("abnormalreaper"),
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetCache(),
		opts.NodeName,
		opts.AbnormalTTL,
		opts.MinimumAbnormalTTLDuration,
		opts.MaximumAbnormalsPerNode,
	)
	go func(stopCh chan struct{}) {
		abnormalreaper.Run(stopCh)
	}(stopCh)

	// Setup information collectors, diagnosers and recoverers.
	podCollector := informationcollector.NewPodCollector(
		context.Background(),
		ctrl.Log.WithName("informationmanager/podcollector"),
		mgr.GetCache(),
		opts.NodeName,
	)
	containerCollector, err := informationcollector.NewContainerCollector(
		context.Background(),
		ctrl.Log.WithName("informationmanager/containercollector"),
		opts.DockerEndpoint,
	)
	if err != nil {
		setupLog.Error(err, "unable to create information collector", "informationcollector", "containercollector")
		return fmt.Errorf("unable to create information collector: %v", err)
	}
	processCollector := informationcollector.NewProcessCollector(
		context.Background(),
		ctrl.Log.WithName("informationmanager/processcollector"),
	)
	podDiskUsageDiagnoser := diagnoser.NewPodDiskUsageDiagnoser(
		context.Background(),
		ctrl.Log.WithName("diagnoserchain/poddiskusagediagnoser"),
	)
	terminatingPodDiagnoser := diagnoser.NewTerminatingPodDiagnoser(
		context.Background(),
		ctrl.Log.WithName("diagnoserchain/terminatingpoddiagnoser"),
	)
	signalRecoverer := recoverer.NewSignalRecoverer(
		context.Background(),
		ctrl.Log.WithName("recovererchain/signalrecoverer"),
	)

	// Start http server.
	go func(stopCh chan struct{}) {
		r := mux.NewRouter()
		r.HandleFunc("/informationcollector", informationManager.Handler)
		r.HandleFunc("/informationcollector/podcollector", podCollector.Handler)
		r.HandleFunc("/informationcollector/containercollector", containerCollector.Handler)
		r.HandleFunc("/informationcollector/processcollector", processCollector.Handler)
		r.HandleFunc("/diagnoser", diagnoserChain.Handler)
		r.HandleFunc("/diagnoser/poddiskusagediagnoser", podDiskUsageDiagnoser.Handler)
		r.HandleFunc("/diagnoser/terminatingpoddiagnoser", terminatingPodDiagnoser.Handler)
		r.HandleFunc("/recoverer", recovererChain.Handler)
		r.HandleFunc("/recoverer/signalrecoverer", signalRecoverer.Handler)
		r.HandleFunc("/healthz", HealthCheckHandler)

		// Start pprof server.
		r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
		if err := http.ListenAndServe(opts.Address, r); err != nil {
			setupLog.Error(err, "unable to start http server")
			close(stopCh)
		}
	}(stopCh)

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
	if err := mgr.Start(stopCh); err != nil {
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
	fs.StringVar(&opts.DockerEndpoint, "docker-endpoint", "unix:///var/run/docker.sock", "The docker endpoint.")
	fs.DurationVar(&opts.AbnormalTTL, "abnormal-ttl", opts.AbnormalTTL, "Amount of time to retain abnormals.")
	fs.DurationVar(&opts.MinimumAbnormalTTLDuration, "minimum-abnormal-ttl-duration", opts.MinimumAbnormalTTLDuration, "Minimum age for a finished abnormal before it is garbage collected.")
	fs.Int32Var(&opts.MaximumAbnormalsPerNode, "maximum-abnormals-per-node", opts.MaximumAbnormalsPerNode, "Maximum number of finished abnormals to retain per node.")
}

// SetupSignalHandler registers for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() chan struct{} {
	stopCh := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		setupLog.Info("stop signal received")
		close(stopCh)
		// Exit directly on the second signal.
		<-c
		setupLog.Info("exit directly on the second signal")
		os.Exit(1)
	}()

	return stopCh
}

// HealthCheckHandler handles health check requests.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.Write([]byte("OK"))
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
