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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	diagnosisv1 "netease.com/k8s/kube-diagnoser/api/v1"
	"netease.com/k8s/kube-diagnoser/pkg/abnormalreaper"
	"netease.com/k8s/kube-diagnoser/pkg/alertmanager"
	"netease.com/k8s/kube-diagnoser/pkg/clusterhealthevaluator"
	"netease.com/k8s/kube-diagnoser/pkg/controllers"
	"netease.com/k8s/kube-diagnoser/pkg/diagnoserchain"
	"netease.com/k8s/kube-diagnoser/pkg/diagnoserchain/diagnoser"
	"netease.com/k8s/kube-diagnoser/pkg/eventer"
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

// KubeDiagnoserOptions is the main context object for the kube diagnoser.
type KubeDiagnoserOptions struct {
	// Mode specifies whether the kube diagnoser is running as a master or an agnet.
	Mode string
	// Address is the address on which to advertise.
	Address string
	// NodeName specifies the node name.
	NodeName string
	// MetricsAddress is the address the metric endpoint binds to.
	MetricsAddress string
	// EnableLeaderElection enables leader election for kube diagnoser master.
	EnableLeaderElection bool
	// Port is the port that the webhook server serves at.
	Port int
	// Host is the hostname that the webhook server binds to.
	Host string
	// CertDir is the directory that contains the server key and certificate.
	CertDir string
	// AlertmanagerRepeatInterval specifies how long to wait before sending a notification again if it has
	// already been sent successfully for an alert.
	AlertmanagerRepeatInterval time.Duration
	// ClusterHealthEvaluatorHousekeepingInterval specifies the interval between cluster health updates.
	ClusterHealthEvaluatorHousekeepingInterval time.Duration
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
	_ = corev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	opts := NewKubeDiagnoserOptions()

	cmd := &cobra.Command{
		Use: "kube-diagnoser",
		Long: `The Kubernetes diagnoser is a daemon that embeds the core pipeline of
abnormal diagnosis and recovery. It could be run in either master mode or
agent mode. In master mode it processes prometheus alerts and monitors
cluster health status. In agent mode it watches Abnormals and executes
information collection, diagnosis and recovery according to specification
of an Abnormal.`,
		Run: func(cmd *cobra.Command, args []string) {
			setupLog.Error(opts.Run(), "failed to run kube diagnoser")
			os.Exit(1)
		},
	}

	opts.AddFlags(cmd.Flags())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// NewKubeDiagnoserOptions creates a new KubeDiagnoserOptions with a default config.
func NewKubeDiagnoserOptions() *KubeDiagnoserOptions {
	return &KubeDiagnoserOptions{
		Mode:                       "agent",
		Address:                    "0.0.0.0:8090",
		MetricsAddress:             "0.0.0.0:10357",
		EnableLeaderElection:       false,
		Port:                       9443,
		CertDir:                    "/etc/kube-diagnoser/serving-certs",
		AlertmanagerRepeatInterval: 6 * time.Hour,
		ClusterHealthEvaluatorHousekeepingInterval: 30 * time.Second,
		AbnormalTTL:                240 * time.Hour,
		MinimumAbnormalTTLDuration: 30 * time.Minute,
		MaximumAbnormalsPerNode:    20,
	}
}

// Run setups all controllers and starts the manager.
func (opts *KubeDiagnoserOptions) Run() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if opts.Mode == "master" {
		setupLog.Info("kube diagnoser is running in master mode")

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: opts.MetricsAddress,
			Port:               opts.Port,
			Host:               opts.Host,
			CertDir:            opts.CertDir,
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
		eventChainCh := make(chan corev1.Event, 1000)

		// Run source manager.
		sourceManager := sourcemanager.NewSourceManager(
			context.Background(),
			ctrl.Log.WithName("sourcemanager"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/sourcemanager"),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.NodeName,
			sourceManagerCh,
		)
		go func(stopCh chan struct{}) {
			sourceManager.Run(stopCh)
		}(stopCh)

		// Create alertmanager for managing prometheus alerts.
		alertmanager := alertmanager.NewAlertmanager(
			context.Background(),
			ctrl.Log.WithName("alertmanager"),
			opts.AlertmanagerRepeatInterval,
			sourceManagerCh,
		)

		// Create eventer for managing kubernetes events.
		eventer := eventer.NewEventer(
			context.Background(),
			ctrl.Log.WithName("eventer"),
			eventChainCh,
			sourceManagerCh,
		)
		go func(stopCh chan struct{}) {
			eventer.Run(stopCh)
		}(stopCh)

		clusterHealthEvaluator := clusterhealthevaluator.NewClusterHealthEvaluator(
			context.Background(),
			ctrl.Log.WithName("clusterhealthevaluator"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/clusterhealthevaluator"),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.ClusterHealthEvaluatorHousekeepingInterval,
		)
		go func(stopCh chan struct{}) {
			clusterHealthEvaluator.Run(stopCh)
		}(stopCh)

		// Start http server.
		go func(stopCh chan struct{}) {
			r := mux.NewRouter()
			r.HandleFunc("/api/v1/alerts", alertmanager.Handler)
			r.HandleFunc("/clusterhealth", clusterHealthEvaluator.Handler)

			// Start pprof server.
			r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
			if err := http.ListenAndServe(opts.Address, r); err != nil {
				setupLog.Error(err, "unable to start http server")
				close(stopCh)
			}
		}(stopCh)

		// Setup reconcilers for Abnormal and AbnormalSource.
		if err = (controllers.NewAbnormalReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Abnormal"),
			mgr.GetScheme(),
			opts.Mode,
			sourceManagerCh,
			nil,
			nil,
			nil,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Abnormal")
			return fmt.Errorf("unable to create controller for Abnormal: %v", err)
		}
		if err = (controllers.NewEventReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Event"),
			mgr.GetScheme(),
			eventChainCh,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Event")
			return fmt.Errorf("unable to create controller for Event: %v", err)
		}
		if err = (controllers.NewAbnormalSourceReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("AbnormalSource"),
			mgr.GetScheme(),
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "AbnormalSource")
			return fmt.Errorf("unable to create controller for AbnormalSource: %v", err)
		}

		// Setup webhooks for Abnormal, InformationCollector, Diagnoser and Recoverer.
		if err = (&diagnosisv1.Abnormal{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Abnormal")
			return fmt.Errorf("unable to create webhook for Abnormal: %v", err)
		}
		if err = (&diagnosisv1.Diagnoser{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Diagnoser")
			return fmt.Errorf("unable to create webhook for Diagnoser: %v", err)
		}
		if err = (&diagnosisv1.InformationCollector{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "InformationCollector")
			return fmt.Errorf("unable to create webhook for InformationCollector: %v", err)
		}
		if err = (&diagnosisv1.Recoverer{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Recoverer")
			return fmt.Errorf("unable to create webhook for Recoverer: %v", err)
		}
		// +kubebuilder:scaffold:builder

		setupLog.Info("starting manager")
		if err := mgr.Start(stopCh); err != nil {
			setupLog.Error(err, "problem running manager")
			return fmt.Errorf("problem running manager: %v", err)
		}
	} else if opts.Mode == "agent" {
		setupLog.Info("kube diagnoser is running in agent mode")

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: opts.MetricsAddress,
			LeaderElection:     false,
			LeaderElectionID:   "8a2b2861.netease.com",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			return fmt.Errorf("unable to start manager: %v", err)
		}

		stopCh := SetupSignalHandler()

		// Channels for queuing Abnormals along the pipeline of information collection, diagnosis, recovery.
		informationManagerCh := make(chan diagnosisv1.Abnormal, 1000)
		diagnoserChainCh := make(chan diagnosisv1.Abnormal, 1000)
		recovererChainCh := make(chan diagnosisv1.Abnormal, 1000)

		// Run information manager, diagnoser chain and recoverer chain.
		informationManager := informationmanager.NewInformationManager(
			context.Background(),
			ctrl.Log.WithName("informationmanager"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/informationmanager"),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.NodeName,
			informationManagerCh,
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
		abnormalReaper := abnormalreaper.NewAbnormalReaper(
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
			abnormalReaper.Run(stopCh)
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
		fileStatusCollector := informationcollector.NewFileStatusCollector(
			context.Background(),
			ctrl.Log.WithName("informationmanager/filestatuscollector"),
		)
		systemdCollector := informationcollector.NewSystemdCollector(
			context.Background(),
			ctrl.Log.WithName("informationmanager/systemdcollector"),
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
			r.HandleFunc("/informationcollector/filestatuscollector", fileStatusCollector.Handler)
			r.HandleFunc("/informationcollector/systemdcollector", systemdCollector.Handler)
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
			opts.Mode,
			nil,
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
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "InformationCollector")
			return fmt.Errorf("unable to create controller for InformationCollector: %v", err)
		}
		if err = (controllers.NewDiagnoserReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Diagnoser"),
			mgr.GetScheme(),
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Diagnoser")
			return fmt.Errorf("unable to create controller for Diagnoser: %v", err)
		}
		if err = (controllers.NewRecovererReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Recoverer"),
			mgr.GetScheme(),
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
	} else {
		return fmt.Errorf("invalid kube diagnoser mode: %s", opts.Mode)
	}

	return nil
}

// AddFlags adds flags to fs and binds them to options.
func (opts *KubeDiagnoserOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.Mode, "mode", opts.Mode, "Whether the kube diagnoser is running as a master or an agnet.")
	fs.StringVar(&opts.Address, "address", opts.Address, "The address on which to advertise.")
	fs.StringVar(&opts.NodeName, "node-name", opts.NodeName, "The node name.")
	fs.StringVar(&opts.MetricsAddress, "metrics-address", opts.MetricsAddress, "The address the metric endpoint binds to.")
	fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", opts.EnableLeaderElection, "Enables leader election for kube diagnoser master.")
	fs.StringVar(&opts.DockerEndpoint, "docker-endpoint", "unix:///var/run/docker.sock", "The docker endpoint.")
	fs.IntVar(&opts.Port, "port", opts.Port, "The port that the webhook server serves at.")
	fs.StringVar(&opts.Host, "host", opts.Host, "The hostname that the webhook server binds to.")
	fs.StringVar(&opts.CertDir, "cert-dir", opts.CertDir, "The directory that contains the server key and certificate.")
	fs.DurationVar(&opts.AlertmanagerRepeatInterval, "repeat-interval", opts.AlertmanagerRepeatInterval, "How long to wait before sending a notification again if it has already been sent successfully for an alert.")
	fs.DurationVar(&opts.ClusterHealthEvaluatorHousekeepingInterval, "cluster-health-evaluator-housekeeping-interval", opts.ClusterHealthEvaluatorHousekeepingInterval, "The interval between cluster health updates.")
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
