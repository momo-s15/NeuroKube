# NEUROKUBE — Autonomous AIOps Kubernetes Platform
## Full Project Blueprint & Technical Documentation

**Stack:** Go · Kubernetes · Prometheus · Loki · Ollama · Slack  
**Cluster:** kind (Kubernetes in Docker) — local, zero cloud cost  
**AI:** Ollama llama3.2 — RTX 3060 GPU inference  
**Timeline:** 3 focused days · Day 1 infra · Day 2 brain · Day 3 action  
**Cost:** $0.00 — entirely free and self-hosted  

---

## 0. Project Overview

NeuroKube is a self-healing, locally hosted Kubernetes platform that closes the full AIOps loop: **detect → diagnose → notify → remediate** without human intervention until the final one-click approval.

**The problem it solves:** Traditional Kubernetes alerting tells you something broke. NeuroKube tells you *why* it broke, what the logs say, and gives you a one-click patch — all within seconds of the crash.

### The Four Pillars

| Pillar | Technology | What it does |
|--------|-----------|--------------|
| 1. Playground | kind, Docker, stress-ng | Local multi-node cluster; victim Nginx pod for demo |
| 2. Nervous System | Prometheus, Loki, Grafana, Alertmanager | Scrape metrics, ingest logs, fire alerts on OOMKilled |
| 3. Brain | Go microservice, Ollama llama3.2 | Receive alert → fetch logs → LLM diagnosis |
| 4. Action | Slack API (Socket Mode), client-go | Rich Slack message + [Apply Patch] button → K8s API fix |

### The Wow-Effect Demo Scenario

1. You run a stress script that intentionally exhausts the Nginx pod's memory limit.
2. Kubernetes kills the pod with an OOMKilled event. Prometheus detects it within 10 seconds.
3. Alertmanager fires an HTTP POST to your Go webhook. The webhook queries Loki for the last 50 log lines.
4. The Go service sends the logs to Ollama with a strict system prompt. Diagnosis returned in < 3 seconds.
5. **Slack chimes:** "⚠️ KUBERNETES CRASH: OOMKilled on frontend pod. Root cause: memory leak in API process. Recommended: raise limit from 256Mi → 512Mi."
6. You click [Apply Patch]. The Go webhook calls client-go, patches the Deployment, pod restarts healthy.

### Resume Bullet Point

> "Engineered an autonomous Kubernetes AIOps platform (NeuroKube) integrating Prometheus, Loki, and a local LLM (Ollama llama3.2) for automated root-cause analysis and one-click remediation via Slack."

> "Developed a Golang microservice (client-go + Socket Mode) that bridges Alertmanager webhooks, Loki log queries, and the Kubernetes API to execute live deployment patches without human intervention."

---

## 1. Prerequisites & Environment

### Hardware Requirements

| Component | Requirement |
|-----------|-------------|
| CPU | Intel i7-12th gen or equivalent (4+ cores) |
| RAM | 16 GB minimum — kind cluster + Prometheus + Loki + Ollama |
| GPU | NVIDIA RTX 3060 (12 GB VRAM) — for llama3.2 inference via Ollama |
| Disk | 30 GB free — Docker images + Ollama model (~4 GB) |
| OS | Windows 11 with WSL2 Ubuntu 22.04, or native Linux |

### Software to Install

| Tool | Version | Purpose |
|------|---------|---------|
| Docker Desktop | 4.x+ | Container runtime for kind |
| kind | 0.22+ | Kubernetes in Docker |
| kubectl | 1.29+ | Kubernetes CLI |
| Helm | 3.14+ | Package manager for K8s |
| Go | 1.22+ | Brain microservice language |
| Ollama | 0.3+ | Local LLM inference server |

### Installation Commands (WSL2 / Ubuntu)

```bash
# Docker
curl -fsSL https://get.docker.com | sh

# kind
go install sigs.k8s.io/kind@v0.22.0

# kubectl
curl -LO https://dl.k8s.io/release/$(curl -Ls https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Go 1.22
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc

# Ollama
curl -fsSL https://ollama.com/install.sh | sh
ollama pull llama3.2
```

### Slack App Setup

Create a free Slack workspace and a new app at api.slack.com/apps.

**Required OAuth scopes:**
- `chat:write` — send messages
- `chat:write.public` — post in channels without joining
- `connections:write` — required for Socket Mode

