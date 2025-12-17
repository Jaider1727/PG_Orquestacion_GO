package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EdgeNodeStatusSpec define los campos deseados del CR
type EdgeNodeStatusSpec struct {
	NodeName string `json:"nodeName,omitempty"`
	NodeType string `json:"nodeType,omitempty"`
	Location string `json:"location,omitempty"`
}

// EdgeNodeStatusStatus define los campos observados del CR
type EdgeNodeStatusStatus struct {
	Connected     bool     `json:"connected,omitempty"`
	LastHeartbeat string   `json:"lastHeartbeat,omitempty"`
	BatteryLevel  int      `json:"batteryLevel,omitempty"`
	CPUUsage      int      `json:"cpuUsage,omitempty"`
	CriticalPods  []string `json:"criticalPods,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EdgeNodeStatus es la definición completa
type EdgeNodeStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeNodeStatusSpec   `json:"spec,omitempty"`
	Status EdgeNodeStatusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EdgeNodeStatusList contiene una lista de EdgeNodeStatus
type EdgeNodeStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeNodeStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeNodeStatus{}, &EdgeNodeStatusList{})
}
