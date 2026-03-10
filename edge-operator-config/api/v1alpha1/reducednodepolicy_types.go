// api/v1alpha1/reducednodepolicy_types.go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReducedNodePolicySpec defines desired configuration for nodes of type reducido.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ReducedNodePolicySpec struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// GracePeriodSeconds es el tiempo de espera antes de migrar cargas.
	GracePeriodSeconds int `json:"gracePeriodSeconds"`
	// CriticalLabelKey es la clave que identifica pods críticos.
	CriticalLabelKey string `json:"criticalLabelKey"`
	// MaxCPUThreshold es el límite opcional de CPU para activar degradación (porcentaje).
	MaxCPUThreshold int `json:"maxCPUThreshold,omitempty"`
	// MaxMemoryThreshold es el límite opcional de memoria para activar degradación (porcentaje).
	MaxMemoryThreshold int `json:"maxMemoryThreshold,omitempty"`
}

// NodeHeartbeatStatus almacena la información de heartbeat de un nodo individual.
type NodeHeartbeatStatus struct {
	// State es "online" u "offline".
	// +kubebuilder:validation:Enum=online;offline
	State string `json:"state"`
	// LastHeartbeat es el timestamp del último heartbeat recibido.
	LastHeartbeat metav1.Time `json:"lastHeartbeat,omitempty"`
	// CPU reportado en el último heartbeat.
	CPU string `json:"cpu,omitempty"`
	// Memory reportada en el último heartbeat.
	Memory string `json:"memory,omitempty"`
}

// ReducedNodePolicyStatus muestra el estado actual del conjunto de nodos gestionados.
// +kubebuilder:object:root=true
type ReducedNodePolicyStatus struct {
	// ObservedNodes es el total de nodos que coinciden con el selector.
	ObservedNodes int `json:"observedNodes"`
	// OfflineNodes es el número de nodos actualmente marcados como offline.
	OfflineNodes int `json:"offlineNodes"`
	// LastSync es el timestamp de la última sincronización del operador.
	LastSync metav1.Time `json:"lastSync"`
	// Nodes contiene el estado de heartbeat de cada nodo observado.
	// La clave del mapa es el nombre del nodo.
	// +optional
	Nodes map[string]NodeHeartbeatStatus `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Observed",type=integer,JSONPath=".status.observedNodes"
// +kubebuilder:printcolumn:name="Offline",type=integer,JSONPath=".status.offlineNodes"
// +kubebuilder:printcolumn:name="LastSync",type=date,JSONPath=".status.lastSync"
type ReducedNodePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReducedNodePolicySpec   `json:"spec,omitempty"`
	Status ReducedNodePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ReducedNodePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReducedNodePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReducedNodePolicy{}, &ReducedNodePolicyList{})
}