**Enable Socket Mode:**
1. Settings → Socket Mode → Enable
2. Generate an App-Level Token with `connections:write` scope → save as `SLACK_APP_TOKEN`
3. OAuth & Permissions → Install to Workspace → save Bot Token as `SLACK_BOT_TOKEN`
4. Settings → Interactivity & Shortcuts → Enable (no public URL needed with Socket Mode)

---

## 2. Day 1 — Cluster & Observability

### Repository Structure

```
neurokube/
├── cluster/
│   ├── kind-config.yaml
│   └── namespaces.yaml
├── observability/
│   ├── prometheus-values.yaml
│   └── loki-values.yaml
├── victim/
│   ├── deployment.yaml
│   └── stress-test.sh
├── brain/
│   ├── main.go
│   ├── handler/
│   │   ├── alert.go
│   │   ├── slack.go
│   │   └── action.go
│   ├── loki/
│   │   └── query.go
│   ├── llm/
│   │   └── ollama.go
│   └── k8s/
│       └── patch.go
├── go.mod
├── go.sum
├── .env.example
└── Makefile
```

### cluster/kind-config.yaml

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: neurokube
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
  - role: worker
    extraPortMappings:
      - containerPort: 30000
        hostPort: 30000
        protocol: TCP
  - role: worker
```

```bash
kind create cluster --config cluster/kind-config.yaml
kubectl cluster-info --context kind-neurokube
kubectl get nodes   # Expect: 1 control-plane + 2 workers
```

### observability/prometheus-values.yaml

```yaml
grafana:
  enabled: true
  adminPassword: neurokube123
  service:
    type: NodePort
    nodePort: 30000

alertmanager:
  enabled: true
  config:
    global:
      resolve_timeout: 5m
    route:
      group_by: ['alertname', 'pod']
      receiver: neurokube-webhook
      routes:
        - matchers:
            - alertname = KubePodCrashLooping
            - alertname = KubePodOOMKilled
          receiver: neurokube-webhook
    receivers:
      - name: neurokube-webhook
        webhook_configs:
          - url: 'http://neurokube-brain.neurokube.svc.cluster.local:8080/alert'
            send_resolved: false
```

### observability/loki-values.yaml

```yaml
loki:
  enabled: true
  persistence:
    enabled: false
promtail:
  enabled: true
```

### Install Both Stacks

```bash
kubectl create namespace monitoring

helm install kube-prom-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values observability/prometheus-values.yaml

helm install loki-stack grafana/loki-stack \
  --namespace monitoring \
  --values observability/loki-values.yaml

kubectl get pods -n monitoring

# Grafana: http://localhost:30000  admin / neurokube123
# Loki data source URL: http://loki-stack.monitoring.svc.cluster.local:3100
```

### victim/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-victim
  namespace: default
  labels:
    app: nginx-victim
    neurokube.io/monitored: "true"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-victim
  template:
    metadata:
      labels:
        app: nginx-victim
    spec:
      containers:
        - name: nginx
          image: nginx:alpine
          resources:
            requests:
              memory: "64Mi"
              cpu: "50m"
            limits:
              memory: "256Mi"   # Intentionally tight for OOMKill demo
              cpu: "200m"
        - name: stress
          image: polinux/stress
          command: ["sleep", "infinity"]
          resources:
            limits:
              memory: "256Mi"
```

### victim/stress-test.sh

```bash
#!/bin/bash
set -e

POD=$(kubectl get pod -l app=nginx-victim -o jsonpath='{.items[0].metadata.name}')
echo "[NeuroKube] Targeting pod: $POD"
echo "[NeuroKube] Starting memory stress — pod will OOMKill in ~15 seconds..."

kubectl exec "$POD" -c stress -- stress --vm 1 --vm-bytes 300M --timeout 30s

echo "[NeuroKube] Stress complete. Watch Prometheus + Slack for the autonomous response."
```

```bash
kubectl apply -f victim/deployment.yaml
chmod +x victim/stress-test.sh
```

---

## 3. Day 2 — The Go Brain Service

### Go Module Setup

```bash
mkdir brain && cd brain
go mod init github.com/yourname/neurokube

go get github.com/slack-go/slack
go get k8s.io/client-go@v0.29.0
go get k8s.io/apimachinery@v0.29.0
go get github.com/gorilla/mux
```

### brain/main.go

