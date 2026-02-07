// controllers/reducednodepolicy_controller.go
package controller

import (
	context "context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	handler "sigs.k8s.io/controller-runtime/pkg/handler"

	iotv1alpha1 "github.com/jaiderssjgod/edge-operator/api/v1alpha1"
)

type ReducedNodePolicyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *ReducedNodePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("reducednodepolicy", req.NamespacedName)

	var policy iotv1alpha1.ReducedNodePolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var nodeList corev1.NodeList
	if err := r.List(ctx, &nodeList, client.MatchingLabels(policy.Spec.NodeSelector)); err != nil {
		return ctrl.Result{}, err
	}

	offlineCount := 0
	for _, node := range nodeList.Items {
		log.Info("Procesando nodo", "name", node.Name, "labels", node.Labels)

		// Etiquetar nodo como tipo reducido si no lo está
		if node.Labels["node-type"] != "reducido" {
			if node.Labels == nil {
				node.Labels = map[string]string{}
			}
			node.Labels["node-type"] = "reducido"
			if err := r.Update(ctx, &node); err != nil {
				log.Error(err, "no se pudo etiquetar el nodo como reducido", "node", node.Name)
				continue
			}
		}

		ready := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready = true
			}
		}
		if !ready {
			offlineCount++
		}

		// Validar si hay al menos un pod crítico corriendo en este nodo
		var podList corev1.PodList
		if err := r.List(ctx, &podList, client.MatchingFields{"spec.nodeName": node.Name}); err == nil {
			foundCritical := false
			for _, pod := range podList.Items {
				if pod.Labels[policy.Spec.CriticalLabelKey] == "true" && pod.Status.Phase == corev1.PodRunning {
					foundCritical = true
					break
				}
			}
			if !foundCritical {
				log.Info("Nodo sin pods críticos activos", "node", node.Name)
			}
		}
	}

	policy.Status.ObservedNodes = len(nodeList.Items)
	policy.Status.OfflineNodes = offlineCount
	policy.Status.LastSync = metav1.NewTime(time.Now())

	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Error(err, "unable to update ReducedNodePolicy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ReducedNodePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.ReducedNodePolicy{}).
		Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
