# NeuroKube build progress log

This file records what was done **in this workspace** following **`neurokube_fromscratch.md`**, using the **same Part letters (A–V)** as that doc so you can line up “where am I?” with the plan.

**Source plan:** `neurokube_fromscratch.md` (reference: `neurokube_blueprint.md`).

**Environment:** Windows 11, PowerShell, Docker Desktop (WSL2-backed), kind cluster name `neurokube`.

### Blueprint letter checklist (this workspace)

| Letter | Topic in `neurokube_fromscratch.md` | Status |
|--------|-------------------------------------|--------|
| **A** | Prerequisites, tools, Ollama | **Done** |
| **B** | Slack app + manifest | **Done** |
| **C** | Repo scaffold + initial brain code | **Done** |
| **D** | kind cluster | **Done** |
| **E** | Helm observability stack | **Done** |
| **F** | Grafana + Loki wiring | **Done** |
| **G** | Victim workload + stress | **Done** |
| **H** | Alertmanager routing | **Done** |
| **I** | Go module / dependencies | **Done** *(landed with **C**)* |
| **J** | Config / env | **Done** *(with **C**)* |
| **K** | Loki client + label discovery | **Done** — exact `pod` + fallback `pod_name` queries, HTTP timeout, errors on non-2xx; see [`docs/loki-labels.md`](docs/loki-labels.md) |
| **L** | Ollama `/api/generate` | **Done** *(with **C**)* |
| **M** | `/alert` handler | **Done** *(with **C**)* |
| **N** | Server glue + routes | **Done** *(with **C**; **`/metrics`** added under **V**)* |
| **O** | Run brain on **host** (`go run`, port-forward Loki) | **Skipped** — used in-cluster brain + port-forward smoke tests instead |
| **P** | K8s client + memory patch | **Done** (+ strategic-merge fix after first Slack patch) |
| **Q** | Slack Block Kit | **Done** *(with **C**)* |
| **R** | Socket Mode + button interactions | **Done** *(with **C**)* |
| **S** | `docker build` + `kind load` | **Done** |
| **T** | Secrets, RBAC, deploy (`host.docker.internal` on Windows) | **Done** *(blueprint **T** Linux `hostAliases` not used)* |
| **U** | Full Day-3 E2E (stress, AM → brain, Slack, patch) | **Mostly done** — synthetic path + patch verified; optional **`victim/deployment-oom.yaml`** + **`make demo-oom`** to push real OOM-style events (alert firing still depends on Prometheus rules) |
| **V** | Optional polish | **Partial** — ServiceMonitor + **`/metrics`**; root **`README.md`**, **`LICENSE`**, **`.github/workflows/ci.yml`**; Grafana dashboard imports still optional |

**You are here:** **A–T** and **I–N, P–R** covered; **O** skipped; **U** optional real-alert tuning; **V** optional Grafana imports + demo video.

---

## Part A — Prerequisites and host setup

### Goals

- Enough disk/RAM awareness for kind + Prometheus + Loki + Ollama.
- Docker, `kubectl`, `kind`, Helm, Go, Ollama installed and verified.
- Choose where cluster commands run (Windows PowerShell vs WSL); we standardized on **PowerShell** with tools on PATH.

### What we verified or installed

| Component | Action / version notes |
|-----------|------------------------|
| **Go** | Already present: `go version` → **go1.26.0 windows/amd64** (meets ≥ 1.22). |
| **Ollama** | Running on host; `curl http://localhost:11434/api/tags` returned **`llama3.2:latest`** (~3.2B parameters). |
| **Docker** | Initially missing from PATH. **Docker Desktop** was downloaded and installed (quiet installer). First `docker version` failed (daemon not up); starting **Docker Desktop** fixed it — client + **Server: Docker Desktop 4.72.0**, context **desktop-linux**. |
| **kubectl** | Not on PATH. Installed by downloading **v1.31.0** Windows amd64 binary to **`%USERPROFILE%\bin\kubectl.exe`**. |
| **kind** | Installed **v0.22.0** to **`%USERPROFILE%\bin\kind.exe`**. |
| **Helm** | Installed **v3.18.0** to **`%USERPROFILE%\bin\helm.exe`** (extracted from official zip). |
| **PATH** | Session PATH prefixed with **`%USERPROFILE%\bin`** so CLI tools work in new shells until the user adds that folder to permanent user PATH. |

