// api/v1alpha1/reducednodepolicy_types.go
package v1alpha1


import (
metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
// ReducedNodePolicySpec defines desired configuration for nodes of type reducido
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ReducedNodePolicySpec struct {
NodeSelector map[string]string `json:"nodeSelector,omitempty"`
GracePeriodSeconds int `json:"gracePeriodSeconds"` // tiempo de espera antes de migrar cargas
CriticalLabelKey string `json:"criticalLabelKey"` // clave que identifica pods críticos
MaxCPUThreshold int `json:"maxCPUThreshold,omitempty"` // límite opcional para activar degradación
MaxMemoryThreshold int `json:"maxMemoryThreshold,omitempty"`
}


// ReducedNodePolicyStatus shows current state
// +kubebuilder:object:root=true
type ReducedNodePolicyStatus struct {
ObservedNodes int `json:"observedNodes"`
OfflineNodes int `json:"offlineNodes"`
LastSync metav1.Time `json:"lastSync"`
}


// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ReducedNodePolicy struct {
metav1.TypeMeta `json:",inline"`
metav1.ObjectMeta `json:"metadata,omitempty"`


Spec ReducedNodePolicySpec `json:"spec,omitempty"`
Status ReducedNodePolicyStatus `json:"status,omitempty"`
}


// +kubebuilder:object:root=true
type ReducedNodePolicyList struct {
metav1.TypeMeta `json:",inline"`
metav1.ListMeta `json:"metadata,omitempty"`
Items []ReducedNodePolicy `json:"items"`
}


func init() {
SchemeBuilder.Register(&ReducedNodePolicy{}, &ReducedNodePolicyList{})
}