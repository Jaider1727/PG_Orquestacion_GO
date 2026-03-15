// internal/degradation/manager.go
package degradation

import (
    "context"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

// ScaleDownNonCriticalDeployments escala a 0 los Deployments no críticos
// cuyos pods corren en el nodo indicado.
// Usa la misma lógica de labels que EvictNonCriticalPods: edge.priority=non-critical
func (m *Manager) ScaleDownNonCriticalDeployments(ctx context.Context, nodeName, _ string) error {
    deployments, err := m.findNonCriticalDeployments(ctx, nodeName)
    if err != nil {
        return err
    }

    for i := range deployments {
        deploy := &deployments[i]
        if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
            continue
        }
        zero := int32(0)
        deploy.Spec.Replicas = &zero
        if err := m.Client.Update(ctx, deploy); err != nil {
            m.Log.Error(err, "Error escalando deployment a 0", "deployment", deploy.Name)
            continue
        }
        m.Log.Info("Deployment escalado a 0 por umbral de recursos",
            "deployment", deploy.Name,
            "node", nodeName,
        )
    }
    return nil
}

// ScaleUpNonCriticalDeployments restaura a 1 los Deployments escalados a 0.
func (m *Manager) ScaleUpNonCriticalDeployments(ctx context.Context, nodeName, _ string) error {
    deployments, err := m.findNonCriticalDeployments(ctx, nodeName)
    if err != nil {
        return err
    }

    for i := range deployments {
        deploy := &deployments[i]
        if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 0 {
            continue
        }
        one := int32(1)
        deploy.Spec.Replicas = &one
        if err := m.Client.Update(ctx, deploy); err != nil {
            m.Log.Error(err, "Error restaurando deployment", "deployment", deploy.Name)
            continue
        }
        m.Log.Info("Deployment restaurado por normalización de recursos",
            "deployment", deploy.Name,
            "node", nodeName,
        )
    }
    return nil
}

// findNonCriticalDeployments busca Deployments no críticos con pods en el nodo.
// Usa edge.priority=non-critical igual que EvictNonCriticalPods.
func (m *Manager) findNonCriticalDeployments(ctx context.Context, nodeName string) ([]appsv1.Deployment, error) {
    var podList corev1.PodList
    if err := m.Client.List(ctx, &podList,
        client.MatchingFields{"spec.nodeName": nodeName},
    ); err != nil {
        return nil, err
    }

    seen := map[string]bool{}
    var result []appsv1.Deployment

    for _, pod := range podList.Items {
        // Usar la misma lógica que EvictNonCriticalPods
        priority, hasPriorityLabel := pod.Labels[PriorityLabelKey]
        if !hasPriorityLabel || priority != PriorityNonCritical {
            continue
        }

        // Buscar el Deployment dueño via label app
        deployName := pod.Labels["app"]
        if deployName == "" || seen[deployName] {
            continue
        }
        seen[deployName] = true

        var deploy appsv1.Deployment
        if err := m.Client.Get(ctx, client.ObjectKey{
            Name:      deployName,
            Namespace: pod.Namespace,
        }, &deploy); err != nil {
            continue
        }
        result = append(result, deploy)
    }
    return result, nil
}