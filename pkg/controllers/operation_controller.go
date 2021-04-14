package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	diagnosisv1 "github.com/kube-diagnoser/kube-diagnoser/api/v1"
)

// OperationReconciler reconciles a Operation object.
type OperationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewOperationReconciler creates a new OperationReconciler.
func NewOperationReconciler(
	cli client.Client,
	log logr.Logger,
	scheme *runtime.Scheme,
) *OperationReconciler {
	return &OperationReconciler{
		Client: cli,
		Log:    log,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=diagnosis.netease.com,resources=operations,verbs=get;list;watch;create;update;patch;delete

// Reconcile synchronizes a Operation object.
func (r *OperationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("operation", req.NamespacedName)

	log.Info("reconciling Operation")

	var err error
	var operation diagnosisv1.Operation
	if err = r.Get(ctx, req.NamespacedName, &operation); err != nil {
		log.Error(err, "unable to fetch Operation")
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	err = r.reconcileOperationSet(ctx, req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle of case 'Terminating':
	// Remove finalizers of deleted OperationSet if Operation in terminating
	if !operation.DeletionTimestamp.IsZero() && len(operation.GetFinalizers()) > 0 {
		err = r.cleanExpiredFinalizers(ctx, operation, log)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager setups OperationReconciler with the provided manager.
func (r *OperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&diagnosisv1.Operation{}).
		Complete(r)
}

// cleanExpiredFinalizers delete the element in the finalizers of this Operation that points to a deleted OperationSet object.
func (r *OperationReconciler) cleanExpiredFinalizers(ctx context.Context, operation diagnosisv1.Operation, log logr.Logger) error {
	finalizers := operation.GetFinalizers()
	remainingFinalizers := []string{}
	var operationSet diagnosisv1.OperationSet
	for _, finalizer := range finalizers {
		// finalizer with a non-exist-OperationSet will not be saved
		if err := r.Get(ctx, client.ObjectKey{Name: finalizer}, &operationSet); apierrors.IsNotFound(err) {
			continue
		}
		remainingFinalizers = append(remainingFinalizers, finalizer)
	}

	if len(remainingFinalizers) < len(finalizers) {
		operation.SetFinalizers(remainingFinalizers)
		if err := r.Update(ctx, &operation); err != nil {
			log.Error(err, "unable to update Operation")
			return client.IgnoreNotFound(err)
		}
	}
	return nil
}

// reconcileOperationSet will check all related OperationSet, and update their status if not accuracy
func (r *OperationReconciler) reconcileOperationSet(ctx context.Context, operationName string) error {
	osList := diagnosisv1.OperationSetList{}
	oList := diagnosisv1.OperationList{}
	err := r.List(ctx, &osList, &client.ListOptions{})
	if err != nil {
		return err
	}
	err = r.List(ctx, &oList, &client.ListOptions{})
	if err != nil {
		return err
	}
	operationActive := make(map[string]bool)
	for _, operation := range oList.Items {
		operationActive[operation.Name] = operation.DeletionTimestamp.IsZero()
	}
	for _, os := range osList.Items {
		if !contains(os.Spec.AdjacencyList, operationName) {
			continue
		}
		osShouldReady := true
		for _, node := range os.Spec.AdjacencyList {
			if node.Operation == "" {
				continue
			}
			if !operationActive[node.Operation] {
				osShouldReady = false
				break
			}
		}
		if osShouldReady != os.Status.Ready {
			os.Status.Ready = osShouldReady
			if err = r.Status().Update(ctx, &os, &client.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// contains return if node list container target operation.
func contains(nodes []diagnosisv1.Node, operation string) bool {
	for _, node := range nodes {
		if node.Operation == operation {
			return true
		}
	}
	return false
}
