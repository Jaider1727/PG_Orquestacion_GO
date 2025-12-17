/*
Copyright 2025.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	iotv1alpha1 "edge-operator/api/v1alpha1"
)

// EdgeNodeStatusReconciler reconciles a EdgeNodeStatus object
type EdgeNodeStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=iot.iot.example.com,resources=edgenodestatuses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iot.iot.example.com,resources=edgenodestatuses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iot.iot.example.com,resources=edgenodestatuses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EdgeNodeStatus object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *EdgeNodeStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var nodeStatus iotv1alpha1.EdgeNodeStatus
	if err := r.Get(ctx, req.NamespacedName, &nodeStatus); err != nil {
		log.Error(err, "no se pudo obtener EdgeNodeStatus")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciliando nodo", "name", nodeStatus.Name, "connected", nodeStatus.Status.Connected, "battery", nodeStatus.Status.BatteryLevel)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EdgeNodeStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.EdgeNodeStatus{}).
		Complete(r)
}
