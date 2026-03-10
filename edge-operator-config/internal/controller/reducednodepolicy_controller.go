// internal/controller/reducednodepolicy_controller.go
package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	iotv1alpha1 "github.com/jaiderssjgod/edge-operator/api/v1alpha1"
	"github.com/jaiderssjgod/edge-operator/internal/degradation"
	"github.com/jaiderssjgod/edge-operator/internal/heartbeatstore"
)

const (
	heartbeatTimeout = 30 * time.Second
	requeueInterval  = 15 * time.Second
)

// ReducedNodePolicyReconciler reconcilia objetos ReducedNodePolicy.
type ReducedNodePolicyReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	HeartbeatStore     *heartbeatstore.Store
	DegradationManager *degradation.Manager // ← nuevo campo inyectado
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

	if policy.Status.Nodes == nil {
		policy.Status.Nodes = make(map[string]iotv1alpha1.NodeHeartbeatStatus)
	}

	offlineCount := 0

	for _, node := range nodeList.Items {
		log.Info("Procesando nodo", "name", node.Name)

		if err := r.ensureNodeLabeled(ctx, log, &node); err != nil {
			continue
		}

		nodeState := r.HeartbeatStore.GetNodeState(node.Name)
		state := "online"
		if nodeState.Offline {
			state = "offline"
			offlineCount++
			log.Info("Nodo OFFLINE detectado", "node", node.Name)

			// RF-04: degradar pods no críticos cuando el nodo está offline
			if err := r.DegradationManager.EvictNonCriticalPods(ctx, node.Name); err != nil {
				log.Error(err, "Error durante la degradación del nodo", "node", node.Name)
				// No bloqueamos la reconciliación por un error de degradación
			}
		}

		hbStatus := iotv1alpha1.NodeHeartbeatStatus{
			State:  state,
			CPU:    nodeState.CPU,
			Memory: nodeState.Memory,
		}
		if !nodeState.LastHeartbeat.IsZero() {
			hbStatus.LastHeartbeat = metav1.NewTime(nodeState.LastHeartbeat)
		}
		policy.Status.Nodes[node.Name] = hbStatus

		r.checkCriticalPods(ctx, log, &policy, node.Name)
	}

	policy.Status.ObservedNodes = len(nodeList.Items)
	policy.Status.OfflineNodes = offlineCount
	policy.Status.LastSync = metav1.NewTime(time.Now())

	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Error(err, "Unable to update ReducedNodePolicy status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// ensureNodeLabeled y checkCriticalPods sin cambios...
func (r *ReducedNodePolicyReconciler) ensureNodeLabeled(
	ctx context.Context, log logr.Logger, node *corev1.Node,
) error {
	if node.Labels["node-type"] == "reducido" {
		return nil
	}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	node.Labels["node-type"] = "reducido"
	if err := r.Update(ctx, node); err != nil {
		log.Error(err, "No se pudo etiquetar el nodo", "node", node.Name)
		return err
	}
	return nil
}

func (r *ReducedNodePolicyReconciler) checkCriticalPods(
	ctx context.Context, log logr.Logger,
	policy *iotv1alpha1.ReducedNodePolicy, nodeName string,
) {
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return
	}
	for _, pod := range podList.Items {
		if pod.Labels[policy.Spec.CriticalLabelKey] == "true" &&
			pod.Status.Phase == corev1.PodRunning {
			return
		}
	}
	log.Info("Nodo sin pods críticos activos", "node", nodeName)
}

func (r *ReducedNodePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
    // Registrar el índice field:spec.nodeName para poder filtrar pods por nodo
    if err := mgr.GetFieldIndexer().IndexField(
        context.Background(),
        &corev1.Pod{},
        "spec.nodeName",
        func(obj client.Object) []string {
            pod := obj.(*corev1.Pod)
            if pod.Spec.NodeName == "" {
                return nil
            }
            return []string{pod.Spec.NodeName}
        },
    ); err != nil {
        return err
    }

    return ctrl.NewControllerManagedBy(mgr).
        For(&iotv1alpha1.ReducedNodePolicy{}).
        Watches(&corev1.Node{}, &handler.EnqueueRequestForObject{}).
        Complete(r)
}