```go
package main

import (
    "log"
    "os"
    "github.com/yourname/neurokube/handler"
)

func main() {
    cfg := handler.Config{
        Port:          getEnv("PORT", "8080"),
        LokiURL:       getEnv("LOKI_URL", "http://loki-stack.monitoring:3100"),
        OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
        OllamaModel:   getEnv("OLLAMA_MODEL", "llama3.2"),
        SlackBotToken: os.Getenv("SLACK_BOT_TOKEN"),
        SlackAppToken: os.Getenv("SLACK_APP_TOKEN"),
        SlackChannel:  getEnv("SLACK_CHANNEL", "#neurokube-alerts"),
        KubeConfig:    os.Getenv("KUBECONFIG"),
    }

    srv := handler.NewServer(cfg)
    log.Printf("[NeuroKube] Brain service starting on :%s", cfg.Port)
    log.Fatal(srv.Start())
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" { return v }
    return fallback
}
```

### handler/alert.go

```go
package handler

import (
    "encoding/json"
    "log"
    "net/http"
)

type AlertmanagerPayload struct {
    Alerts []Alert `json:"alerts"`
}

type Alert struct {
    Status      string            `json:"status"`
    Labels      map[string]string `json:"labels"`
    Annotations map[string]string `json:"annotations"`
}

func (s *Server) handleAlert(w http.ResponseWriter, r *http.Request) {
    var payload AlertmanagerPayload
    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, "bad payload", 400); return
    }

    for _, alert := range payload.Alerts {
        if alert.Status != "firing" { continue }
        go s.processAlert(alert)
    }
    w.WriteHeader(http.StatusOK)
}

func (s *Server) processAlert(alert Alert) {
    pod       := alert.Labels["pod"]
    namespace := alert.Labels["namespace"]
    alertName := alert.Labels["alertname"]
    log.Printf("[alert] %s fired for %s/%s", alertName, namespace, pod)

    // 1. Fetch last 50 log lines from Loki
    logs, err := s.loki.FetchLogs(namespace, pod, 50)
    if err != nil {
        log.Printf("[loki] error: %v", err)
        logs = "(log fetch failed — proceeding with alert metadata only)"
    }

    // 2. Ask Ollama to diagnose
    diagnosis, err := s.llm.Diagnose(alertName, pod, logs)
    if err != nil {
        log.Printf("[llm] error: %v", err)
        diagnosis = "AI diagnosis unavailable."
    }

    // 3. Send rich Slack message with [Apply Patch] button
    s.slack.SendCrashAlert(namespace, pod, alertName, diagnosis)
}
```

### loki/query.go

```go
package loki

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"
)

type Client struct { BaseURL string }

type lokiResponse struct {
    Data struct {
        Result []struct {
            Values [][]string `json:"values"`
        } `json:"result"`
    } `json:"data"`
}

func (c *Client) FetchLogs(namespace, pod string, lines int) (string, error) {
    query := fmt.Sprintf(`{namespace=%q, pod=~"%s.*"}`, namespace, pod)
    params := url.Values{
        "query":     {query},
        "limit":     {fmt.Sprintf("%d", lines)},
        "start":     {fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).UnixNano())},
        "end":       {fmt.Sprintf("%d", time.Now().UnixNano())},
        "direction": {"backward"},
    }

    resp, err := http.Get(c.BaseURL + "/loki/api/v1/query_range?" + params.Encode())
    if err != nil { return "", err }
    defer resp.Body.Close()

    var result lokiResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return "", err }

    var sb strings.Builder
    for _, stream := range result.Data.Result {
        for _, entry := range stream.Values {
            if len(entry) >= 2 { sb.WriteString(entry[1] + "\n") }
        }
    }
    if sb.Len() == 0 { return "(no logs found in last 10 minutes)", nil }
    return sb.String(), nil
}
```

### llm/ollama.go

