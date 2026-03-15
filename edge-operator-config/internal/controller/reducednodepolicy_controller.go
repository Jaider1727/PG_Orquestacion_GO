package controller

import (
    "context"
    "os"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/go-logr/logr"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    controller "sigs.k8s.io/controller-runtime/pkg/controller"
    appsv1 "k8s.io/api/apps/v1"

    iotv1alpha1 "github.com/jaiderssjgod/edge-operator/api/v1alpha1"
    "github.com/jaiderssjgod/edge-operator/internal/degradation"
    "github.com/jaiderssjgod/edge-operator/internal/heartbeatstore"
)

const (
    heartbeatTimeout        = 30 * time.Second
    requeueInterval         = 15 * time.Second
    defaultGracePeriodSecs  = 60
)

// gracePeriod lee GRACE_PERIOD_SECONDS del entorno; si no existe usa el default.
func gracePeriod() time.Duration {
    if raw := os.Getenv("GRACE_PERIOD_SECONDS"); raw != "" {
        if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
            return time.Duration(secs) * time.Second
        }
    }
    return defaultGracePeriodSecs * time.Second
}

// ReducedNodePolicyReconciler reconcilia objetos ReducedNodePolicy.
type ReducedNodePolicyReconciler struct {
    client.Client
    Log                logr.Logger
    Scheme             *runtime.Scheme
    HeartbeatStore     *heartbeatstore.Store
    DegradationManager *degradation.Manager
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
    gp := gracePeriod()

    for _, node := range nodeList.Items {
        log.Info("Procesando nodo", "name", node.Name)

        if err := r.ensureNodeLabeled(ctx, log, &node); err != nil {
            continue
        }

        nodeState := r.HeartbeatStore.GetNodeState(node.Name)
        existing := policy.Status.Nodes[node.Name]

        var hbStatus iotv1alpha1.NodeHeartbeatStatus

        if nodeState.Offline {
            offlineCount++
            hbStatus = r.handleOfflineNode(ctx, log, existing, node.Name, nodeState, gp)
        } else {
            // Nodo online: limpiar estado offline previo
            hbStatus = iotv1alpha1.NodeHeartbeatStatus{
                State:  "online",
                CPU:    nodeState.CPU,
                Memory: nodeState.Memory,
            }
            if !nodeState.LastHeartbeat.IsZero() {
                hbStatus.LastHeartbeat = metav1.NewTime(nodeState.LastHeartbeat)
            }
            hbStatus.OfflineEvents = existing.OfflineEvents
            // Preservar el flag de degradación por recursos del ciclo anterior
// Usar estado real de deployments como fuente de verdad
            hbStatus.ResourceDegradationExecuted = existing.ResourceDegradationExecuted || 
                r.hasScaledDownDeployments(ctx)

            if existing.State == "offline" {
                log.Info("Nodo recuperado antes de que expirara el grace period",
                    "node", node.Name)
                log.Info("Node reconnected after offline period",
                    "node", node.Name,
                    "offlineDuration", time.Since(existing.OfflineSince.Time))
            }

            // Evaluar umbrales de recursos
            r.checkResourceThresholds(ctx, log, &policy, node.Name, &hbStatus)
            // OfflineSince y DegradationExecuted quedan en zero value → reset implícito
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

// handleOfflineNode gestiona la lógica de grace period para un nodo offline.
// Retorna el NodeHeartbeatStatus actualizado.
func (r *ReducedNodePolicyReconciler) handleOfflineNode(
    ctx context.Context,
    log logr.Logger,
    existing iotv1alpha1.NodeHeartbeatStatus,
    nodeName string,
    nodeState heartbeatstore.NodeState, // ajusta al tipo real de tu store
    gp time.Duration,
) iotv1alpha1.NodeHeartbeatStatus {

    now := time.Now()

    // Determinar offlineSince: conservar el existente o fijar ahora
    offlineSince := existing.OfflineSince
    if existing.State != "offline" || offlineSince.IsZero() {
        // Primera vez que detectamos el nodo offline en este evento
        offlineSince = metav1.NewTime(now)
        log.Info("Nodo OFFLINE detectado, iniciando grace period",
            "node", nodeName,
            "offlineSince", offlineSince,
            "gracePeriod", gp,
        )
    }

    hbStatus := iotv1alpha1.NodeHeartbeatStatus{
        State:               "offline",
        CPU:                 nodeState.CPU,
        Memory:              nodeState.Memory,
        OfflineSince:        offlineSince,
        DegradationExecuted: existing.DegradationExecuted,
    }
    if !nodeState.LastHeartbeat.IsZero() {
        hbStatus.LastHeartbeat = metav1.NewTime(nodeState.LastHeartbeat)
    }

    // RF-05: solo degradar si el grace period ya expiró y no se ha degradado antes
    offlineDuration := now.Sub(offlineSince.Time)
    switch {
    case existing.DegradationExecuted:
        log.Info("Degradación ya ejecutada para este evento offline, omitiendo",
            "node", nodeName)
        hbStatus.OfflineEvents = existing.OfflineEvents

    case offlineDuration >= gp:
        log.Info("Grace period expirado, ejecutando degradación",
            "node", nodeName,
            "offlineDuration", offlineDuration,
            "gracePeriod", gp,
        )
        if err := r.DegradationManager.EvictNonCriticalPods(ctx, nodeName); err != nil {
            log.Error(err, "Error durante la degradación del nodo", "node", nodeName)
            // No marcamos DegradationExecuted para poder reintentar
        } else {
            hbStatus.DegradationExecuted = true
                // RF-06: registrar el evento offline con su duración
            offlineEvent := fmt.Sprintf("offline from %s, degraded at %s (duration: %s)",
                offlineSince.Time.Format(time.RFC3339),
                time.Now().UTC().Format(time.RFC3339),
                offlineDuration.Round(time.Second).String(),
            )
            hbStatus.OfflineEvents = append(existing.OfflineEvents, offlineEvent)
        }

    default:
        remaining := gp - offlineDuration
        log.Info("Nodo offline dentro del grace period, esperando",
            "node", nodeName,
            "offlineDuration", offlineDuration,
            "remaining", remaining,
        )
    }

    return hbStatus
}

// ensureNodeLabeled garantiza que el nodo tenga la etiqueta "node-type=reducido".
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

// checkCriticalPods verifica que el nodo tenga al menos un pod crítico activo.
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

// parsePercent convierte "85.41%" → 85.41
func parsePercent(s string) float64 {
    s = strings.TrimSuffix(s, "%")
    v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
    if err != nil {
        return 0
    }
    return v
}

// checkResourceThresholds evalúa si CPU o memoria superan los umbrales
// definidos en la policy y ejecuta degradación si corresponde.
func (r *ReducedNodePolicyReconciler) checkResourceThresholds(
    ctx context.Context,
    log logr.Logger,
    policy *iotv1alpha1.ReducedNodePolicy,
    nodeName string,
    hbStatus *iotv1alpha1.NodeHeartbeatStatus,
) {
    if policy.Spec.MaxCPUThreshold == 0 && policy.Spec.MaxMemoryThreshold == 0 {
        return
    }

    cpu := parsePercent(hbStatus.CPU)
    mem := parsePercent(hbStatus.Memory)

    cpuExceeded := policy.Spec.MaxCPUThreshold > 0 && cpu >= float64(policy.Spec.MaxCPUThreshold)
    memExceeded := policy.Spec.MaxMemoryThreshold > 0 && mem >= float64(policy.Spec.MaxMemoryThreshold)

    if !cpuExceeded && !memExceeded {
        // Recursos normalizados → restaurar deployments si estaban escalados a 0
        if hbStatus.ResourceDegradationExecuted || r.hasScaledDownDeployments(ctx) {
                log.Info("Recursos normalizados, restaurando deployments", "node", nodeName)
                if err := r.DegradationManager.ScaleUpNonCriticalDeployments(
                    ctx, nodeName, policy.Spec.CriticalLabelKey,
                ); err != nil {
                    log.Error(err, "Error restaurando deployments", "node", nodeName)
                }
            }
        hbStatus.ResourceDegradationExecuted = false
        return
    }

    if hbStatus.ResourceDegradationExecuted {
        log.Info("Degradación por recursos ya ejecutada, omitiendo",
            "node", nodeName, "cpu", hbStatus.CPU, "memory", hbStatus.Memory)
        return
    }

    if cpuExceeded {
        log.Info("Umbral de CPU superado, escalando deployments a 0",
            "node", nodeName, "cpu", hbStatus.CPU, "threshold", policy.Spec.MaxCPUThreshold)
    }
    if memExceeded {
        log.Info("Umbral de memoria superado, escalando deployments a 0",
            "node", nodeName, "memory", hbStatus.Memory, "threshold", policy.Spec.MaxMemoryThreshold)
    }

    // ← CAMBIO: ScaleDown en lugar de EvictNonCriticalPods
    if err := r.DegradationManager.ScaleDownNonCriticalDeployments(
        ctx, nodeName, policy.Spec.CriticalLabelKey,
    ); err != nil {
        log.Error(err, "Error en degradación por recursos", "node", nodeName)
        return
    }

    hbStatus.ResourceDegradationExecuted = true
    event := fmt.Sprintf("resource degradation at %s (cpu: %s, memory: %s)",
        time.Now().UTC().Format(time.RFC3339),
        hbStatus.CPU,
        hbStatus.Memory,
    )
    hbStatus.OfflineEvents = append(hbStatus.OfflineEvents, event)
    log.Info("Degradación por recursos completada", "node", nodeName)
}

func (r *ReducedNodePolicyReconciler) hasScaledDownDeployments(ctx context.Context) bool {
    var deployList appsv1.DeploymentList
    if err := r.Client.List(ctx, &deployList,
        client.MatchingLabels{"edge.priority": "non-critical"},
    ); err != nil {
        return false
    }
    for _, deploy := range deployList.Items {
        if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
            return true
        }
    }
    return false
}

func (r *ReducedNodePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
        WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
        Complete(r)
}