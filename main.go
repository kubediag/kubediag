/*
Copyright 2020 The KubeDiag Authors.

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

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
	"github.com/kubediag/kubediag/pkg/alertmanager"
	"github.com/kubediag/kubediag/pkg/controllers"
	"github.com/kubediag/kubediag/pkg/diagnosisreaper"
	"github.com/kubediag/kubediag/pkg/eventer"
	"github.com/kubediag/kubediag/pkg/executor"
	"github.com/kubediag/kubediag/pkg/features"
	"github.com/kubediag/kubediag/pkg/graphbuilder"
	"github.com/kubediag/kubediag/pkg/kafka"
	"github.com/kubediag/kubediag/pkg/processors/register"
	// +kubebuilder:scaffold:imports
)

var (
	scheme           = runtime.NewScheme()
	setupLog         = ctrl.Log.WithName("setup")
	defaultTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultCertDir   = "/etc/kubediag/serving-certs"
	defaultDataRoot  = "/var/lib/kubediag"
)

// KubeDiagOptions is the main context object for the kubediag.
type KubeDiagOptions struct {
	// Mode specifies whether the kubediag is running as a master or an agnet.
	Mode string
	// BindAddress is the address on which to advertise.
	BindAddress string
	// Port is the port for the kubediag to serve on.
	Port int
	// NodeName specifies the node name.
	NodeName string
	// MetricsPort is the port the metric endpoint to serve on.
	MetricsPort int
	// EnableLeaderElection enables leader election for kubediag master.
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
	// KafkaBrokers is the list of broker addresses used to connect to the kafka cluster.
	KafkaBrokers []string
	// KafkaTopic specifies the topic to read messages from.
	KafkaTopic string
	// DockerEndpoint specifies the docker endpoint.
	DockerEndpoint string
	// DiagnosisTTL is amount of time to retain diagnoses.
	DiagnosisTTL time.Duration
	// MinimumDiagnosisTTLDuration is minimum age for a finished diagnosis before it is garbage collected.
	MinimumDiagnosisTTLDuration time.Duration
	// MaximumDiagnosesPerNode is maximum number of finished diagnoses to retain per node.
	MaximumDiagnosesPerNode int32
	// FeatureGates is a map of feature names to bools that enable or disable features. This field modifies
	// piecemeal the default values from "github.com/kubediag/kubediag/pkg/features/features.go".
	FeatureGates map[string]bool
	// DataRoot is root directory of persistent kubediag data.
	DataRoot string
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = diagnosisv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme

}

func main() {
	opts, err := NewKubeDiagOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use: "kubediag",
		// TODO: Rewrite this long message.
		Long: `The KubeDiag is a daemon that embeds the core pipeline of
diagnosis and recovery. It could be run in either master mode or
agent mode. In master mode it processes prometheus alerts and monitors
cluster health status. In agent mode it watches Diagnoses and executes
information collection, diagnosis and recovery according to specification
of an Diagnosis.`,
		Run: func(cmd *cobra.Command, args []string) {
			setupLog.Error(opts.Run(), "failed to run kubediag")
			os.Exit(1)
		},
	}

	opts.AddFlags(cmd.Flags())

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// NewKubeDiagOptions creates a new KubeDiagOptions with a default config.
func NewKubeDiagOptions() (*KubeDiagOptions, error) {
	return &KubeDiagOptions{
		Mode:                        "agent",
		BindAddress:                 "0.0.0.0",
		Port:                        8090,
		MetricsPort:                 10357,
		EnableLeaderElection:        false,
		WebhookPort:                 9443,
		CertDir:                     defaultCertDir,
		AlertmanagerRepeatInterval:  6 * time.Hour,
		DiagnosisTTL:                240 * time.Hour,
		MinimumDiagnosisTTLDuration: 30 * time.Minute,
		MaximumDiagnosesPerNode:     20,
		DataRoot:                    defaultDataRoot,
	}, nil
}

// Run setups all controllers and starts the manager.
func (opts *KubeDiagOptions) Run() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	featureGate := features.NewFeatureGate()
	err := featureGate.SetFromMap(opts.FeatureGates)
	if err != nil {
		setupLog.Error(err, "unable to set feature gates")
		return fmt.Errorf("unable to set feature gates: %v", err)
	}

	if opts.Mode == "master" {
		setupLog.Info("kubediag is running in master mode")

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.MetricsPort),
			Port:               opts.WebhookPort,
			Host:               opts.Host,
			CertDir:            opts.CertDir,
			LeaderElection:     opts.EnableLeaderElection,
			LeaderElectionID:   "8a2b2861.kubediag.org",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			return fmt.Errorf("unable to start manager: %v", err)
		}
		// Collect feature gate metrics
		features.Collect(featureGate)

		// Channel for queuing kubernetes events and operation sets.
		eventChainCh := make(chan corev1.Event, 1000)
		graphBuilderCh := make(chan diagnosisv1.OperationSet, 1000)
		stopCh := SetupSignalHandler()

		// Create graph builder for generating graph from operation set.
		graphbuilder := graphbuilder.NewGraphBuilder(
			context.Background(),
			ctrl.Log.WithName("graphbuilder"),
			mgr.GetClient(),
			mgr.GetEventRecorderFor("kubediag/graphbuilder"),
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

		// Create kafka consumer for managing kafka messages.
		if len(opts.KafkaBrokers) != 0 && opts.KafkaTopic != "" {
			kafkaConsumer, err := kafka.NewConsumer(
				context.Background(),
				ctrl.Log.WithName("kafkaconsumer"),
				mgr.GetClient(),
				opts.KafkaBrokers,
				opts.KafkaTopic,
				featureGate.Enabled(features.KafkaConsumer),
			)
			if err != nil {
				setupLog.Error(err, "unable to create kafka consumer")
				return fmt.Errorf("unable to create kafka consumer: %v", err)
			}
			go func(stopCh chan struct{}) {
				kafkaConsumer.Run(stopCh)
			}(stopCh)
		}

		// Start http server.
		go func(stopCh chan struct{}) {
			r := mux.NewRouter()
			r.HandleFunc("/api/v1/alerts", alertmanager.Handler)

			// Start pprof server.
			r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
			if err := http.ListenAndServe(fmt.Sprintf("%s:%d", opts.BindAddress, opts.Port), r); err != nil {
				setupLog.Error(err, "unable to start http server")
				close(stopCh)
			}
		}(stopCh)

		// Setup reconcilers for Diagnosis, Trigger, Operation, OperationSet and Event.
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
		if err = (controllers.NewOperationReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Operation"),
			mgr.GetScheme(),
			opts.Mode,
			opts.DataRoot,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Operation")
			return fmt.Errorf("unable to create controller for Operation: %v", err)
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
		if err = (&diagnosisv1.OperationSet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OperationSet")
			return fmt.Errorf("unable to create webhook for OperationSet: %v", err)
		}
		// +kubebuilder:scaffold:builder

		setupLog.Info("starting manager")
		if err := mgr.Start(stopCh); err != nil {
			setupLog.Error(err, "problem running manager")
			return fmt.Errorf("problem running manager: %v", err)
		}

	} else if opts.Mode == "agent" {
		setupLog.Info("kubediag is running in agent mode")

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: fmt.Sprintf("%s:%d", opts.BindAddress, opts.MetricsPort),
			LeaderElection:     false,
			LeaderElectionID:   "8a2b2861.kubediag.org",
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
			mgr.GetEventRecorderFor("kubediag/executor"),
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

		router := mux.NewRouter()
		router.HandleFunc("/healthz", HealthCheckHandler)
		// Start pprof server.
		router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

		// Setup operation processors.
		registryOpt := &register.RegistryOption{
			NodeName:       opts.NodeName,
			DockerEndpoint: opts.DockerEndpoint,
			DataRoot:       opts.DataRoot,
			BindAddress:    opts.BindAddress,
		}
		err = register.RegisterProcessors(mgr, registryOpt, featureGate, router, setupLog)
		if err != nil {
			setupLog.Error(err, "unable to register processors")
			return fmt.Errorf("unable to register processors for Diagnosis: %v", err)
		}

		// Start http server.
		go func(stopCh chan struct{}) {
			if err := http.ListenAndServe(fmt.Sprintf("%s:%d", opts.BindAddress, opts.Port), router); err != nil {
				setupLog.Error(err, "unable to start http server")
				close(stopCh)
			}
		}(stopCh)

		// Setup reconcilers for Diagnosis and Operation.
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
		if err = (controllers.NewOperationReconciler(
			mgr.GetClient(),
			ctrl.Log.WithName("controllers").WithName("Operation"),
			mgr.GetScheme(),
			opts.Mode,
			opts.DataRoot,
		)).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Operation")
			return fmt.Errorf("unable to create controller for Operation: %v", err)
		}
		// +kubebuilder:scaffold:builder

		setupLog.Info("starting manager")
		if err := mgr.Start(stopCh); err != nil {
			setupLog.Error(err, "problem running manager")
			return fmt.Errorf("problem running manager: %v", err)
		}
	} else {
		return fmt.Errorf("invalid kubediag mode: %s", opts.Mode)
	}

	return nil
}

// AddFlags adds flags to fs and binds them to options.
func (opts *KubeDiagOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.Mode, "mode", opts.Mode, "Whether the kubediag is running as a master or an agnet.")
	fs.StringVar(&opts.BindAddress, "bind-address", opts.BindAddress, "The address on which to advertise.")
	fs.IntVar(&opts.Port, "port", opts.Port, "The port for the kubediag to serve on.")
	fs.StringVar(&opts.NodeName, "node-name", opts.NodeName, "The node name.")
	fs.IntVar(&opts.MetricsPort, "metrics-port", opts.MetricsPort, "The port the metric endpoint to serve on.")
	fs.BoolVar(&opts.EnableLeaderElection, "enable-leader-election", opts.EnableLeaderElection, "Enables leader election for kubediag master.")
	fs.StringVar(&opts.DockerEndpoint, "docker-endpoint", "unix:///var/run/docker.sock", "The docker endpoint.")
	fs.IntVar(&opts.WebhookPort, "webhook-port", opts.WebhookPort, "The port that the webhook server serves at.")
	fs.StringVar(&opts.Host, "host", opts.Host, "The hostname that the webhook server binds to.")
	fs.StringVar(&opts.CertDir, "cert-dir", opts.CertDir, "The directory that contains the server key and certificate.")
	fs.DurationVar(&opts.AlertmanagerRepeatInterval, "repeat-interval", opts.AlertmanagerRepeatInterval, "How long to wait before sending a notification again if it has already been sent successfully for an alert.")
	fs.StringSliceVar(&opts.KafkaBrokers, "kafka-brokers", opts.KafkaBrokers, "The list of broker addresses used to connect to the kafka cluster.")
	fs.StringVar(&opts.KafkaTopic, "kafka-topic", opts.KafkaTopic, "The topic to read messages from.")
	fs.DurationVar(&opts.DiagnosisTTL, "diagnosis-ttl", opts.DiagnosisTTL, "Amount of time to retain diagnoses.")
	fs.DurationVar(&opts.MinimumDiagnosisTTLDuration, "minimum-diagnosis-ttl-duration", opts.MinimumDiagnosisTTLDuration, "Minimum age for a finished diagnosis before it is garbage collected.")
	fs.Int32Var(&opts.MaximumDiagnosesPerNode, "maximum-diagnoses-per-node", opts.MaximumDiagnosesPerNode, "Maximum number of finished diagnoses to retain per node.")
	fs.Var(flag.NewMapStringBool(&opts.FeatureGates), "feature-gates", "A map of feature names to bools that enable or disable features. Options are:\n"+strings.Join(features.NewFeatureGate().KnownFeatures(), "\n"))
	fs.StringVar(&opts.DataRoot, "data-root", opts.DataRoot, "Root directory of persistent kubediag data.")
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
