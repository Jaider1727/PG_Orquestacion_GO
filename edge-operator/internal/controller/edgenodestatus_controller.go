package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		log.Error(err, "❌ No se pudo obtener EdgeNodeStatus")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeName := nodeStatus.Spec.NodeName
	connected := nodeStatus.Status.Connected
	lastHb := nodeStatus.Status.LastHeartbeat

	log.Info("🔁 Nodo actualizado", "name", nodeName, "connected", connected, "battery", nodeStatus.Status.BatteryLevel)

	t, err := time.Parse(time.RFC3339, lastHb)
	if err != nil {
		log.Error(err, "❌ Formato de LastHeartbeat inválido", "value", lastHb)
		return ctrl.Result{}, nil
	}

	ahora := time.Now().UTC()
	hbAntiguo := ahora.Sub(t) > 1*time.Minute

	var node corev1.Node
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, &node); err != nil {
		log.Error(err, "❌ No se pudo obtener el Node real")
		return ctrl.Result{}, nil
	}

	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	// 🔴 Marcar como desconectado si aplica
	if !connected || hbAntiguo {
		log.Info("⚠️ Nodo considerado desconectado", "últimoHeartbeat", lastHb)

		if node.Annotations["iot.example.com/disconnected"] != "true" {
			node.Annotations["iot.example.com/disconnected"] = "true"
			if err := r.Update(ctx, &node); err != nil {
				log.Error(err, "❌ No se pudo anotar como desconectado")
				return ctrl.Result{}, err
			}
			log.Info("✅ Nodo anotado como desconectado", "nodeName", nodeName)
		}
	} else {
		// 🟢 Eliminar anotación si se reconectó
		if node.Annotations["iot.example.com/disconnected"] == "true" {
			delete(node.Annotations, "iot.example.com/disconnected")
			if err := r.Update(ctx, &node); err != nil {
				log.Error(err, "❌ No se pudo remover anotación de desconexión")
				return ctrl.Result{}, err
			}
			log.Info("🔄 Nodo reconectado, anotación eliminada", "nodeName", nodeName)
		}
	}

	// 🏷️ Etiquetar nodo según spec.nodeType
	nodeType := nodeStatus.Spec.NodeType
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	if node.Labels["iot.example.com/node-type"] != nodeType {
		node.Labels["iot.example.com/node-type"] = nodeType
		if err := r.Update(ctx, &node); err != nil {
			log.Error(err, "❌ No se pudo actualizar label del nodo")
			return ctrl.Result{}, err
		}
		log.Info("🏷️ Nodo etiquetado según tipo", "nodeName", nodeName, "tipo", nodeType)
	}

	return ctrl.Result{}, nil
}

func (r *EdgeNodeStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&iotv1alpha1.EdgeNodeStatus{}).
		Complete(r)
}