```go
package llm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type Client struct {
    BaseURL string
    Model   string
}

type ollamaRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
}

type ollamaResponse struct {
    Response string `json:"response"`
}

const systemPrompt = `You are an expert Kubernetes SRE.
You receive a Kubernetes pod crash alert and its recent logs.
Respond in exactly this JSON format, nothing else:
{
  "root_cause": "one sentence — the exact technical reason for the crash",
  "evidence":   "the specific log line or metric that confirms it",
  "fix":        "one sentence — the exact remediation command or config change",
  "new_limit":  "if OOMKilled: the recommended new memory limit (e.g. 512Mi), else null"
}`

func (c *Client) Diagnose(alertName, pod, logs string) (string, error) {
    userPrompt := fmt.Sprintf(
        "Alert: %s\nPod: %s\nLogs:\n%s",
        alertName, pod, logs)

    body, _ := json.Marshal(ollamaRequest{
        Model:  c.Model,
        Prompt: systemPrompt + "\n\nUser:\n" + userPrompt,
        Stream: false,
    })

    resp, err := http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewReader(body))
    if err != nil { return "", err }
    defer resp.Body.Close()

    var result ollamaResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return "", err }
    return result.Response, nil
}
```

---

## 4. Day 3 — Slack Action & Kubernetes Remediation

### handler/slack.go

```go
package handler

import (
    "encoding/json"
    "fmt"
    "log"
    slackgo "github.com/slack-go/slack"
)

type Diagnosis struct {
    RootCause string `json:"root_cause"`
    Evidence  string `json:"evidence"`
    Fix       string `json:"fix"`
    NewLimit  string `json:"new_limit"`
}

func (s *SlackClient) SendCrashAlert(ns, pod, alertName, rawDiagnosis string) {
    var d Diagnosis
    if err := json.Unmarshal([]byte(rawDiagnosis), &d); err != nil {
        d = Diagnosis{RootCause: rawDiagnosis, Fix: "Manual investigation required."}
    }

    actionValue := fmt.Sprintf("%s|%s|%s", ns, pod, d.NewLimit)

    blocks := []slackgo.Block{
        slackgo.NewHeaderBlock(slackgo.NewTextBlockObject(
            "plain_text", "⚠️ KUBERNETES CRASH DETECTED", false, false)),

        slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
            fmt.Sprintf("*Pod:* `%s/%s`\n*Alert:* `%s`", ns, pod, alertName),
        nil), nil, nil),

        slackgo.NewDividerBlock(),

        slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
            "*Root Cause:*\n"+d.RootCause, false, false), nil, nil),

        slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
            "*Evidence:*\n`"+d.Evidence+"`", false, false), nil, nil),

        slackgo.NewSectionBlock(slackgo.NewTextBlockObject("mrkdwn",
            "*Recommended Fix:*\n"+d.Fix, false, false), nil, nil),

        slackgo.NewActionBlock("",
            slackgo.NewButtonBlockElement(
                "apply_patch",
                actionValue,
                slackgo.NewTextBlockObject("plain_text", "✅ Apply Patch", false, false),
            ).WithStyle(slackgo.StylePrimary),
            slackgo.NewButtonBlockElement(
                "dismiss",
                actionValue,
                slackgo.NewTextBlockObject("plain_text", "✖ Dismiss", false, false),
            ).WithStyle(slackgo.StyleDanger),
        ),
    }

    _, _, err := s.api.PostMessage(s.channel,
        slackgo.MsgOptionBlocks(blocks...))
    if err != nil { log.Printf("[slack] post error: %v", err) }
}
```

### handler/action.go

```go
package handler

import (
    "log"
    "strings"
    "github.com/slack-go/slack/socketmode"
)

func (s *Server) startSocketMode() {
    client := socketmode.New(s.slack.api, socketmode.OptionDebug(false))
    go func() {
        for evt := range client.Events {
            switch evt.Type {
            case socketmode.EventTypeInteractive:
                client.Ack(*evt.Request)
                go s.handleInteraction(evt)
            }
        }
    }()
    client.Run()
}

func (s *Server) handleButtonAction(actionID, value, responseURL string) {
    parts := strings.Split(value, "|")
    if len(parts) < 3 { return }
    namespace, pod, newLimit := parts[0], parts[1], parts[2]

    switch actionID {
    case "apply_patch":
        log.Printf("[action] patching %s/%s → memory limit %s", namespace, pod, newLimit)
        err := s.k8s.PatchMemoryLimit(namespace, pod, newLimit)
        if err != nil {
            s.slack.PostEphemeral(responseURL, "❌ Patch failed: "+err.Error())
            return
        }
        s.slack.PostEphemeral(responseURL,
            "✅ Patch applied. Pod `"+pod+"` restarting with new memory limit: "+newLimit)

    case "dismiss":
        s.slack.PostEphemeral(responseURL, "Alert dismissed. No changes made.")
    }
}
```

