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
	"github.com/kubediag/kubediag/pkg/util"
)

const (
	// ScriptSubDirectory is the directory under kubediag data root for storing scripts.
	ScriptSubDirectory = "scripts"
	// FunctionSubDirectory is the directory under kubediag data root for storing functions.
	FunctionSubDirectory = "functions"
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

	mode                string
	dataRoot            string
	python3MainFilePath string
}

// NewOperationReconciler creates a new OperationReconciler.
func NewOperationReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
	mode string,
	dataRoot string,
	python3MainFilePath string,
) *OperationReconciler {
	if mode == "master" {
		metrics.Registry.MustRegister(
			operationInfo,
		)
	}

	return &OperationReconciler{
		Client:              cli,
		Log:                 log,
		Scheme:              scheme,
		mode:                mode,
		dataRoot:            dataRoot,
		python3MainFilePath: python3MainFilePath,
	}
}

// +kubebuilder:rbac:groups=diagnosis.kubediag.org,resources=operations,verbs=get;list;watch;create;update;patch;delete

// Reconcile synchronizes an Operation object.
func (r *OperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
				_, err := os.Stat(scriptFilePath)
				if !os.IsNotExist(err) {
					err := os.RemoveAll(scriptFilePath)
					if err != nil {
						log.Error(err, "failed to remove script file", "filepath", scriptFilePath)
						return ctrl.Result{}, nil
					}
				}

				functionDirectory := filepath.Join(r.dataRoot, FunctionSubDirectory, req.Name)
				_, err = os.Stat(functionDirectory)
				if !os.IsNotExist(err) {
					err = os.RemoveAll(functionDirectory)
					if err != nil {
						log.Error(err, "failed to remove function directory", "filepath", functionDirectory)
						return ctrl.Result{}, nil
					}
				}

				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, err
		}

		// Update the script if the operation is a script runner.
		if operation.Spec.Processor.ScriptRunner != nil {
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
					err = os.RemoveAll(scriptFilePath)
					if err != nil {
						log.Error(err, "failed to remove file", "filepath", scriptFilePath)
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, nil
				}
			}

			err = util.CreateFile(scriptFilePath, operation.Spec.Processor.ScriptRunner.Script)
			if err != nil {
				log.Error(err, "failed to create script file", "filepath", scriptFilePath)
				return ctrl.Result{}, err
			}

			err = os.Chmod(scriptFilePath, os.ModePerm)
			if err != nil {
				log.Error(err, "failed to change file mode", "filepath", scriptFilePath)
				return ctrl.Result{}, err
			}
		} else if operation.Spec.Processor.Function != nil {
			// Create function directory if not exists.
			functionDirectory := filepath.Join(r.dataRoot, FunctionSubDirectory, operation.Name)
			_, err := os.Stat(functionDirectory)
			if os.IsNotExist(err) {
				err := os.MkdirAll(functionDirectory, os.ModePerm)
				if err != nil {
					log.Error(err, "failed to create function directory", "filepath", functionDirectory)
					return ctrl.Result{}, err
				}
			}

			// Read content of python3 main file template.
			mainFileTemplate, err := ioutil.ReadFile(r.python3MainFilePath)
			if err != nil {
				log.Error(err, "failed to read python3 main file template", "filepath", r.python3MainFilePath)
				return ctrl.Result{}, err
			}

			// Update content in main file.
			mainFilePath := filepath.Join(functionDirectory, "main.py")
			_, err = os.Stat(mainFilePath)
			if os.IsNotExist(err) {
				err := util.CreateFile(mainFilePath, string(mainFileTemplate))
				if err != nil {
					log.Error(err, "failed to create main file", "filepath", mainFilePath)
					return ctrl.Result{}, err
				}
			} else {
				// Get the content of existing main file.
				content, err := ioutil.ReadFile(mainFilePath)
				if err != nil {
					log.Error(err, "failed to read main file", "filepath", mainFilePath)
					return ctrl.Result{}, err
				}

				// Remove the existing main file if it is not equal to main file template.
				// Create a new main file according to main file template.
				if !reflect.DeepEqual(string(content), string(mainFileTemplate)) {
					err = os.RemoveAll(mainFilePath)
					if err != nil {
						log.Error(err, "failed to remove file", "filepath", mainFilePath)
						return ctrl.Result{}, err
					}

					err := util.CreateFile(mainFilePath, string(mainFileTemplate))
					if err != nil {
						log.Error(err, "failed to create main file", "filepath", mainFilePath)
						return ctrl.Result{}, err
					}
				}
			}

			files, err := ioutil.ReadDir(functionDirectory)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Update all function files with defined code source.
			for key, value := range operation.Spec.Processor.Function.CodeSource {
				found := false
				for _, file := range files {
					filename := file.Name()
					functionFilePath := filepath.Join(functionDirectory, filename)

					// Continue if the file name is "main.py".
					if filename == "main.py" {
						continue
					}

					// Delete the existing function file if it is not found in code source.
					if _, ok := operation.Spec.Processor.Function.CodeSource[filename]; !ok {
						// Continue if the file does not exist.
						_, err := os.Stat(functionFilePath)
						if os.IsNotExist(err) {
							continue
						}

						err = os.RemoveAll(functionFilePath)
						if err != nil {
							log.Error(err, "failed to remove file", "filepath", functionFilePath)
							return ctrl.Result{}, err
						}
						continue
					}

					// Continue if the key does not match file name.
					if filename != key {
						continue
					}

					// Get the content of existing function file.
					content, err := ioutil.ReadFile(functionFilePath)
					if err != nil {
						log.Error(err, "failed to read function file", "filepath", functionFilePath)
						return ctrl.Result{}, err
					}

					// Remove the existing function file if it is not equal to the defined code source.
					if !reflect.DeepEqual(string(content), operation.Spec.Processor.Function.CodeSource[filename]) {
						err = os.RemoveAll(functionFilePath)
						if err != nil {
							log.Error(err, "failed to remove file", "filepath", functionFilePath)
							return ctrl.Result{}, err
						}
					} else {
						found = true
					}
				}

				// Create the function file if the code source is not found in function directory.
				if !found {
					functionFilePath := filepath.Join(functionDirectory, key)
					err := util.CreateFile(functionFilePath, value)
					if err != nil {
						log.Error(err, "failed to create function file", "filepath", functionFilePath)
						return ctrl.Result{}, err
					}
				}
			}
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

// collectOperationMetrics collects metrics with operation info.
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
