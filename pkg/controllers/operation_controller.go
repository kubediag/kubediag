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

package controllers

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	diagnosisv1 "github.com/kubediag/kubediag/api/v1"
)

const (
	// ScriptSubDirectory is the directory under kubediag data root for storing scripts.
	ScriptSubDirectory = "scripts"
)

var (
	operationInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "operation_info",
			Help: "Information about Operation",
		},
		[]string{"name"},
	)
)

// OperationReconciler reconciles a Operation object.
type OperationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	mode     string
	dataRoot string
}

// NewOperationReconciler creates a new OperationReconciler.
func NewOperationReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	mode string,
	dataRoot string,
) *OperationReconciler {
	if mode == "master" {
		metrics.Registry.MustRegister(
			operationInfo,
		)
	}

	return &OperationReconciler{
		Client:   cli,
		Log:      log,
		Scheme:   scheme,
		mode:     mode,
		dataRoot: dataRoot,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=Operations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=Operations/status,verbs=get;update;patch

// Reconcile synchronizes an Operation object.
func (r *OperationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("operation", req.NamespacedName)

	// The master will keep collecting metrics for operations, while the agent will process operations of script runner type.
	if r.mode == "master" {
		r.collectOperationMetrics(ctx, log)
	} else if r.mode == "agent" {
		var operation diagnosisv1.Operation
		if err := r.Get(ctx, req.NamespacedName, &operation); err != nil {
			log.Error(err, "unable to fetch Operation")

			// Remove script file if the operation is deleted.
			if apierrors.IsNotFound(err) {
				scriptFilePath := filepath.Join(r.dataRoot, ScriptSubDirectory, req.Name)
				err := os.Remove(scriptFilePath)
				if err != nil {
					log.Error(err, "failed to remove file", "filepath", scriptFilePath)
					return ctrl.Result{}, nil
				}
				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, err
		}

		// Ignore operation if it is not a script runner.
		if operation.Spec.Processor.ScriptRunner == nil {
			return ctrl.Result{}, nil
		}

		// Create script directory if not exists.
		scriptDirectory := filepath.Join(r.dataRoot, ScriptSubDirectory)
		_, err := os.Stat(scriptDirectory)
		if os.IsNotExist(err) {
			err := os.MkdirAll(scriptDirectory, os.ModePerm)
			if err != nil {
				log.Error(err, "failed to create script directory", "filepath", scriptDirectory)
				return ctrl.Result{}, err
			}
		}

		// Check if the script file exists.
		scriptFilePath := filepath.Join(scriptDirectory, operation.Name)
		_, err = os.Stat(scriptFilePath)
		if !os.IsNotExist(err) {
			content, err := ioutil.ReadFile(scriptFilePath)
			if err != nil {
				log.Error(err, "failed to read script file", "filepath", scriptFilePath)
				return ctrl.Result{}, err
			}

			// Remove script file if the script has been changed. Otherwise return.
			if !reflect.DeepEqual(string(content), operation.Spec.Processor.ScriptRunner.Script) {
				err = os.Remove(scriptFilePath)
				if err != nil {
					log.Error(err, "failed to remove file", "filepath", scriptFilePath)
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, nil
			}
		}

		file, err := os.Create(scriptFilePath)
		if err != nil {
			log.Error(err, "failed to create file", "filepath", scriptFilePath)
			return ctrl.Result{}, err
		}
		defer file.Close()

		_, err = file.WriteString(operation.Spec.Processor.ScriptRunner.Script)
		if err != nil {
			log.Error(err, "failed to write script to file", "filepath", scriptFilePath)
			return ctrl.Result{}, err
		}

		err = os.Chmod(scriptFilePath, os.ModePerm)
		if err != nil {
			log.Error(err, "failed to change file mode", "filepath", scriptFilePath)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager setups OperationReconciler with the provided manager.
func (r *OperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Operation{}).
		Complete(r)
}

func (r *OperationReconciler) collectOperationMetrics(ctx context.Context, log logr.Logger) {
	var operationList diagnosisv1.OperationList
	err := r.Client.List(ctx, &operationList)
	if err != nil {
		log.Error(err, "error in collect Operation metrics")
		return
	}

	operationInfo.Reset()
	for _, op := range operationList.Items {
		operationInfo.WithLabelValues(op.Name).Set(1)
	}
	log.Info("collected operation metrics.")
}