### k8s/patch.go

```go
package k8s

import (
    "context"
    "encoding/json"
    "fmt"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

type Client struct { clientset *kubernetes.Clientset }

func NewClient(kubeconfig string) (*Client, error) {
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil { return nil, err }
    cs, err := kubernetes.NewForConfig(config)
    if err != nil { return nil, err }
    return &Client{clientset: cs}, nil
}

func (c *Client) PatchMemoryLimit(namespace, podName, newLimit string) error {
    ctx := context.Background()

    pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
    if err != nil { return fmt.Errorf("get pod: %w", err) }

    deployName, ok := pod.Labels["app"]
    if !ok { return fmt.Errorf("pod has no 'app' label") }

    patch := map[string]interface{}{
        "spec": map[string]interface{}{
            "template": map[string]interface{}{
                "spec": map[string]interface{}{
                    "containers": []map[string]interface{}{
                        {
                            "name": deployName,
                            "resources": map[string]interface{}{
                                "limits": map[string]string{"memory": newLimit},
                            },
                        },
                    },
                },
            },
        },
    }

    patchBytes, _ := json.Marshal(patch)
    _, err = c.clientset.AppsV1().Deployments(namespace).Patch(
        ctx, deployName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
    return err
}

func (c *Client) RestartPod(namespace, podName string) error {
    return c.clientset.CoreV1().Pods(namespace).Delete(
        context.Background(), podName, metav1.DeleteOptions{})
}
```

---

## 5. Deploying the Brain Into the Cluster

### .env.example

```
PORT=8080
LOKI_URL=http://loki-stack.monitoring.svc.cluster.local:3100
OLLAMA_URL=http://host.docker.internal:11434
OLLAMA_MODEL=llama3.2
SLACK_BOT_TOKEN=xoxb-your-bot-token
SLACK_APP_TOKEN=xapp-your-app-level-token
SLACK_CHANNEL=#neurokube-alerts
KUBECONFIG=/root/.kube/config
```

> **NOTE:** Ollama runs on your host machine (using the RTX 3060), not inside the cluster. Pods reach it via `host.docker.internal`. On Linux, add `--add-host=host.docker.internal:host-gateway` to Docker args if needed.

### Dockerfile

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o neurokube-brain ./brain/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/neurokube-brain .
EXPOSE 8080
ENTRYPOINT ["/app/neurokube-brain"]
```

### brain/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: neurokube-brain
  namespace: neurokube
spec:
  replicas: 1
  selector:
    matchLabels:
      app: neurokube-brain
  template:
    metadata:
      labels:
        app: neurokube-brain
    spec:
      serviceAccountName: neurokube-brain-sa
      containers:
        - name: brain
          image: neurokube-brain:latest
          imagePullPolicy: Never
          ports:
            - containerPort: 8080
          envFrom:
            - secretRef:
                name: neurokube-secrets
          resources:
            requests: { memory: "128Mi", cpu: "100m" }
            limits:   { memory: "256Mi", cpu: "500m" }
---
apiVersion: v1
kind: Service
metadata:
  name: neurokube-brain
  namespace: neurokube
spec:
  selector:
    app: neurokube-brain
  ports:
    - port: 8080
      targetPort: 8080
```

### brain/rbac.yaml

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: neurokube-brain-sa
  namespace: neurokube
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: neurokube-brain-role
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "patch", "update"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: neurokube-brain-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: neurokube-brain-role
subjects:
  - kind: ServiceAccount
    name: neurokube-brain-sa
    namespace: neurokube
```

### Build and Load Into kind

```bash
docker build -t neurokube-brain:latest ./brain
kind load docker-image neurokube-brain:latest --name neurokube

kubectl create namespace neurokube
kubectl create secret generic neurokube-secrets \
  --from-env-file=.env \
  --namespace neurokube

kubectl apply -f brain/rbac.yaml
kubectl apply -f brain/deployment.yaml

kubectl get pods -n neurokube
kubectl logs -n neurokube -l app=neurokube-brain -f
```

---

## 6. Makefile

```makefile
.PHONY: cluster-up cluster-down obs-install brain-build brain-deploy demo clean all

cluster-up:
	kind create cluster --config cluster/kind-config.yaml
	kubectl cluster-info --context kind-neurokube