### `.env` / model alignment (started in A, completed when `.env` existed)

- **`OLLAMA_MODEL`** set to match the pulled Ollama model (e.g. `llama3.2:latest`) so the brain’s `/api/generate` call uses a real tag.

### Part A status

**Complete** for local development: Docker, kubectl, kind, Helm, Go, Ollama verified; WSL2 reported as default (`wsl.exe --status`).

---

## Part B — Slack application

### Goals

- Slack app with **Bot Token** + **Socket Mode App-Level Token**, scopes for posting and Socket Mode, interactivity for buttons, channel for alerts.

### What we did

1. Added **`slack-app-manifest.yaml`** at repo root for **Create app → From an app manifest**:
   - Bot scopes: `chat:write`, `chat:write.public`
   - **`settings.socket_mode_enabled: true`**
   - **`settings.interactivity.is_enabled: true`**
   - Clarified in comments: **`connections:write`** belongs on the **App-Level token** (`xapp-`), not necessarily as a separate bot OAuth scope in the manifest.
2. Opened **`https://api.slack.com/apps`** in the browser for manual steps.
3. User created the app and supplied tokens; we **did not** repeat those secrets in docs after creation.

### Security note (important)

- Slack tokens were pasted in chat during setup; **they should be rotated** in the Slack app settings if exposure is a concern, then `.env` updated locally.

### Part B status

**Complete** from a project perspective once the app exists, tokens are in **`.env`**, and channel name matches **`SLACK_CHANNEL`**.

---

## Part C — Repository scaffold (files + brain service)

### Goals

- Directory layout: `cluster/`, `observability/`, `victim/`, `brain/` (Go **module root** under `brain/`), root **`Makefile`**, **`.env.example`**, **`.gitignore`**.

### Files and directories created

| Path | Purpose |
|------|---------|
| `cluster/kind-config.yaml` | kind: 1 control-plane (ingress-ready label), 2 workers, **NodePort 30000** mapped on first worker. |
| `cluster/namespaces.yaml` | Namespaces **`neurokube`** and **`monitoring`**. |
| `observability/prometheus-values.yaml` | Grafana NodePort **30000**, admin password, Alertmanager route to brain webhook; **fixed route matchers** (one alert name per child route + `KubeContainerOOMKilled`). |
| `observability/loki-values.yaml` | Loki + Promtail; later **`loki.isDefault: false`** (see Part E). |
| `victim/deployment.yaml` | `nginx-victim` Deployment (nginx + stress sidecar). |
| `victim/stress-test.sh` | Stress script for OOM demo. |
| `.env.example` | Documented keys (placeholders for secrets). |
| `.gitignore` | **`.env`**, binaries, IDE dirs; brain build artifacts. |
| `Makefile` | `cluster-up`, `cluster-down`, `obs-install` (includes `helm repo add` + `update`), `brain-build`, `brain-deploy`, `demo`, `logs`, `clean`, `all`. |
| `brain/go.mod` | Module: **`github.com/ms11m/smartkubernetes`**. |
| `brain/main.go` | Loads **`handler.Config`** from env. |
| `brain/handler/server.go` | **`NewServer`**, HTTP **`/alert`**, **`go startSocketMode()`** when Slack tokens set. |
| `brain/handler/alert.go` | Alertmanager JSON, async **`processAlert`**, Loki + LLM + Slack. |
| `brain/handler/slack.go` | Block Kit message, **`PostResponseURL`** for button replies. |
| `brain/handler/action.go` | Socket Mode loop; unpack **`slack.InteractionCallback`**; **`apply_patch`** / **`dismiss`**. |
| `brain/loki/query.go` | LogQL `query_range` client (blueprint-style query; label discovery still recommended in Part K of from-scratch). |
| `brain/llm/ollama.go` | **`/api/generate`**, timeout-aware HTTP client. |
| `brain/k8s/patch.go` | **`NewClient`**: kubeconfig path or **in-cluster**; **`PatchMemoryLimit`** patches **container name `nginx`** (not the `app` label) for the victim workload. |
| `brain/Dockerfile` | Multi-stage build: **`go build -o neurokube-brain .`** from `brain/` context. |
| `brain/deployment.yaml` / `brain/rbac.yaml` | Brain Deployment + Service + RBAC (as blueprint). |
| `brain/.dockerignore` | Exclude local `neurokube-brain.exe` from image context. |

