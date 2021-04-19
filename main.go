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
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/component-base/cli/flag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/alertmanager"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/clusterhealthevaluator"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/controllers"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/diagnosisreaper"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/eventer"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/executor"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/features"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/graphbuilder"
	"github.com/kube-diagnoser/kube-diagnoser/pkg/processors"
	// +kubebuilder:scaffold:imports
)

var (
	scheme           = runtime.NewScheme()
	setupLog         = ctrl.Log.WithName("setup")
	defaultTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultCertDir   = "/etc/kube-diagnoser/serving-certs"
	defaultDataRoot  = "/var/lib/kube-diagnoser"
)

// KubeDiagnoserOptions is the main context object for the kube diagnoser.
type KubeDiagnoserOptions struct {
	// Mode specifies whether the kube diagnoser is running as a master or an agnet.
	Mode string
	// BindAddress is the address on which to advertise.
	BindAddress string
	// Port is the port for the kube diagnoser to serve on.
	Port int
	// NodeName specifies the node name.
	NodeName string
	// MetricsPort is the port the metric endpoint to serve on.
	MetricsPort int
	// EnableLeaderElection enables leader election for kube diagnoser master.
	EnableLeaderElection bool
	// WebhookPort is the port that the webhook server serves at.
	WebhookPort int
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
	// DiagnosisTTL is amount of time to retain diagnoses.
	DiagnosisTTL time.Duration
	// MinimumDiagnosisTTLDuration is minimum age for a finished diagnosis before it is garbage collected.
	MinimumDiagnosisTTLDuration time.Duration
	// MaximumDiagnosesPerNode is maximum number of finished diagnoses to retain per node.
	MaximumDiagnosesPerNode int32
	// APIServerAccessToken is the kubernetes apiserver access token.
	APIServerAccessToken string
	// FeatureGates is a map of feature names to bools that enable or disable features. This field modifies
	// piecemeal the default values from "github.com/kube-diagnoser/kube-diagnoser/pkg/features/features.go".
	FeatureGates map[string]bool
	// DataRoot is root directory of persistent kube diagnoser data.
	DataRoot string
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = diagnosisv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	opts, err := NewKubeDiagnoserOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use: "kube-diagnoser",
		Long: `The Kubernetes diagnoser is a daemon that embeds the core pipeline of
diagnosis diagnosis and recovery. It could be run in either master mode or
agent mode. In master mode it processes prometheus alerts and monitors
cluster health status. In agent mode it watches Diagnoses and executes
information collection, diagnosis and recovery according to specification
of an Diagnosis.`,
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
func NewKubeDiagnoserOptions() (*KubeDiagnoserOptions, error) {
	// Set token at /var/run/secrets/kubernetes.io/serviceaccount/token as default if kube diagnoser
	// is running in a pod.
	token := []byte{}
	_, err := os.Stat(defaultTokenFile)
	if err == nil {
		token, err = ioutil.ReadFile(defaultTokenFile)
		if err != nil {
			return nil, err
		}
	}

	return &KubeDiagnoserOptions{
		Mode:                       "agent",
		BindAddress:                "0.0.0.0",
		Port:                       8090,
		MetricsPort:                10357,
		EnableLeaderElection:       false,
		WebhookPort:                9443,
		CertDir:                    defaultCertDir,
		AlertmanagerRepeatInterval: 6 * time.Hour,
		ClusterHealthEvaluatorHousekeepingInterval: 30 * time.Second,
		DiagnosisTTL:                240 * time.Hour,
		MinimumDiagnosisTTLDuration: 30 * time.Minute,
		MaximumDiagnosesPerNode:     20,
		APIServerAccessToken:        string(token),
		DataRoot:                    defaultDataRoot,
	}, nil
}

// Run setups all controllers and starts the manager.
func (opts *KubeDiagnoserOptions) Run() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	featureGate := features.NewFeatureGate()
	err := featureGate.SetFromMap(opts.FeatureGates)
	if err != nil {
		setupLog.Error(err, "unable to set feature gates")
		return fmt.Errorf("unable to set feature gates: %v", err)
	}

	if opts.Mode == "master" {
		setupLog.Info("kube diagnoser is running in master mode")

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.MetricsPort),
			Port:               opts.WebhookPort,
			Host:               opts.Host,
			CertDir:            opts.CertDir,
			LeaderElection:     opts.EnableLeaderElection,
			LeaderElectionID:   "8a2b2861.netease.com",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			return fmt.Errorf("unable to start manager: %v", err)
		}

		// Channel for queuing kubernetes events and operation sets.
		eventChainCh := make(chan corev1.Event, 1000)
		graphBuilderCh := make(chan diagnosisv1.OperationSet, 1000)
		stopCh := SetupSignalHandler()

		// Create graph builder for generating graph from operation set.
		graphbuilder := graphbuilder.NewGraphBuilder(
			context.Background(),
			ctrl.Log.WithName("graphbuilder"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/graphbuilder"),
			mgr.GetScheme(),
			mgr.GetCache(),
			graphBuilderCh,
		)
		go func(stopCh chan struct{}) {
			graphbuilder.Run(stopCh)
		}(stopCh)

		// Create alertmanager for managing prometheus alerts.
		alertmanager := alertmanager.NewAlertmanager(
			context.Background(),
			ctrl.Log.WithName("alertmanager"),
			mgr.GetClient(),
			mgr.GetCache(),
			opts.NodeName,
			opts.AlertmanagerRepeatInterval,
			featureGate.Enabled(features.Alertmanager),
		)

		// Create eventer for managing kubernetes events.
		eventer := eventer.NewEventer(
			context.Background(),
			ctrl.Log.WithName("eventer"),
			mgr.GetClient(),
			mgr.GetCache(),
			opts.NodeName,
			eventChainCh,
			featureGate.Enabled(features.Eventer),
		)
		go func(stopCh chan struct{}) {
			eventer.Run(stopCh)
		}(stopCh)

		// Create alertmanager for evaluating cluster health.
		clusterHealthEvaluator := clusterhealthevaluator.NewClusterHealthEvaluator(
			context.Background(),
			ctrl.Log.WithName("clusterhealthevaluator"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/clusterhealthevaluator"),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.ClusterHealthEvaluatorHousekeepingInterval,
			opts.APIServerAccessToken,
			featureGate.Enabled(features.ClusterHealthEvaluator),
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
			if err := http.ListenAndServe(fmt.Sprintf("%s:%d", opts.BindAddress, opts.Port), r); err != nil {
				setupLog.Error(err, "unable to start http server")
				close(stopCh)
			}
		}(stopCh)

		// Setup reconcilers for Diagnosis, Trigger, OperationSet and Event.
		if err = (controllers.NewDiagnosisReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Diagnosis"),
			mgr.GetScheme(),
			opts.Mode,
			opts.NodeName,
			nil,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Diagnosis")
			return fmt.Errorf("unable to create controller for Diagnosis: %v", err)
		}
		if err = (controllers.NewTriggerReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Trigger"),
			mgr.GetScheme(),
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Trigger")
			return fmt.Errorf("unable to create controller for Trigger: %v", err)
		}
		if err = (controllers.NewOperationSetReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("OperationSet"),
			mgr.GetScheme(),
			graphBuilderCh,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "OperationSet")
			return fmt.Errorf("unable to create controller for OperationSet: %v", err)
		}
		if featureGate.Enabled(features.Eventer) {
			if err = (controllers.NewEventReconciler(
				mgr.GetClient(),
				ctrl.Log.WithName("controllers").WithName("Event"),
				mgr.GetScheme(),
				eventChainCh,
			)).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", "Event")
				return fmt.Errorf("unable to create controller for Event: %v", err)
			}
		}

		// Setup webhooks for Diagnosis, Trigger and Operation.
		if err = (&diagnosisv1.Diagnosis{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Diagnosis")
			return fmt.Errorf("unable to create webhook for Diagnosis: %v", err)
		}
		if err = (&diagnosisv1.Trigger{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Trigger")
			return fmt.Errorf("unable to create webhook for Trigger: %v", err)
		}
		if err = (&diagnosisv1.Operation{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Operation")
			return fmt.Errorf("unable to create webhook for Operation: %v", err)
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
			MetricsBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.MetricsPort),
			LeaderElection:     false,
			LeaderElectionID:   "8a2b2861.netease.com",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			return fmt.Errorf("unable to start manager: %v", err)
		}

		// Channel for queuing Diagnoses to pipeline for executing operations.
		executorCh := make(chan diagnosisv1.Diagnosis, 1000)
		stopCh := SetupSignalHandler()

		// Run executor.
		executor := executor.NewExecutor(
			context.Background(),
			ctrl.Log.WithName("executor"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kube-diagnoser/executor"),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.NodeName,
			opts.BindAddress,
			opts.Port,
			opts.DataRoot,
			executorCh,
		)
		go func(stopCh chan struct{}) {
			executor.Run(stopCh)
		}(stopCh)

		// Run diagnosis reaper for garbage collection.
		diagnosisReaper := diagnosisreaper.NewDiagnosisReaper(
			context.Background(),
			ctrl.Log.WithName("diagnosisreaper"),
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetCache(),
			opts.NodeName,
			opts.DiagnosisTTL,
			opts.MinimumDiagnosisTTLDuration,
			opts.MaximumDiagnosesPerNode,
			opts.DataRoot,
		)
		go func(stopCh chan struct{}) {
			diagnosisReaper.Run(stopCh)
		}(stopCh)

		// Setup operation processors.
		podCollector := processors.NewPodCollector(
			context.Background(),
			ctrl.Log.WithName("processor/podcollector"),
			mgr.GetCache(),
			opts.NodeName,
			featureGate.Enabled(features.PodCollector),
		)
		containerCollector, err := processors.NewContainerCollector(
			context.Background(),
			ctrl.Log.WithName("processor/containercollector"),
			opts.DockerEndpoint,
			featureGate.Enabled(features.ContainerCollector),
		)
		if err != nil {
			setupLog.Error(err, "unable to create processor", "processors", "containercollector")
			return fmt.Errorf("unable to create processor: %v", err)
		}
		processCollector := processors.NewProcessCollector(
			context.Background(),
			ctrl.Log.WithName("processor/processcollector"),
			featureGate.Enabled(features.ProcessCollector),
		)
		commandexecutor := processors.NewCommandExecutor(
			context.Background(),
			ctrl.Log.WithName("processor/commandexecutor"),
			featureGate.Enabled(features.CommandExecutor),
		)
		goProfiler := processors.NewGoProfiler(
			context.Background(),
			ctrl.Log.WithName("processor/goprofiler"),
			mgr.GetCache(),
			opts.DataRoot,
			opts.BindAddress,
			featureGate.Enabled(features.GoProfiler),
		)

		coreFileProfiler, err := processors.NewCoreFileProfiler(
			context.Background(),
			ctrl.Log.WithName("processor/corefileprofiler"),
			opts.DockerEndpoint,
			featureGate.Enabled(features.CorefileProfiler),
			opts.DataRoot)
		if err != nil {
			setupLog.Error(err, "unable to create processor", "processors", "corefileprofiler")
			return fmt.Errorf("unable to create processor: %v", err)
		}

		// Start http server.
		go func(stopCh chan struct{}) {
			// TODO: Implement a registry for managing processor registrations.
			r := mux.NewRouter()
			// handle information collectors
			r.HandleFunc("/processor/podcollector", podCollector.Handler)
			r.HandleFunc("/processor/containercollector", containerCollector.Handler)
			r.HandleFunc("/processor/processcollector", processCollector.Handler)
			// handle executors
			r.HandleFunc("/processor/commandexecutor", commandexecutor.Handler)
			// handle profilers
			r.HandleFunc("/processor/corefileprofiler", coreFileProfiler.Handler)
			r.HandleFunc("/processor/goprofiler", goProfiler.Handler)

			r.HandleFunc("/healthz", HealthCheckHandler)

			// Start pprof server.
			r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
			if err := http.ListenAndServe(fmt.Sprintf("%s:%d", opts.BindAddress, opts.Port), r); err != nil {
				setupLog.Error(err, "unable to start http server")
				close(stopCh)
			}
		}(stopCh)

		// Setup reconcilers for Diagnosis.
		if err = (controllers.NewDiagnosisReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Diagnosis"),
			mgr.GetScheme(),
			opts.Mode,
			opts.NodeName,
			executorCh,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Diagnosis")
			return fmt.Errorf("unable to create controller for Diagnosis: %v", err)
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
	fs.StringVar(&opts.BindAddress, "bind-address", opts.BindAddress, "The address on which to advertise.")
	fs.IntVar(&opts.Port, "port", opts.Port, "The port for the kube diagnoser to serve on.")
	fs.StringVar(&opts.NodeName, "node-name", opts.NodeName, "The node name.")
	fs.IntVar(&opts.MetricsPort, "metrics-port", opts.MetricsPort, "The port the metric endpoint to serve on.")
	fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", opts.EnableLeaderElection, "Enables leader election for kube diagnoser master.")
	fs.StringVar(&opts.DockerEndpoint, "docker-endpoint", "unix:///var/run/docker.sock", "The docker endpoint.")
	fs.IntVar(&opts.WebhookPort, "webhook-port", opts.WebhookPort, "The port that the webhook server serves at.")
	fs.StringVar(&opts.Host, "host", opts.Host, "The hostname that the webhook server binds to.")
	fs.StringVar(&opts.CertDir, "cert-dir", opts.CertDir, "The directory that contains the server key and certificate.")
	fs.DurationVar(&opts.AlertmanagerRepeatInterval, "repeat-interval", opts.AlertmanagerRepeatInterval, "How long to wait before sending a notification again if it has already been sent successfully for an alert.")
	fs.DurationVar(&opts.ClusterHealthEvaluatorHousekeepingInterval, "cluster-health-evaluator-housekeeping-interval", opts.ClusterHealthEvaluatorHousekeepingInterval, "The interval between cluster health updates.")
	fs.DurationVar(&opts.DiagnosisTTL, "diagnosis-ttl", opts.DiagnosisTTL, "Amount of time to retain diagnoses.")
	fs.DurationVar(&opts.MinimumDiagnosisTTLDuration, "minimum-diagnosis-ttl-duration", opts.MinimumDiagnosisTTLDuration, "Minimum age for a finished diagnosis before it is garbage collected.")
	fs.Int32Var(&opts.MaximumDiagnosesPerNode, "maximum-diagnoses-per-node", opts.MaximumDiagnosesPerNode, "Maximum number of finished diagnoses to retain per node.")
	fs.StringVar(&opts.APIServerAccessToken, "apiserver-access-token", opts.APIServerAccessToken, "The kubernetes apiserver access token.")
	fs.Var(flag.NewMapStringBool(&opts.FeatureGates), "feature-gates", "A map of feature names to bools that enable or disable features. Options are:\n"+strings.Join(features.NewFeatureGate().KnownFeatures(), "\n"))
	fs.StringVar(&opts.DataRoot, "data-root", opts.DataRoot, "Root directory of persistent kube diagnoser data.")
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
