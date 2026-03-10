// internal/degradation/degradation.go
package degradation

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// PriorityLabelKey es la etiqueta que clasifica la prioridad del pod.
	PriorityLabelKey = "edge.priority"
	// PriorityNonCritical es el valor que identifica pods no críticos.
	PriorityNonCritical = "non-critical"
)

// Manager se encarga de reducir la carga de trabajo no crítica en un nodo degradado.
type Manager struct {
	Client client.Client
	Log    logr.Logger
}

// New crea un Manager de degradación listo para usar.
func New(c client.Client, log logr.Logger) *Manager {
	return &Manager{Client: c, Log: log}
}

// EvictNonCriticalPods lista y elimina todos los pods con label
// edge.priority=non-critical que corren en el nodo indicado.
// Pods sin la etiqueta edge.priority nunca son tocados.
func (m *Manager) EvictNonCriticalPods(ctx context.Context, nodeName string) error {
	log := m.Log.WithValues("node", nodeName)

	var podList corev1.PodList
	if err := m.Client.List(ctx, &podList,
		client.MatchingFields{"spec.nodeName": nodeName},
	); err != nil {
		log.Error(err, "Error al listar pods del nodo")
		return err
	}

	evicted := 0
	for i := range podList.Items {
		pod := &podList.Items[i]

		priority, hasPriorityLabel := pod.Labels[PriorityLabelKey]
		// Restricción: no tocar pods sin la etiqueta
		if !hasPriorityLabel {
			continue
		}
		// No tocar pods críticos
		if priority != PriorityNonCritical {
			continue
		}
		// Pods en fase terminal ya no necesitan acción
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		log.Info("Eliminando pod no crítico", "pod", pod.Name, "namespace", pod.Namespace)
		if err := m.Client.Delete(ctx, pod); err != nil {
			log.Error(err, "No se pudo eliminar pod no crítico", "pod", pod.Name)
			// Continuar con el resto aunque uno falle
			continue
		}
		evicted++
	}

	log.Info("Degradación completada", "podsEviccionados", evicted)
	return nil
}