### Implementation fixes vs blueprint snippets

1. **Slack `slack-go` API:** `NewTextBlockObject` requires **four** arguments (`emoji`, `verbatim` booleans); fixed **mrkdwn** section for pod/alert line.
2. **Alertmanager values:** Avoid **two `alertname` matchers in one route** (AND); split into separate routes; add **`KubeContainerOOMKilled`** for chart/version drift.
3. **Prometheus CR (see Part E):** Added **`prometheus.prometheusSpec.maximumStartupDurationSeconds`** — required by operator validation (≥ 60).
4. **Grafana + Loki default datasource conflict (see Part E):** Set **`loki.isDefault: false`** in `loki-values.yaml`.
5. **Kubernetes patch:** Patch **Deployment** using **`app`** label for deployment name but **container name `nginx`** in the strategic merge patch body.

### Builds verified

- **`go build`** in `brain/` succeeded on Windows.
- **`docker build -t neurokube-brain:latest ./brain`** succeeded.

### Secrets

- **`.env`** created with real Slack tokens and local **`KUBECONFIG`** (user path); **`.gitignore`** excludes **`.env`**.

### Part C status

**Complete** — repo scaffold and brain code are in place and build.

---

## Part D — Create cluster (kind)

### Commands executed (conceptually)

From repo root, with **`%USERPROFILE%\bin`** on PATH:

1. **`kind create cluster --config cluster/kind-config.yaml`**
   - Cluster name: **`neurokube`**
   - Node image: **kindest/node:v1.29.2**
   - Context: **`kind-neurokube`**
2. **`kubectl cluster-info --context kind-neurokube`**
3. **`kubectl get nodes`** — **3 nodes Ready** (1 control-plane, 2 workers).
4. **`kubectl apply -f cluster/namespaces.yaml`** — **`neurokube`**, **`monitoring`** created.

### Part D status

**Complete.**

---

## Part E — Helm repos and observability stack

### Commands executed

With Helm repos added/updated and releases installed into **`monitoring`**:

1. **`helm repo add prometheus-community https://prometheus-community.github.io/helm-charts`**
2. **`helm repo add grafana https://grafana.github.io/helm-charts`**
3. **`helm repo update`**
4. **`helm install kube-prom-stack prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace --values observability/prometheus-values.yaml`**
5. **`helm install loki-stack grafana/loki-stack --namespace monitoring --values observability/loki-values.yaml`**

Helm reported **loki-stack** chart as **deprecated** (still usable for this walkthrough).

### Issue 1 — kube-prometheus-stack install failed (then fixed)

**Symptom:** Prometheus CR rejected:

- `spec.maximumStartupDurationSeconds: Invalid value: 0: ... should be greater than or equal to 60`

**Fix:** In **`observability/prometheus-values.yaml`**:

```yaml
prometheus:
  prometheusSpec:
    maximumStartupDurationSeconds: 300
```

Then:

- **`helm upgrade kube-prom-stack prometheus-community/kube-prometheus-stack -n monitoring -f observability/prometheus-values.yaml`**

**Result:** Release **deployed** (revision 2).

### Issue 2 — Grafana CrashLoopBackOff (then fixed)

**Symptom:** Grafana logs showed provisioning error:

