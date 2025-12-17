package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EdgeNodeStatusSpec es la parte declarativa del CR
type EdgeNodeStatusSpec struct {
	NodeName string `json:"nodeName"`
	NodeType string `json:"nodeType"` // "normal" o "reducido"
	Location string `json:"location"`
}

// EdgeNodeStatusStatus es la parte observable del CR
type EdgeNodeStatusStatus struct {
	Connected     bool     `json:"connected"`
	LastHeartbeat string   `json:"lastHeartbeat"`
	BatteryLevel  int      `json:"batteryLevel"`
	CPUUsage      int      `json:"cpuUsage"`
	CriticalPods  []string `json:"criticalPods"`
}

// EdgeNodeStatus representa el CR completo
type EdgeNodeStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeNodeStatusSpec   `json:"spec"`
	Status EdgeNodeStatusStatus `json:"status"`
}
