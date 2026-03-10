// cmd/agent/main.go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jaiderssjgod/agent-node-status/heartbeat"
)

const (
	criticalLabelKey  = "iot/critical"
	nodeTypeLabel     = "node-type"
	nodeTypeValue     = "reducido"
	checkInterval     = 20 * time.Second
	heartbeatInterval = 10 * time.Second
)

func main() {
	fmt.Println("[AGENT] Starting agent...")

	cfg, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to load cluster config: %v\n", err)
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to create clientset: %v\n", err)
		panic(err.Error())
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] NODE_NAME env var required but missing\n")
		panic("NODE_NAME env var required")
	}

	// OPERATOR_HEARTBEAT_URL es la URL del endpoint del operador, e.g.:
	// http://edge-operator-service.edge-system.svc.cluster.local:8080/heartbeat
	operatorURL := os.Getenv("OPERATOR_HEARTBEAT_URL")
	if operatorURL == "" {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] OPERATOR_HEARTBEAT_URL env var required but missing\n")
		panic("OPERATOR_HEARTBEAT_URL env var required")
	}

	fmt.Printf("[AGENT] Running on node: %s\n", nodeName)

	// Goroutine independiente para el heartbeat (cada 10 s)
	go runHeartbeatLoop(nodeName, operatorURL)

	// Loop principal de monitoreo (cada 20 s)
	for {
		fmt.Println("[AGENT] Monitoring node...")
		monitorNode(clientset, nodeName)
		time.Sleep(checkInterval)
	}
}

// runHeartbeatLoop envía un heartbeat al operador cada heartbeatInterval.
// Corre en su propia goroutine para ser independiente del ciclo de monitoreo.
func runHeartbeatLoop(nodeName, operatorURL string) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		sendHeartbeat(nodeName, operatorURL)
	}
}

// sendHeartbeat construye y envía el payload de heartbeat al operador.
func sendHeartbeat(nodeName, operatorURL string) {
	payload := heartbeat.Payload{
		NodeName:  nodeName,
		Timestamp: time.Now().UTC(),
		CPU:       readCPUUsage(),
		Memory:    readMemUsage(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to marshal heartbeat: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, operatorURL, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to create heartbeat request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to send heartbeat: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[AGENT WARN] Heartbeat returned non-200 status: %d\n", resp.StatusCode)
		return
	}

	fmt.Printf("[AGENT] Heartbeat sent for node %s at %s\n", nodeName, payload.Timestamp.Format(time.RFC3339))
}

func monitorNode(clientset *kubernetes.Clientset, nodeName string) {
	ctx := context.Background()

	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Error fetching node: %v\n", err)
		return
	}

	if v, ok := node.Labels[nodeTypeLabel]; !ok || v != nodeTypeValue {
		fmt.Printf(
			"[AGENT] Node %s is not labeled as '%s=%s' (label is '%s'), skipping\n",
			node.Name, nodeTypeLabel, nodeTypeValue, v,
		)
		return
	}

	pods, err := clientset.CoreV1().Pods("").List(
		ctx,
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Error listing pods: %v\n", err)
		return
	}

	cpuUsage := readCPUUsage()
	memUsage := readMemUsage()

	status := struct {
		Time         time.Time `json:"time"`
		Node         string    `json:"node"`
		NodeType     string    `json:"node_type"`
		CPU          string    `json:"cpu"`
		Mem          string    `json:"memory"`
		CriticalPods int       `json:"critical_pods"`
	}{
		Time:         time.Now(),
		Node:         node.Name,
		NodeType:     node.Labels[nodeTypeLabel],
		CPU:          cpuUsage,
		Mem:          memUsage,
		CriticalPods: 0,
	}

	for _, pod := range pods.Items {
		if pod.Labels[criticalLabelKey] == "true" && pod.Status.Phase == corev1.PodRunning {
			status.CriticalPods++
		}
	}

	out, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to marshal status: %v\n", err)
		return
	}

	fmt.Println(string(out))
}

func readCPUUsage() string {
	file, err := os.Open("/proc/stat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Cannot open /proc/stat: %v\n", err)
		return "unavailable"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				fmt.Fprintf(os.Stderr, "[AGENT ERROR] /proc/stat malformed: not enough fields\n")
				return "invalid"
			}

			user, err1 := strconv.ParseFloat(fields[1], 64)
			nice, err2 := strconv.ParseFloat(fields[2], 64)
			system, err3 := strconv.ParseFloat(fields[3], 64)
			idle, err4 := strconv.ParseFloat(fields[4], 64)
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
				fmt.Fprintf(os.Stderr, "[AGENT ERROR] Error parsing CPU fields\n")
				return "invalid"
			}

			total := user + nice + system + idle
			if total == 0 {
				return "0.00%"
			}
			usage := ((user + nice + system) / total) * 100.0
			return fmt.Sprintf("%.2f%%", usage)
		}
	}

	fmt.Fprintf(os.Stderr, "[AGENT ERROR] No 'cpu ' line found in /proc/stat\n")
	return "not found"
}

func readMemUsage() string {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Cannot open /proc/meminfo: %v\n", err)
		return "unavailable"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	memTotal := 0.0
	memAvailable := 0.0
	foundTotal := false
	foundAvail := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
					memTotal = v
					foundTotal = true
				}
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
					memAvailable = v
					foundAvail = true
				}
			}
		}
	}

	if !foundTotal || !foundAvail || memTotal == 0 {
		fmt.Fprintf(os.Stderr, "[AGENT ERROR] Missing or invalid MemTotal/MemAvailable\n")
		return "invalid"
	}

	used := memTotal - memAvailable
	usage := (used / memTotal) * 100.0
	return fmt.Sprintf("%.2f%%", usage)
}