- *Only one datasource per organization can be marked as default*

**Cause:** **kube-prometheus-stack** Grafana marks **Prometheus** as default; **loki-stack** provisioned **Loki** with **`loki.isDefault: true`** by default.

**Fix:** In **`observability/loki-values.yaml`**:

```yaml
loki:
  isDefault: false
```

Then:

- **`helm upgrade loki-stack grafana/loki-stack -n monitoring -f observability/loki-values.yaml`**
- **`kubectl rollout restart deployment/kube-prom-stack-grafana -n monitoring`**

**Result:** Grafana **3/3 Running**; Promtail **1/1** on all nodes; Prometheus **2/2**; Loki **1/1**; Alertmanager **2/2**.

### End state — key services (monitoring namespace)

| Resource | Notes |
|----------|--------|
| **Grafana** | Service **`kube-prom-stack-grafana`**, **NodePort 30000** → **http://localhost:30000**, admin / **`neurokube123`**. |
| **Loki** | Service **`loki-stack`**, port **3100** → in-cluster **`http://loki-stack.monitoring.svc.cluster.local:3100`**. |
| **Prometheus** | Operator-managed; service **`kube-prom-stack-kube-prome-prometheus`** (port 9090). |
| **Alertmanager** | Service **`kube-prom-stack-kube-prome-alertmanager`** (9093). |

### Part E status

**Complete** — stacks installed; Prometheus/Grafana/Loki issues above documented and resolved in values files.

---

## Part F — Grafana and Loki wiring

### Goals (from `neurokube_fromscratch.md`)

- Reach Grafana on **NodePort 30000** (`http://localhost:30000`).
- Login **admin** / **`neurokube123`** (from `observability/prometheus-values.yaml`).
- Ensure **Loki** is a Grafana datasource (cluster DNS, port **3100**).
- Save & test datasource; Explore with Loki (full log smoke easiest after **Part G** victim).

### What we executed / verified

| Step | Result |
|------|--------|
| **Grafana `/api/health`** | HTTP **200** on `http://localhost:30000`. |
| **Grafana API auth** | `GET /api/datasources` with **Basic auth** `admin:neurokube123` succeeded. |
| **Loki datasource** | Already **provisioned** by **loki-stack** sidecar: name **`Loki`**, UID **`P8E80F9AEF21F6940`**, URL **`http://loki-stack:3100`** (same namespace as Grafana → resolves; equivalent to `http://loki-stack.monitoring.svc.cluster.local:3100`). |
| **Save & Test (UI / health API)** | `GET /api/datasources/uid/P8E80F9AEF21F6940/health` returns **ERROR** in this combo (**Grafana 13** + **Loki 2.9** from `loki-stack`). Grafana’s check issues a **metrics-style** probe (`vector(1)+vector(1)`); **Loki 2.9** responds with LogQL **parse error** — so the button is **not** a reliable signal here. |
| **Loki actually works** | From the **Grafana pod**, `wget` to **`/loki/api/v1/query_range`** with LogQL `{namespace="kube-system"}` returned **`status":"success"`** and log lines — confirms **network + Loki + Promtail** end-to-end. |

### Optional UI follow-up (you)

1. Open **http://localhost:30000** → **Connections → Data sources → Loki** (or **Explore** → Loki).
2. If “Test” shows an error, **ignore it** for now if **Explore** runs queries (known version quirk above).
3. Try **Explore → Loki** with: `{namespace="kube-system"}` — you should see streams.
4. After **Part G**, use `{namespace="default", pod=~"nginx-victim.*"}` (or discover label names per Part K of the checklist).

### Part F status

**Complete** — automation checks plus **manual UI confirmation**:

- **Explore → Prometheus:** `up` returned many series (mix of `1` and expected `0` on kind for some control-plane scrapes).
- **Explore → Loki:** `{namespace="kube-system"}` returned live streams (e.g. kindnet / CNI **`Handling node with IPs`** lines), confirming end-to-end logs in Grafana.

