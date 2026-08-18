package controllers

import (
	"context"
	"fmt"
	"time"

	schedulingv1 "github.com/8terbahn/distribute_gpu_scheduler/api/v1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	gpuWorkloadFinalizer = "scheduling.gpu-platform.io/finalizer"

	PhaseQueued    = "Queued"
	PhaseAssigned  = "Assigned"
	PhaseRunning   = "Running"
	PhaseCompleted = "Completed"
	PhaseFailed    = "Failed"
)

// GPUWorkloadReconciler reconciles GPUWorkload objects.
//
// This controller implements the Kubernetes Operator pattern (controller-runtime).
// It continuously watches GPUWorkload CRs, selects the best-fit GPUPool based on
// GPU type and available capacity, and tracks the workload lifecycle through a
// well-defined phase state machine.
//
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpuworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpuworkloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpuworkloads/finalizers,verbs=update
// +kubebuilder:rbac:groups=scheduling.gpu-platform.io,resources=gpupools,verbs=get;list;watch;update;patch
type GPUWorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// SetupWithManager registers this controller with the controller-manager.
func (r *GPUWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingv1.GPUWorkload{}).
		Complete(r)
}

// Reconcile is the core reconciliation loop. It is invoked whenever a GPUWorkload
// is created, updated, or deleted. Each call is idempotent.
//
// State machine:
//
//	(empty) -> Queued -> Assigned -> Running -> Completed
//	                                       \---> Failed
func (r *GPUWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gpuworkload", req.NamespacedName)

	// 1. Fetch the GPUWorkload resource.
	var workload schedulingv1.GPUWorkload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get GPUWorkload: %w", err)
	}

	// 2. Handle deletion: release pool capacity via finalizer before GC.
	if !workload.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, log, &workload)
	}

	// 3. Ensure the finalizer is registered (protects pool capacity on delete).
	if !controllerutil.ContainsFinalizer(&workload, gpuWorkloadFinalizer) {
		controllerutil.AddFinalizer(&workload, gpuWorkloadFinalizer)
		if err := r.Update(ctx, &workload); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Drive the phase state machine.
	switch workload.Status.Phase {
	case "", PhaseQueued:
		return r.handleQueued(ctx, log, &workload)
	case PhaseAssigned, PhaseRunning:
		// Workers report results via the scheduler REST API.
		// Re-queue periodically to observe status changes.
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	case PhaseCompleted, PhaseFailed:
		return ctrl.Result{}, nil
	default:
		log.Info("unknown phase; resetting to Queued", "phase", workload.Status.Phase)
		return r.setPhase(ctx, &workload, PhaseQueued, "unknown phase; reset by operator")
	}
}

// handleQueued finds the best GPUPool and assigns the workload to it.
func (r *GPUWorkloadReconciler) handleQueued(
	ctx context.Context,
	log logr.Logger,
	workload *schedulingv1.GPUWorkload,
) (ctrl.Result, error) {
	var poolList schedulingv1.GPUPoolList
	if err := r.List(ctx, &poolList, client.InNamespace(workload.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list GPUPools: %w", err)
	}

	// Select the pool that matches the GPU type and has the most free capacity.
	var best *schedulingv1.GPUPool
	for i := range poolList.Items {
		pool := &poolList.Items[i]
		if pool.Spec.GPUType != workload.Spec.GPURequirement {
			continue
		}
		if pool.Status.AvailableUnits < 1 {
			continue
		}
		if best == nil || pool.Status.AvailableUnits > best.Status.AvailableUnits {
			best = pool
		}
	}

	if best == nil {
		log.Info("no suitable GPUPool available; will retry",
			"gpuType", workload.Spec.GPURequirement)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Reserve one unit in the winning pool.
	best.Status.AvailableUnits--
	best.Status.AllocatedUnits++
	if err := r.Status().Update(ctx, best); err != nil {
		return ctrl.Result{}, fmt.Errorf("update GPUPool status: %w", err)
	}

	now := metav1.Now()
	workload.Status.AssignedWorker = best.Name
	workload.Status.StartTime = &now
	log.Info("workload assigned", "pool", best.Name)
	return r.setPhase(ctx, workload, PhaseAssigned,
		fmt.Sprintf("assigned to pool %s", best.Name))
}

// handleDeletion releases reserved pool capacity and removes the finalizer.
func (r *GPUWorkloadReconciler) handleDeletion(
	ctx context.Context,
	log logr.Logger,
	workload *schedulingv1.GPUWorkload,
) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(workload, gpuWorkloadFinalizer) {
		if workload.Status.AssignedWorker != "" &&
			workload.Status.Phase != PhaseCompleted &&
			workload.Status.Phase != PhaseFailed {
			var pool schedulingv1.GPUPool
			key := client.ObjectKey{Namespace: workload.Namespace, Name: workload.Status.AssignedWorker}
			if err := r.Get(ctx, key, &pool); err == nil {
				pool.Status.AllocatedUnits = maxInt(0, pool.Status.AllocatedUnits-1)
				pool.Status.AvailableUnits = pool.Spec.TotalUnits - pool.Status.AllocatedUnits
				if err := r.Status().Update(ctx, &pool); err != nil {
					log.Error(err, "failed to release pool capacity on delete (best-effort)")
				}
			}
		}
		controllerutil.RemoveFinalizer(workload, gpuWorkloadFinalizer)
		if err := r.Update(ctx, workload); err != nil {
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}
	}
	return ctrl.Result{}, nil
}

// setPhase is a convenience helper that updates status.Phase and status.Message.
func (r *GPUWorkloadReconciler) setPhase(
	ctx context.Context,
	workload *schedulingv1.GPUWorkload,
	phase, message string,
) (ctrl.Result, error) {
	workload.Status.Phase = phase
	workload.Status.Message = message
	if err := r.Status().Update(ctx, workload); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status (phase=%s): %w", phase, err)
	}
	return ctrl.Result{}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
