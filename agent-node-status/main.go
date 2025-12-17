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
	// Obtener el nombre del nodo desde una variable de entorno (set en el DaemonSet)
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		fmt.Println("NODE_NAME no definido")
		os.Exit(1)
	}

	// Configuración in-cluster
	cfg := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Sprintf("No se pudo crear cliente: %v", err))
	}

	for {
		now := time.Now().UTC().Format(time.RFC3339)

		// Construimos el objeto como un recurso dinámico
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
					"connected":     true,
					"lastHeartbeat": now,
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

		// Hacemos apply: create or update
		err = k8sClient.Patch(context.TODO(), ens, client.Apply, &client.PatchOptions{
			FieldManager: "agent",
		})
		if err != nil {
			fmt.Printf("Error actualizando CR: %v\n", err)
		} else {
			fmt.Println("Estado actualizado en CR")
		}

		time.Sleep(15 * time.Second)
	}
}
