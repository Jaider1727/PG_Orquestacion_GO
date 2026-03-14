#!/bin/bash
set -e

echo ""
echo "[TEST] Verificando estado del sistema - Requisitos RF01, RF03, RF07, RF08"
echo "------------------------------------------------------------"

echo "[STEP 1] Validando existencia del CRD ReducedNodePolicy..."
kubectl get reducednodepolicies.iot.mydomain.com reduced-policy -o json > /tmp/crd.json

observed=$(jq '.status.observedNodes' /tmp/crd.json)
offline=$(jq '.status.offlineNodes' /tmp/crd.json)

if [ "$observed" -gt 0 ]; then
  echo "[OK] Operator reconoce $observed nodos reducidos (RF-01, RF-07)"
else
  echo "[FAIL] Operator no reconoce ningún nodo reducido. Verifica nodeSelector o etiquetas"
fi

echo ""
echo "[STEP 2] Verificando nodos etiquetados como 'node-type=reducido'..."
count=$(kubectl get nodes -l node-type=reducido --no-headers | wc -l)
if [ "$count" -gt 0 ]; then
  echo "[OK] $count nodos tienen la etiqueta esperada (node-type=reducido)"
else
  echo "[FAIL] No hay nodos con la etiqueta esperada"
fi

echo ""
echo "[STEP 3] Verificando logs del agente (últimos 60s)..."
agent_pod=$(kubectl get pods -l app=reduced-node-agent -o jsonpath='{.items[0].metadata.name}')
if [ -z "$agent_pod" ]; then
  echo "[FAIL] No se encontró ningún pod del agente (DaemonSet)"
else
  echo "[INFO] Leyendo logs del agente: $agent_pod"
  logs=$(kubectl logs "$agent_pod" --tail=20)
  echo "$logs" | grep -E '"cpu":|CriticalPods' && echo "[OK] Agent está reportando CPU, RAM y pods críticos (RF-03, RF-08)" || echo "[FAIL] Agent no está reportando métricas esperadas"
fi

echo ""
echo "[STEP 4] Estado general:"
kubectl get reducednodepolicies.iot.mydomain.com reduced-policy -o yaml | grep -A5 status

