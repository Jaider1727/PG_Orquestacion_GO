// cmd/agent/main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	criticalLabelKey = "iot/critical"
	nodeTypeLabel    = "node-type"
	nodeTypeValue    = "reducido"
	checkInterval    = 20 * time.Second
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

	fmt.Printf("[AGENT] Running on node: %s\n", nodeName)

	for {
		fmt.Println("[AGENT] Monitoring node...")
		monitorNode(clientset, nodeName)
		time.Sleep(checkInterval)
	}
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
			node.Name,
			nodeTypeLabel,
			nodeTypeValue,
			v,
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
		if pod.Labels[criticalLabelKey] == "true" &&
			pod.Status.Phase == corev1.PodRunning {
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
				fmt.Fprintf(os.Stderr, "[AGENT ERROR] Error parsing CPU fields: %v %v %v %v\n", err1, err2, err3, err4)
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
				memTotal, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to parse MemTotal: %v\n", err)
				} else {
					foundTotal = true
				}
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memAvailable, err = strconv.ParseFloat(fields[1], 64)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[AGENT ERROR] Failed to parse MemAvailable: %v\n", err)
				} else {
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