cluster-down:
	kind delete cluster --name neurokube

obs-install:
	helm install kube-prom-stack prometheus-community/kube-prometheus-stack \
	  --namespace monitoring --create-namespace \
	  --values observability/prometheus-values.yaml
	helm install loki-stack grafana/loki-stack \
	  --namespace monitoring \
	  --values observability/loki-values.yaml

brain-build:
	docker build -t neurokube-brain:latest ./brain
	kind load docker-image neurokube-brain:latest --name neurokube

brain-deploy:
	kubectl create namespace neurokube --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic neurokube-secrets \
	  --from-env-file=.env --namespace neurokube \
	  --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f brain/rbac.yaml
	kubectl apply -f brain/deployment.yaml
	kubectl apply -f victim/deployment.yaml

demo:
	@echo "==> Triggering OOMKill demo..."
	bash victim/stress-test.sh

logs:
	kubectl logs -n neurokube -l app=neurokube-brain -f

clean:
	kind delete cluster --name neurokube
	docker rmi neurokube-brain:latest 2>/dev/null || true

all: cluster-up obs-install brain-build brain-deploy
	@echo "==> NeuroKube is live. Run: make demo"
```

---

## 7. Troubleshooting Guide

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Alertmanager not firing | Webhook URL wrong | Check prometheus-values.yaml URL; verify svc DNS |
| Loki returns empty logs | Promtail not running | `kubectl get ds -n monitoring`; check promtail pods |
| Ollama timeout | Model not pulled | `ollama pull llama3.2` on host machine |
| client-go 403 Forbidden | RBAC missing | `kubectl apply -f brain/rbac.yaml` |
| Slack button does nothing | Socket Mode not enabled | api.slack.com → Settings → Socket Mode → Enable |
| kind image not found | Image not loaded | `kind load docker-image neurokube-brain:latest` |
| Ollama unreachable from cluster | host.docker.internal not set | Add `--add-host` on Linux; use host IP directly |

### Debug Commands

```bash
# Watch all pods
kubectl get pods -A -w

# Check Alertmanager config
kubectl exec -n monitoring \
  $(kubectl get pod -n monitoring -l app.kubernetes.io/name=alertmanager -o name) \
  -- cat /etc/alertmanager/config_out/alertmanager.env.yaml

# Manually fire a test alert
curl -X POST http://localhost:8080/alert \
  -H 'Content-Type: application/json' \
  -d '{"alerts":[{"status":"firing","labels":{"alertname":"OOMKilled","pod":"nginx-victim-abc","namespace":"default"}}]}'

# Test Loki query
curl 'http://localhost:3100/loki/api/v1/query_range?query={namespace="default"}&limit=10'

# Test Ollama
curl http://localhost:11434/api/generate \
  -d '{"model":"llama3.2","prompt":"test","stream":false}'