---

## Part G — Victim workload and stress demo

### Goals (from `neurokube_fromscratch.md`)

- Apply **`victim/deployment.yaml`** (`nginx-victim` in **`default`**).
- Wait for pod **Running**; confirm **two** containers (**`nginx`**, **`stress`**).
- Run memory stress (`stress-test.sh` or **`kubectl exec`**); watch metrics / alerts (OOM **alert** and **pod restart** depend on kube / Prometheus rules — see note below).
- Use Grafana **Explore → Prometheus** for nginx-victim-style metrics (memory working set, restarts).

### Execution log (this workspace)

| Step | Result |
|------|--------|
| **`kubectl apply -f victim/deployment.yaml`** | Deployment **`nginx-victim`** created in **`default`**. |
| **Rollout** | **`kubectl rollout status deployment/nginx-victim -n default`** → success; pod **`2/2 Running`**. |
| **Containers** | **`kubectl get pod … -o jsonpath` for container names** → **`nginx`**, **`stress`**. |
| **Stress** | **`kubectl exec … -c stress -- stress --vm 1 --vm-bytes 300M --timeout 25s`** → worker received **signal 9** (SIGKILL, typical under memory pressure vs **256Mi** cgroup); **`stress`** exits non‑zero; pod remained **`2/2 Running`** with **0** restarts because **PID 1** in the stress container is **`sleep infinity`** (OOM/cgroup often kills the worker only). |
| **Manifest experiment (reverted)** | A variant with **`stress` as PID 1** and **`300M`** vs **`256Mi`** led to **CrashLoopBackOff** / rollout timeouts (**`stress`** exits with **Error** when allocation fails), so **`victim/deployment.yaml`** was restored to the **blueprint** pattern (`sleep infinity` + on-demand **`kubectl exec`**). |

### Grafana / Prometheus (manual)

In **Explore → Prometheus**, try:

- `container_memory_working_set_bytes{namespace="default", pod=~"nginx-victim.*"}`
- `kube_pod_container_status_restarts_total{namespace="default", pod=~"nginx-victim.*"}`

After stress, working set may spike; restarts may stay **0** until something kills a whole container (see note).

### Note — OOMKill **alerts** vs this stress pattern

Prometheus **`KubePodOOMKilled` / `KubeContainerOOMKilled`** fire when the **kernel** records an **OOMKilled** termination on a **container**. The **`sleep infinity` + `kubectl exec` stress** pattern often produces **SIGKILL on a child worker** without restarting the pod, so alerts may **not** fire until Part **H** tuning or a different workload pattern. That is OK for **Part G** (workload + metrics visibility); tightening the demo for reliable alert firing is **Part H** and optional manifest hardening.

### Part G status

**Complete** — victim deployed, **nginx** + **stress** verified, stress command executed once from the terminal; **`victim/stress-test.sh`** left aligned with the blueprint (uses **`-n default`** on **`kubectl`**).

---

## Part H — Alertmanager routing (noise control + webhook prep)

### Goals

- Find Alertmanager in-cluster (**Service**, port **9093**).
- Stop **every** firing alert from hitting **`neurokube-webhook`** while the brain is not deployed (avoids **`AlertmanagerFailedToSendAlerts`** / webhook spam).
- Keep **OOM / crash** class alerts routed to **`neurokube-webhook`** when they fire.
- Confirm routing via **`/api/v2/status`** and **`/api/v2/alerts`**.

### Execution log (this workspace)

