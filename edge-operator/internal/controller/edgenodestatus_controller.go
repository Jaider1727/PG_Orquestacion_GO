package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	iotv1alpha1 "edge-operator/api/v1alpha1"
)

type EdgeNodeStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *EdgeNodeStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var nodeStatus iotv1alpha1.EdgeNodeStatus
	if err := r.Get(ctx, req.NamespacedName, &nodeStatus); err != nil {
		log.Error(err, "no se pudo obtener EdgeNodeStatus")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("🔁 Nodo actualizado",
		"name", nodeStatus.Name,
		"connected", nodeStatus.Status.Connected,
		"battery", nodeStatus.Status.BatteryLevel,
		"cpu", nodeStatus.Status.CPUUsage,
		"criticalPods", nodeStatus.Status.CriticalPods,
	)

	return ctrl.Result{}, nil
}

func (r *EdgeNodeStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.EdgeNodeStatus{}).
		Complete(r)
}