```

---

## 8. Grafana Dashboard

### Recommended Panels

| Panel Name | Query |
|-----------|-------|
| Pod Memory Usage | `container_memory_working_set_bytes{pod=~"nginx-victim.*"}` |
| OOMKill Events | `kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}` |
| Pod Restart Count | `kube_pod_container_status_restarts_total{pod=~"nginx-victim.*"}` |
| Brain Service Uptime | `up{job="neurokube-brain"}` |
| Loki Log Volume | `sum(rate({namespace="default"}[1m]))` |
| Alert History | `ALERTS{alertstate="firing"}` |

Import pre-built dashboards: ID `15760` (pod resources) and ID `13639` (Loki viewer).

---

## 9. Three-Day Sprint Plan

### Day 1 — Cluster & Observability
- [ ] Install all prerequisites (Docker, kind, kubectl, Helm, Go, Ollama)
- [ ] Create kind cluster — verify 3 nodes Running
- [ ] `helm install kube-prometheus-stack` with custom Alertmanager webhook URL
- [ ] `helm install loki-stack` — verify Promtail DaemonSet running
- [ ] Deploy nginx-victim — confirm it appears in Grafana
- [ ] Run stress-test.sh manually — confirm OOMKill captured by Prometheus
- [ ] Add Loki as Grafana data source — query nginx-victim logs
- [ ] ✅ Done when: Alertmanager fires on OOMKill (check AM UI at :9093)

### Day 2 — The Go Brain
- [ ] `go mod init`, install dependencies
- [ ] Write `loki/query.go` — test standalone with curl
- [ ] Write `llm/ollama.go` — test standalone, confirm valid JSON diagnosis
- [ ] Write `handler/alert.go` — wire Loki + Ollama together
- [ ] Run brain locally, fire test alert with curl, confirm pipeline in logs
- [ ] Write Dockerfile — confirm `docker build` succeeds
- [ ] `kind load docker-image`, deploy to cluster, check `kubectl logs`
- [ ] ✅ Done when: curl fake alert at in-cluster service → see diagnosis in logs

### Day 3 — Slack Action & Demo Video
- [ ] Create Slack app, enable Socket Mode, save tokens to .env
- [ ] Write `handler/slack.go` — test SendCrashAlert with hardcoded diagnosis
- [ ] Write `handler/action.go` — Socket Mode listener + button handler
- [ ] Write `k8s/patch.go` — test PatchMemoryLimit with kubectl diff
- [ ] Apply RBAC manifests, redeploy brain with Slack env vars
- [ ] End-to-end test: `make demo` → watch Slack → click Apply Patch
- [ ] Verify pod restarts: `kubectl describe pod nginx-victim`
- [ ] ✅ Record LinkedIn demo video: terminal + Grafana + Slack side by side

---

## 10. Optional Extensions

| Extension | Notes |
|-----------|-------|
| Multi-alert types | Handle CrashLoopBackOff — different diagnosis prompt |
| Fix history log | Append every diagnosis + action to SQLite inside cluster |
| Grafana annotations | POST to Grafana annotation API when patch is applied |
| Slack /status command | Query current pod health from Slack directly |
| GitHub Actions CI | Auto-build and `kind load` on git push to main |
| Helm chart for brain | Package brain service as a reusable Helm chart |
| Multiple LLMs | A/B test Ollama llama3.2 vs Claude API — compare diagnosis quality |
| Alert deduplication | Rate-limit duplicate alerts per pod to avoid Slack spam |
| Memory leak simulator | Replace stress-ng with a real Go app that leaks on a timer |

---

## 11. Resume & Interview Assets

### CV Bullets

**Primary:**
> "Engineered an autonomous Kubernetes AIOps platform (NeuroKube) integrating Prometheus, Loki, and a local LLM (Ollama llama3.2) for automated root-cause analysis and one-click remediation via Slack."

**Supporting:**
> "Developed a Golang microservice (client-go + Slack Socket Mode) that bridges Alertmanager webhooks, Loki log queries, and the Kubernetes API to execute live deployment patches without human intervention."

**Skills surfaced:**
`Go · Kubernetes · Prometheus · Loki · Grafana · Helm · Docker · Ollama · LLM · Slack API · AIOps · SRE · Platform Engineering`

### LinkedIn Post Structure

1. **Hook:** "I built a Kubernetes cluster that fixes itself. Here's the 60-second demo ↓"
2. **The problem:** Traditional alerting only tells you something broke.
3. **The solution:** NeuroKube detects the crash, reads the logs with AI, sends a one-click fix in Slack.
4. **The tech:** kind · Prometheus · Loki · Grafana · Go · Ollama on local GPU · Slack Socket Mode · client-go
5. **The wow moment:** Screen recording of the patch button applying live.
6. **Hashtags:** `#Kubernetes #DevOps #AIOps #Golang #SRE #PlatformEngineering #LLM #OpenSource`

### Interview Talking Points

**Why Go?**
Statically compiled single binary, excellent concurrency for handling multiple simultaneous alerts, first-class Kubernetes client library (client-go).

**Why local LLM?**
Zero API cost during development, full control over the prompt, GPU-accelerated on the RTX 3060, no data leaves the machine — important for production SRE contexts.

**Why Socket Mode?**
Eliminates the need for a public-facing webhook URL. The bot maintains a persistent WebSocket to Slack's servers, making the entire stack runnable on a developer laptop with no port forwarding.

**Scaling considerations:**
The Go service is stateless — multiple replicas behind a K8s Service would work. Alert deduplication would be the first production concern.

**Production delta:**
Replace kind with EKS/GKE, Ollama with a managed LLM endpoint, add PersistentVolumes for Loki, and add mTLS between components.

---

*NeuroKube Blueprint v1.0 · $0 infrastructure · 3-day build · production-grade resume impact*
