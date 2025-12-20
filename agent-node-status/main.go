package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme = runtime.NewScheme()
)

func main() {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Println("NODE_NAME no definido")
		os.Exit(1)
	}

	cfg := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("No se pudo crear cliente: %v", err))
	}

	const maxRetries = 3
	const retryDelay = 5 * time.Second

	prevConnected := true

	for {
		now := time.Now().UTC().Format(time.RFC3339)
		connected := true

		var lastErr error
		for i := 0; i < maxRetries; i++ {
			err = tryPatchStatus(k8sClient, nodeName, now, true)
			if err == nil {
				break
			}
			lastErr = err
			time.Sleep(retryDelay)
		}

		if err != nil {
			fmt.Printf("Falló la conexión tras %d intentos: %v\n", maxRetries, lastErr)
			connected = false
		}

		// Solo marcar desconectado si cambia el estado
		if connected != prevConnected {
			now := time.Now().UTC().Format(time.RFC3339)
			_ = tryPatchStatus(k8sClient, nodeName, now, connected)
			if connected {
				fmt.Println("Reconectado: marcado como conectado")
			} else {
				fmt.Println("Desconectado: marcado como desconectado")
			}
			prevConnected = connected
		} else {
			fmt.Println("Estado estable, sin cambios")
		}

		time.Sleep(15 * time.Second)
	}
}

func tryPatchStatus(k8sClient client.Client, nodeName string, heartbeat string, connected bool) error {
	ens := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "iot.example.com/v1alpha1",
			"kind":       "EdgeNodeStatus",
			"metadata": map[string]interface{}{
				"name": nodeName,
			},
			"spec": map[string]interface{}{
				"nodeName": nodeName,
				"nodeType": "reducido",
				"location": "AWS-Zone-1",
			},
			"status": map[string]interface{}{
				"connected":     connected,
				"lastHeartbeat": heartbeat,
				"batteryLevel":  87,
				"cpuUsage":      41,
				"criticalPods":  []string{"sensor-reader", "local-cache"},
			},
		},
	}

	ens.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "iot.example.com",
		Version: "v1alpha1",
		Kind:    "EdgeNodeStatus",
	})

	return k8sClient.Patch(context.TODO(), ens, client.Apply, &client.PatchOptions{
		FieldManager: "agent",
	})
}
