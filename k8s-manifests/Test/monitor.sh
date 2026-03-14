# monitor.sh
#!/bin/bash
WORKER_NODE=$(kubectl get nodes --no-headers | grep -v master | awk '{print $1}' | head -1)
echo "Monitoreando nodo: $WORKER_NODE"
echo "---"

while true; do
  clear
  echo "=== $(date) ==="
  echo ""

  echo "--- ESTADO DEL NODO ---"
  kubectl get node $WORKER_NODE -o jsonpath='{.status.conditions[-1]}' 2>/dev/null | python3 -m json.tool 2>/dev/null || \
  kubectl get node $WORKER_NODE
  echo ""

  echo "--- ESTADO EN CRD ---"
  kubectl get reducednodepolicy reduced-policy \
    -o jsonpath="{.status.nodes.$WORKER_NODE}" 2>/dev/null | \
    python3 -m json.tool 2>/dev/null || echo "(sin datos aún)"
  echo ""

  echo "--- PODS EN EL WORKER ---"
  kubectl get pods -o wide --field-selector spec.nodeName=$WORKER_NODE \
    --no-headers 2>/dev/null | awk '{printf "%-30s %-12s %-20s\n", $1, $3, $7}'
  echo ""

  echo "--- LOGS OPERATOR (últimas 10 líneas) ---"
  kubectl logs -l app=reduced-node-operator --tail=10 2>/dev/null
  echo ""

  sleep 5
done




