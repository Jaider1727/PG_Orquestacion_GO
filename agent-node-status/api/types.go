package api

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ReducedNodeStatusSpec struct {
	NodeName        string      `json:"nodeName"`
	State           string      `json:"state"` // Online, Offline, Degraded
	IsReduced       bool        `json:"isReduced"`
	BatteryLevel    int         `json:"batteryLevel"`
	CpuLoad         float64     `json:"cpuLoad"`
	MemoryUsageMB   int         `json:"memoryUsageMB"`
	LastHeartbeat   metav1.Time `json:"lastHeartbeat"`
	CriticalPods    []string    `json:"criticalPods"`
	NonCriticalPods []string    `json:"nonCriticalPods"`
	OfflineEvents   []string    `json:"offlineEvents"`
}

type ReducedNodeStatusStatus struct {
	Reconciled       bool        `json:"reconciled"`
	LastSynced       metav1.Time `json:"lastSynced"`
	NeedsMigration   bool        `json:"needsMigration"`
	PolicyViolations []string    `json:"policyViolations"`
}

type ReducedNodeStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReducedNodeStatusSpec   `json:"spec,omitempty"`
	Status ReducedNodeStatusStatus `json:"status,omitempty"`
}
