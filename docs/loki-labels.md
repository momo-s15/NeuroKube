# Loki labels (NeuroKube)

Promtail (via `loki-stack`) usually attaches Kubernetes pod metadata to log streams.

## Typical keys

- **`namespace`** — Kubernetes namespace.
- **`pod`** — Pod name (common on current chart combinations).
- **`pod_name`** — Seen on some older or alternate Promtail configs.

## Brain behavior

[`brain/loki/query.go`](../brain/loki/query.go) queries in order:

1. `{namespace="<ns>", pod="<full-pod-name>"}`
2. `{namespace="<ns>", pod_name="<full-pod-name>"}`

Alertmanager alerts for workload pods normally include the full **`pod`** label matching the Pod metadata name.

## Manual discovery

From a pod in `monitoring` (or port-forward Loki to localhost):

```bash
curl -sS "http://loki-stack.monitoring.svc.cluster.local:3100/loki/api/v1/labels"
curl -sS "http://loki-stack.monitoring.svc.cluster.local:3100/loki/api/v1/label/pod/values"
curl -sS "http://loki-stack.monitoring.svc.cluster.local:3100/loki/api/v1/label/pod_name/values"
```

Use **Explore → Loki** in Grafana to confirm which label keys exist for `namespace="default"`.