| Step | Result |
|------|--------|
| **Service** | **`kube-prom-stack-kube-prome-alertmanager`** in **`monitoring`**, port **9093** (cluster DNS: **`http://kube-prom-stack-kube-prome-alertmanager.monitoring.svc.cluster.local:9093`**). |
| **Problem** | Earlier **`observability/prometheus-values.yaml`** set the **root** route **`receiver: neurokube-webhook`**, so **Watchdog**, **TargetDown**, **etcd** noise, etc. all POSTed to the brain URL → failures when the brain is absent. |
| **Fix** | **`helm upgrade`** with merged **kube-prometheus-stack**-style Alertmanager config in **`observability/prometheus-values.yaml`**: **`inhibit_rules`** (chart-like), **root `receiver: "null"`**, child route **Watchdog → `"null"`**, child routes **KubePodCrashLooping / KubePodOOMKilled / KubeContainerOOMKilled → neurokube-webhook**, **`receivers`**: **`"null"`** + **`neurokube-webhook`**. |
| **Verify config** | **`GET /api/v2/status`** → **`config.original`** shows root **`null`**, Watchdog branch, and OOM/crash branches as intended. |
| **Verify live alerts** | **`GET /api/v2/alerts`**: **Watchdog** → **`receivers: [{"name":"null"}]`**; **TargetDown** → **`null`**; **etcdMembersDown** → **`null`**. **KubePodCrashLooping** / **KubePodOOMKilled** not firing in this cluster state (expected with Part G **`sleep infinity`** stress pattern — see Part G note). |

### Part H status

**Complete** — routing verified after Helm upgrade; infra alerts no longer target the webhook; OOM/crash matchers remain for when those rules fire.

---

## Parts I–N — Brain code (blueprint Day 2)

**Status: Complete** — mostly delivered with **Part C**; exercised end-to-end after **H** and **S–T**.

| Letter | Blueprint focus | Where it lives |
|--------|-----------------|----------------|
| **I** | `go mod`, deps | `brain/go.mod` |
| **J** | `Config`, env | `brain/main.go`, `handler.Config` |
| **K** | Loki + LogQL | `brain/loki/query.go` |
| **L** | Ollama | `brain/llm/ollama.go` |
| **M** | `/alert` | `brain/handler/alert.go` |
| **N** | `Server`, routes, Socket Mode startup | `brain/handler/server.go` |

---

## Part O — Run brain locally (host)

**Status: Skipped** — blueprint: export `KUBECONFIG`, port-forward Loki, `OLLAMA_URL=http://localhost:11434`, `go run .` from `brain/`, curl **`localhost:8080/alert`**. We validated using the **in-cluster** pod and port-forward instead.

---

## Parts P–R — Patch + Slack

**Status: Complete** — **P** `brain/k8s/patch.go` (Deployment strategic merge, container **`nginx`**), **Q** `brain/handler/slack.go`, **R** `brain/handler/action.go`.

---

## Parts S–T — Docker, kind load, secrets, RBAC, deploy

*Blueprint **S**: image + `kind load`. **T**: `.env` → Secret, RBAC, apply manifests; on Linux, optional **`hostAliases`** for `host.docker.internal` — **not used** here (Docker Desktop on Windows).*

### Goals

- Build **`neurokube-brain:latest`**, load into **kind** (`imagePullPolicy: Never`).
- Create **`neurokube-secrets`** from **`.env`** for **`neurokube`**, **omitting host `KUBECONFIG`** → **`rest.InClusterConfig()`**.
- Apply **`brain/rbac.yaml`**, **`brain/deployment.yaml`**; Service **8080** matches Alertmanager **`…/alert`**.
- Smoke-test **`POST /alert`** (`{"alerts":[]}` → **200**).

### Execution log (this workspace)

| Step | Result |
|------|--------|
| **`docker build -t neurokube-brain:latest ./brain`** | Succeeded (multi-stage **Go** → **Alpine**). |
| **`kind load docker-image neurokube-brain:latest --name neurokube`** | Image on control-plane + workers. |
| **Secret** | **`neurokube-secrets`** from **`.env`** minus **`KUBECONFIG`**. **`OLLAMA_URL`**: **`http://host.docker.internal:11434`**. |
| **RBAC + workload** | **`neurokube-brain-sa`**, ClusterRole/Binding, Deployment **Running**, ClusterIP **8080**. |
| **`POST /alert`** | **200** (port-forward smoke test). |

### Makefile note

