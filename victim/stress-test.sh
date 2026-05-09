#!/bin/bash
set -e

POD=$(kubectl get pod -l app=nginx-victim -n default -o jsonpath='{.items[0].metadata.name}')
echo "[NeuroKube] Targeting pod: $POD"
echo "[NeuroKube] Starting memory stress in stress sidecar (300M vs 256Mi limit) — worker may get SIGKILL; watch metrics/alerts."

kubectl exec -n default "$POD" -c stress -- stress --vm 1 --vm-bytes 300M --timeout 30s

echo "[NeuroKube] Stress finished. Watch Prometheus / Grafana / Alertmanager for OOM-related signals."
