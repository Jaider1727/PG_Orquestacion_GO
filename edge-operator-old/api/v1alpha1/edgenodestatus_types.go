package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EdgeNodeStatusSpec defines the desired state
type EdgeNodeStatusSpec struct {
	NodeName string `json:"nodeName"`
	NodeType string `json:"nodeType"` // "reducido", "normal"
	Location string `json:"location"`
}

// EdgeNodeStatusStatus defines the observed state
type EdgeNodeStatusStatus struct {
	Connected     bool     `json:"connected"`
	LastHeartbeat string   `json:"lastHeartbeat"`
	BatteryLevel  int      `json:"batteryLevel"`
	CPUUsage      int      `json:"cpuUsage"`
	CriticalPods  []string `json:"criticalPods"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EdgeNodeStatus is the Schema for the edgenodestatuses API
type EdgeNodeStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeNodeStatusSpec   `json:"spec,omitempty"`
	Status EdgeNodeStatusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EdgeNodeStatusList contains a list of EdgeNodeStatus
type EdgeNodeStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeNodeStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeNodeStatus{}, &EdgeNodeStatusList{})
}