**`make brain-deploy`** builds **`.env.k8s.tmp`** (`.env` without **`KUBECONFIG`** lines), creates the Secret, applies RBAC + deployment + **`brain/servicemonitor.yaml`**, deletes the temp file.

### Parts S–T status

**Complete.**

---

## Part U — End-to-end demo (blueprint Day 3)

Blueprint expects **stress** → (maybe) **Alertmanager** fires **OOM/crash** → webhook → brain → **Slack** → **Apply Patch** → check **`nginx-victim`** limits. We proved the brain/Slack/patch path with a **synthetic** alert as well.

### U.1 — Synthetic firing alert → Slack → Apply Patch

| Step | Result |
|------|--------|
| **POST** | Firing **`KubePodOOMKilled`** payload with live **`nginx-victim`** pod name → **HTTP 200**. |
| **Logs** | **`[alert] …`** then **`[action] patching … → memory limit 512Mi`** after **Apply Patch**. |
| **Note** | Happy path often omits **`[loki]`** / **`[llm]`** log lines; Ollama can take up to **~3 min** on CPU. |

Rerun:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/post-synthetic-alert.ps1
```

### U.2 — Webhook check, stress, deployment limits

| Step | Result |
|------|--------|
| **Webhook** | **`POST /alert`** via port-forward → **200**; in-cluster URL **`http://neurokube-brain.neurokube.svc.cluster.local:8080/alert`**. |
| **Stress** | Same as **`victim/stress-test.sh`**: **`kubectl exec … -c stress -- stress …`** → worker **SIGKILL**; may **not** create a real **`KubePodOOMKilled`** time series (see **G** note). |
| **OOM-oriented manifest** | **`victim/deployment-oom.yaml`** + **`make demo-oom`** — stress as **PID 1** over **256Mi** to encourage **OOMKilled** / alerting (revert: **`kubectl apply -f victim/deployment.yaml`**). |
| **Deploy** | After patch: **`nginx`** limit **512Mi**, **`stress`** **256Mi** (example RS **`nginx-victim-7fc6c47579`**). |

### Part U status

**Mostly done** — synthetic + stress + patch verified; optional **OOM manifest** for a more realistic Prometheus alert path.

---

## Part V — Optional polish (blueprint)

### V.1 — Prometheus scrape (done)

- **`GET /metrics`** — **`promhttp.Handler()`** in **`handler/server.go`**
- **`brain/servicemonitor.yaml`** — **`release: kube-prom-stack`**, port **`http`**, path **`/metrics`**
- **Service** — named port **`http`**, label **`app: neurokube-brain`** (ServiceMonitor selects **Service** metadata labels)

| Check | Result |
|------|--------|
| **Targets** | **`…:8080/metrics`** **up** |
| **PromQL** | **`up{namespace="neurokube"}`**, **`job="neurokube-brain"`** |

### V.2 — Rest of blueprint Part V (not done)

- Import Grafana dashboards (**IDs in blueprint**, e.g. **15760**, **13639**) if you want pre-built views.
- Other bullets (troubleshooting, video) — reference only.

### Part V status

**Partial** — scraping implemented; dashboard import etc. optional.

---

## What we have not done yet (by letter)

- **O** — Optional local **`go run`** brain on host (see README + blueprint Part O).
- **U** — Tune **`victim/deployment-oom.yaml`** / Prometheus rules if your cluster never emits **`KubePodOOMKilled`** for the stress container.
- **V** — Import Grafana dashboards (**15760** / **13639** per blueprint); optional demo video.
- **General** — Rotate Slack tokens if ever exposed; set GitHub repo topics/description.

---

## Quick reference — important local paths

- **Tools (if not on permanent PATH):** `%USERPROFILE%\bin` — `kubectl.exe`, `kind.exe`, `helm.exe`
- **Repo root:** `SmartKubernetes\`
- **Secrets:** `.env` (gitignored)
- **kind context:** `kind-neurokube`

---

*Last updated: Loki hardening + OOM optional manifest + README/LICENSE/CI; letter checklist refreshed.*
