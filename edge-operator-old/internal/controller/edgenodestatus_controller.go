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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	iotv1alpha1 "github.com/jaiderssj/edge-operator/api/v1alpha1"
)

// EdgeNodeStatusReconciler reconciles a EdgeNodeStatus object
type EdgeNodeStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=iot.example.com,resources=edgenodestatuses,verbs=get;list;watch;update;patch

func (r *EdgeNodeStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var node iotv1alpha1.EdgeNodeStatus
	err := r.Get(ctx, req.NamespacedName, &node)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("EdgeNodeStatus eliminado", "name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("📡 Reconciliando nodo",
		"name", node.Name,
		"connected", node.Status.Connected,
		"battery", node.Status.BatteryLevel,
		"cpu", node.Status.CPUUsage,
	)

	return ctrl.Result{}, nil
}

func (r *EdgeNodeStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.EdgeNodeStatus{}).
		Complete(r)
}
