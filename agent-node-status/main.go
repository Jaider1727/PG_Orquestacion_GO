package main

import (
	"context"
	"fmt"
	"math/rand"
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
	rand.Seed(time.Now().UnixNano()) // para datos aleatorios distintos en cada ejecución

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

		//Datos de prueba
		batteryLevel := rand.Intn(41) + 60
		cpuUsage := rand.Intn(21) + 30

		var lastErr error
		for i := 0; i < maxRetries; i++ {
			err = tryPatchStatus(k8sClient, nodeName, now, connected, batteryLevel, cpuUsage)
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

		if connected != prevConnected {
			now := time.Now().UTC().Format(time.RFC3339)
			_ = tryPatchStatus(k8sClient, nodeName, now, connected, batteryLevel, cpuUsage)
			if connected {
				fmt.Println("Reconectado: marcado como conectado")
			} else {
				fmt.Println("Desconectado: marcado como desconectado")
			}
			prevConnected = connected
		} else {
			fmt.Printf("Estado actualizado dinámicamente - Battery: %d%% CPU: %d%%\n", batteryLevel, cpuUsage)
		}

		time.Sleep(15 * time.Second)
	}
}

func tryPatchStatus(k8sClient client.Client, nodeName string, heartbeat string, connected bool, battery int, cpu int) error {
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
				"batteryLevel":  battery,
				"cpuUsage":      cpu,
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
