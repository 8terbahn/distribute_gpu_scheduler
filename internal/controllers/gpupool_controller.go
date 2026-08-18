package controllers

import (
	"context"
	"fmt"

	schedulingv1 "github.com/8terbahn/distribute_gpu_scheduler/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GPUPoolReconciler keeps GPUPool capacity accurate by counting active GPUWorkloads.
//
// It is intentionally simple: on every reconcile it recomputes AllocatedUnits by
// scanning all GPUWorkloads in the namespace and counting those assigned to this pool.
// This makes the pool status eventually-consistent and self-correcting after failures.
//
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpupools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpupools/status,verbs=get;update;patch
type GPUPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager registers this controller with the controller-manager.
func (r *GPUPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingv1.GPUPool{}).
		Complete(r)
}

// Reconcile recomputes pool capacity from the live set of active GPUWorkloads.
func (r *GPUPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pool schedulingv1.GPUPool
	if err := r.Get(ctx, req.NamespacedName, &pool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get GPUPool: %w", err)
	}

	// Count non-terminal workloads assigned to this pool.
	var workloadList schedulingv1.GPUWorkloadList
	if err := r.List(ctx, &workloadList, client.InNamespace(pool.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list GPUWorkloads: %w", err)
	}

	allocated := 0
	for _, w := range workloadList.Items {
		if w.Status.AssignedWorker == pool.Name &&
			w.Status.Phase != PhaseCompleted &&
			w.Status.Phase != PhaseFailed {
			allocated++
		}
	}

	available := pool.Spec.TotalUnits - allocated
	if available < 0 {
		available = 0
	}

	if pool.Status.AllocatedUnits != allocated || pool.Status.AvailableUnits != available {
		pool.Status.AllocatedUnits = allocated
		pool.Status.AvailableUnits = available
		pool.Status.Message = fmt.Sprintf("%d/%d units available", available, pool.Spec.TotalUnits)
		if err := r.Status().Update(ctx, &pool); err != nil {
			return ctrl.Result{}, fmt.Errorf("update GPUPool status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
