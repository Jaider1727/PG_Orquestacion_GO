package degradation_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/jaiderssjgod/edge-operator/internal/degradation"
)

func TestEvictNonCriticalPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pods := []corev1.Pod{
		makePod("critical-pod",   "default", "node-1", "critical"),
		makePod("noncrit-pod",    "default", "node-1", "non-critical"),
		makePod("unlabeled-pod",  "default", "node-1", ""),           // sin etiqueta: no tocar
		makePod("other-node-pod", "default", "node-2", "non-critical"),
	}

	objs := make([]runtime.Object, len(pods))
	for i := range pods {
		objs[i] = &pods[i]
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	mgr := degradation.New(fakeClient, logr.Discard())
	if err := mgr.EvictNonCriticalPods(context.Background(), "node-1"); err != nil {
		t.Fatalf("EvictNonCriticalPods returned error: %v", err)
	}

	var remaining corev1.PodList
	_ = fakeClient.List(context.Background(), &remaining)

	for _, pod := range remaining.Items {
		if pod.Name == "noncrit-pod" {
			t.Error("noncrit-pod debería haber sido eliminado pero sigue presente")
		}
	}

	names := podNames(remaining.Items)
	for _, expected := range []string{"critical-pod", "unlabeled-pod", "other-node-pod"} {
		if !contains(names, expected) {
			t.Errorf("pod %s no debería haber sido eliminado", expected)
		}
	}
}

func makePod(name, ns, node, priority string) corev1.Pod {
	labels := map[string]string{}
	if priority != "" {
		labels[degradation.PriorityLabelKey] = priority
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec:       corev1.PodSpec{NodeName: node},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func podNames(pods []corev1.Pod) []string {
	names := make([]string, len(pods))
	for i, p := range pods {
		names[i] = p.Name
	}
	return names
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}